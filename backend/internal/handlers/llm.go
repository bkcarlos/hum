package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/bkcarlos/hum/internal/httpx"
	"github.com/bkcarlos/hum/internal/llm"
)

// llmConfigDTO is the NON-SECRET LLM config carried in the request body. The
// secret (API key) is read separately from the header — never the body.
type llmConfigDTO struct {
	Provider string `json:"provider"`
	BaseURL  string `json:"baseUrl"`
	Model    string `json:"model"`
}

// provider builds a request-scoped llm.Provider from the body config + header
// key, requiring a model. On any problem it writes the error response and
// returns ok=false.
func (h *Handlers) provider(c *gin.Context, dto llmConfigDTO) (llm.Provider, bool) {
	return h.providerWith(c, dto, true)
}

// providerWith is provider() with control over whether a model is required.
// ListModels targets the key + BaseURL only (no model yet), so it passes false.
func (h *Handlers) providerWith(c *gin.Context, dto llmConfigDTO, requireModel bool) (llm.Provider, bool) {
	key := strings.TrimSpace(c.GetHeader(llmAPIKeyHeader))
	if key == "" {
		httpx.Fail(c, http.StatusBadRequest, "no_key",
			"未配置 LLM API Key，请先在「LLM 设置」中完成配置（F0）。")
		return nil, false
	}
	if requireModel && strings.TrimSpace(dto.Model) == "" {
		httpx.Fail(c, http.StatusBadRequest, "bad_request", "缺少模型名（model）。")
		return nil, false
	}
	p, err := llm.New(llm.Config{
		Provider: llm.ProviderType(dto.Provider),
		BaseURL:  dto.BaseURL,
		Model:    dto.Model,
		APIKey:   key, // request-scoped; discarded when this request ends
		Timeout:  h.cfg.UpstreamHTTPTimeout,
	})
	if err != nil {
		httpx.Fail(c, http.StatusBadRequest, "bad_request", err.Error())
		return nil, false
	}
	return p, true
}

// TestLLM (POST /api/llm/test) validates the key/model with a minimal call (F0).
func (h *Handlers) TestLLM(c *gin.Context) {
	var body struct {
		LLM llmConfigDTO `json:"llm"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Fail(c, http.StatusBadRequest, "bad_request", "请求体无效。")
		return
	}
	p, ok := h.provider(c, body.LLM)
	if !ok {
		return
	}
	if err := p.Ping(c.Request.Context()); err != nil {
		writeLLMError(c, err)
		return
	}
	httpx.OK(c, gin.H{"ok": true})
}

// ParseIntent (POST /api/intent) turns natural language into an Intent (F3).
func (h *Handlers) ParseIntent(c *gin.Context) {
	var body struct {
		LLM         llmConfigDTO `json:"llm"`
		Text        string       `json:"text"`
		SeedArtists []string     `json:"seedArtists"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Fail(c, http.StatusBadRequest, "bad_request", "请求体无效。")
		return
	}
	if strings.TrimSpace(body.Text) == "" {
		httpx.Fail(c, http.StatusBadRequest, "bad_request", "请输入一段描述。")
		return
	}
	p, ok := h.provider(c, body.LLM)
	if !ok {
		return
	}
	intent, err := p.ParseIntent(c.Request.Context(), body.Text, body.SeedArtists)
	if err != nil {
		writeLLMError(c, err)
		return
	}
	httpx.OK(c, intent)
}

// Rank (POST /api/rank) selects & orders from a real candidate pool (F5/F10).
func (h *Handlers) Rank(c *gin.Context) {
	var body struct {
		LLM         llmConfigDTO    `json:"llm"`
		Intent      *llm.Intent     `json:"intent"`
		Candidates  []llm.Candidate `json:"candidates"`
		Instruction string          `json:"instruction"` // optional F10 refinement
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Fail(c, http.StatusBadRequest, "bad_request", "请求体无效。")
		return
	}
	if len(body.Candidates) == 0 {
		httpx.Fail(c, http.StatusBadRequest, "bad_request", "候选池为空，无法排序。")
		return
	}
	// Cap the pool before it is fed verbatim into the LLM prompt. The real pool is
	// built at ≤ maxPoolSize; this bounds prompt size (and thus the server's token
	// cost on the free tier) against a client that posts an oversized candidate list.
	if len(body.Candidates) > maxPoolSize {
		body.Candidates = body.Candidates[:maxPoolSize]
	}
	p, refund, ok := h.resolveProvider(c, body.LLM, true)
	if !ok {
		return
	}
	res, err := p.RankSongs(c.Request.Context(), body.Intent, body.Candidates, body.Instruction)
	if err != nil {
		refund() // failed call shouldn't burn free-tier quota
		writeLLMError(c, err)
		return
	}
	httpx.OK(c, res)
}

// Examples (POST /api/examples) generates personalized empty-state example
// prompts ("千人千面") from the client's context + recent local tastes (F2).
func (h *Handlers) Examples(c *gin.Context) {
	var body struct {
		LLM     llmConfigDTO `json:"llm"`
		Context string       `json:"context"`
		Tastes  []string     `json:"tastes"`
		Count   int          `json:"count"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Fail(c, http.StatusBadRequest, "bad_request", "请求体无效。")
		return
	}
	p, ok := h.provider(c, body.LLM)
	if !ok {
		return
	}
	ex, err := p.SuggestExamples(c.Request.Context(), llm.ExampleHints{Context: body.Context, Tastes: body.Tastes}, body.Count)
	if err != nil {
		writeLLMError(c, err)
		return
	}
	httpx.OK(c, gin.H{"examples": ex})
}

// Models (POST /api/llm/models) lists the provider's available models for the
// given key + BaseURL (F0 convenience). Best-effort: many OpenAI-compatible
// gateways don't support it, so the UI falls back to manual entry on error. No
// model is required in the body — discovering which ones exist is the point.
func (h *Handlers) Models(c *gin.Context) {
	var body struct {
		LLM llmConfigDTO `json:"llm"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Fail(c, http.StatusBadRequest, "bad_request", "请求体无效。")
		return
	}
	p, ok := h.providerWith(c, body.LLM, false)
	if !ok {
		return
	}
	models, err := p.ListModels(c.Request.Context())
	if err != nil {
		writeLLMError(c, err)
		return
	}
	httpx.OK(c, gin.H{"models": models})
}

// writeLLMError maps a normalized *llm.APIError to an HTTP status + code so the
// frontend can show a precise message (key invalid / quota / rate-limit / …).
func writeLLMError(c *gin.Context, err error) {
	var apiErr *llm.APIError
	if errors.As(err, &apiErr) {
		// 原始上游细节（含 Base URL / 响应体）始终只进服务端日志。
		if apiErr.Detail != "" {
			slog.Warn("llm upstream error",
				"kind", apiErr.Kind, "status", apiErr.Status,
				"provider", apiErr.Provider, "detail", apiErr.Detail)
		}
		// 原始细节（含 Base URL / 响应体）绝不外露给用户——只进上面的服务端日志。
		// BYOK 也不回显（自己的私有网关地址同样不该出现在错误 UI）；排查 Base URL
		// 去配置页 / 诊断日志看。免费档同理（服务端 Base URL 是红线）。
		httpx.Fail(c, statusForKind(apiErr.Kind), string(apiErr.Kind), apiErr.Message)
		return
	}
	// 未分类错误：原文可能含 URL，只进日志，给用户清洁文案。
	slog.Warn("llm error (unclassified)", "err", err.Error())
	httpx.Fail(c, http.StatusBadGateway, "upstream", "调用上游失败，请稍后重试。")
}

func statusForKind(k llm.ErrorKind) int {
	switch k {
	case llm.ErrAuth:
		return http.StatusUnauthorized
	case llm.ErrQuota:
		return http.StatusPaymentRequired
	case llm.ErrRateLimit:
		return http.StatusTooManyRequests
	case llm.ErrModelNotFound:
		return http.StatusNotFound
	case llm.ErrBadRequest:
		return http.StatusBadRequest
	default: // ErrUpstream, ErrNetwork
		return http.StatusBadGateway
	}
}
