package handlers

import (
	"errors"
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
// key. On any problem it writes the error response and returns ok=false.
func (h *Handlers) provider(c *gin.Context, dto llmConfigDTO) (llm.Provider, bool) {
	key := strings.TrimSpace(c.GetHeader(llmAPIKeyHeader))
	if key == "" {
		httpx.Fail(c, http.StatusBadRequest, "no_key",
			"未配置 LLM API Key，请先在「LLM 设置」中完成配置（F0）。")
		return nil, false
	}
	if strings.TrimSpace(dto.Model) == "" {
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
	p, ok := h.provider(c, body.LLM)
	if !ok {
		return
	}
	res, err := p.RankSongs(c.Request.Context(), body.Intent, body.Candidates, body.Instruction)
	if err != nil {
		writeLLMError(c, err)
		return
	}
	httpx.OK(c, res)
}

// writeLLMError maps a normalized *llm.APIError to an HTTP status + code so the
// frontend can show a precise message (key invalid / quota / rate-limit / …).
func writeLLMError(c *gin.Context, err error) {
	var apiErr *llm.APIError
	if errors.As(err, &apiErr) {
		httpx.Fail(c, statusForKind(apiErr.Kind), string(apiErr.Kind), apiErr.Message)
		return
	}
	httpx.Fail(c, http.StatusBadGateway, "upstream", err.Error())
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
