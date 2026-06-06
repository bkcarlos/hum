package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bkcarlos/hum/internal/auth"
	"github.com/bkcarlos/hum/internal/httpx"
	"github.com/bkcarlos/hum/internal/llm"
	"github.com/bkcarlos/hum/internal/quota"
)

// sessionTTL is how long a session token issued from a Sign in with Apple login
// stays valid; the iOS client re-authenticates silently with its stored Apple
// credential when it expires.
const sessionTTL = 30 * 24 * time.Hour

// AppleAuth (POST /api/auth/apple) verifies a Sign in with Apple identity token
// and returns a server session token the client sends as `Authorization: Bearer`
// on free-tier requests. Only the stable Apple `sub` is kept (as the quota key);
// the identity token is never stored.
func (h *Handlers) AppleAuth(c *gin.Context) {
	if h.appleAuth == nil || len(h.sessionSecret) == 0 {
		httpx.Fail(c, http.StatusServiceUnavailable, "free_tier_unconfigured", "免费登录暂未开放。")
		return
	}
	var body struct {
		IdentityToken string `json:"identityToken"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.IdentityToken) == "" {
		httpx.Fail(c, http.StatusBadRequest, "bad_request", "缺少 identityToken。")
		return
	}
	sub, err := h.appleAuth.Verify(strings.TrimSpace(body.IdentityToken))
	if err != nil {
		httpx.Fail(c, http.StatusUnauthorized, "apple_auth_failed", "Apple 登录校验失败，请重试。")
		return
	}
	tok, err := auth.IssueSession(h.sessionSecret, sub, sessionTTL, time.Now())
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, "session_error", "签发会话失败。")
		return
	}
	httpx.OK(c, gin.H{"session": tok, "expiresInSeconds": int(sessionTTL.Seconds())})
}

// AppleWebConfig (GET /api/auth/apple/web) returns the non-secret config the
// browser Sign in with Apple JS flow needs: the Services ID (clientId), the
// registered return URL, and scope. enabled=false when no Services ID is set, so
// the web UI falls back to pasting a session token. Public (no auth) — none of
// this is secret (the clientId is a public identifier).
func (h *Handlers) AppleWebConfig(c *gin.Context) {
	if h.cfg == nil || h.cfg.AppleWebClientID == "" {
		httpx.OK(c, gin.H{"enabled": false})
		return
	}
	httpx.OK(c, gin.H{
		"enabled":     true,
		"clientId":    h.cfg.AppleWebClientID,
		"redirectUri": h.cfg.AppleWebRedirectURI,
		"scope":       "",
	})
}

// resolveProvider returns an LLM provider for this request plus a refund func.
//
//   - BYOK: an X-LLM-Api-Key header → provider from the request body config + that
//     key, UNLIMITED. refund is a no-op.
//   - Free tier: no key but a valid Bearer session → provider from the SERVER's
//     default LLM config + server key, gated by quota. One use is RESERVED here;
//     the caller MUST call refund() if the downstream LLM call fails (or yields no
//     result), so a failed request doesn't burn the user's daily quota.
//
// On any problem it writes the error response and returns ok=false.
func (h *Handlers) resolveProvider(c *gin.Context, dto llmConfigDTO, requireModel bool) (llm.Provider, func(), bool) {
	noop := func() {}

	// BYOK takes precedence and is unlimited — the user pays with their own key.
	if strings.TrimSpace(c.GetHeader(llmAPIKeyHeader)) != "" {
		p, ok := h.providerWith(c, dto, requireModel)
		return p, noop, ok
	}

	// No key → free tier. Must be fully configured.
	if !h.freeTierReady() {
		httpx.Fail(c, http.StatusBadRequest, "no_key",
			"未配置 LLM API Key，请在「设置」中配置，或登录后使用免费额度。")
		return nil, noop, false
	}

	sub, err := h.sessionSub(c)
	if err != nil {
		httpx.Fail(c, http.StatusUnauthorized, "no_session",
			"请先登录（Sign in with Apple）使用免费额度，或配置自己的 LLM Key。")
		return nil, noop, false
	}

	cfg, err := h.quota.GetConfig(c.Request.Context())
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, "quota_error", "读取配额配置失败。")
		return nil, noop, false
	}

	day := quota.Day(time.Now())
	dec, err := h.quota.Reserve(c.Request.Context(), sub, day, cfg)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, "quota_error", "配额检查失败。")
		return nil, noop, false
	}
	if !dec.Allowed {
		writeQuotaDenied(c, dec)
		return nil, noop, false
	}

	// Build the provider from the SERVER's default LLM config + server key. The
	// request body's llm config is ignored on the free tier (no client control of
	// which model the server pays for).
	p, err := llm.New(llm.Config{
		Provider: llm.ProviderType(cfg.LLMProvider),
		BaseURL:  cfg.LLMBaseURL,
		Model:    cfg.LLMModel,
		APIKey:   h.cfg.DefaultLLMKey,
		Timeout:  h.cfg.UpstreamHTTPTimeout,
	})
	if err != nil {
		_ = h.quota.Refund(context.Background(), sub, day)
		httpx.Fail(c, http.StatusInternalServerError, "config", "默认 LLM 配置无效。")
		return nil, noop, false
	}
	refund := func() { _ = h.quota.Refund(context.Background(), sub, day) }
	return p, refund, true
}

// sessionSub extracts and verifies the Apple `sub` from the Bearer session token.
func (h *Handlers) sessionSub(c *gin.Context) (string, error) {
	const prefix = "Bearer "
	authz := c.GetHeader("Authorization")
	if !strings.HasPrefix(authz, prefix) {
		return "", errors.New("missing bearer token")
	}
	return auth.VerifySession(h.sessionSecret, strings.TrimSpace(authz[len(prefix):]), time.Now())
}

// writeQuotaDenied writes a 429 with a user-facing message and the counters, so
// the client can show "x/limit used" and steer the user to BYOK.
func writeQuotaDenied(c *gin.Context, d quota.Decision) {
	msg := "今日免费额度已用完，明天再来，或在设置里填入自己的 LLM Key。"
	switch d.Reason {
	case quota.ReasonGlobal:
		msg = "今日免费额度（全站）已用完，请稍后再试，或使用自己的 LLM Key。"
	case quota.ReasonBanned:
		msg = "你的账号已被限制使用免费额度。"
	case quota.ReasonDisabled:
		msg = "免费额度当前未开放，请配置自己的 LLM Key。"
	}
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": gin.H{
		"code":        "quota_exceeded",
		"message":     msg,
		"reason":      string(d.Reason),
		"userUsed":    d.UserUsed,
		"userLimit":   d.UserLimit,
		"globalUsed":  d.GlobalUsed,
		"globalLimit": d.GlobalLimit,
	}})
}
