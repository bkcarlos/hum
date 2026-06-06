package handlers

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bkcarlos/hum/internal/httpx"
	"github.com/bkcarlos/hum/internal/llm"
	"github.com/bkcarlos/hum/internal/quota"
)

// The admin API (C3) sits behind AdminOnly. It reads/writes the live free-tier
// policy, surfaces daily usage, and bans/unbans users — all without a restart,
// since the quota store is the single live source the request path already reads.

// AdminGetConfig (GET /api/admin/config) returns the live free-tier policy +
// admin allowlist. No secret is exposed: the server LLM key is not part of Config.
func (h *Handlers) AdminGetConfig(c *gin.Context) {
	cfg, err := h.quota.GetConfig(c.Request.Context())
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, "quota_error", "读取配额配置失败。")
		return
	}
	httpx.OK(c, cfg)
}

// adminConfigPatch is a partial update: a nil field is left unchanged, so a
// client that omits `admins` can never accidentally wipe the allowlist (the
// allowlist shares the config doc — see docs/freetier-roadmap.md C3 note).
type adminConfigPatch struct {
	Enabled           *bool     `json:"enabled"`
	PerUserDailyLimit *int      `json:"perUserDailyLimit"`
	GlobalDailyLimit  *int      `json:"globalDailyLimit"`
	LLMProvider       *string   `json:"llmProvider"`
	LLMBaseURL        *string   `json:"llmBaseUrl"`
	LLMModel          *string   `json:"llmModel"`
	Admins            *[]string `json:"admins"`
}

// AdminUpdateConfig (POST /api/admin/config) merges a partial patch onto the
// live config and persists it. Read-modify-write preserves any omitted field
// (notably `admins`). Takes effect without a restart. POST (not PUT) to match
// the project's GET/POST-only API + CORS convention.
func (h *Handlers) AdminUpdateConfig(c *gin.Context) {
	var p adminConfigPatch
	if err := c.ShouldBindJSON(&p); err != nil {
		httpx.Fail(c, http.StatusBadRequest, "bad_request", "请求体格式错误。")
		return
	}
	ctx := c.Request.Context()
	cfg, err := h.quota.GetConfig(ctx)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, "quota_error", "读取配额配置失败。")
		return
	}

	if p.Enabled != nil {
		cfg.Enabled = *p.Enabled
	}
	if p.PerUserDailyLimit != nil {
		cfg.PerUserDailyLimit = nonNeg(*p.PerUserDailyLimit)
	}
	if p.GlobalDailyLimit != nil {
		cfg.GlobalDailyLimit = nonNeg(*p.GlobalDailyLimit)
	}
	if p.LLMProvider != nil {
		prov := strings.TrimSpace(*p.LLMProvider)
		if !validProvider(prov) {
			httpx.Fail(c, http.StatusBadRequest, "bad_request", "未知的 LLM provider。")
			return
		}
		cfg.LLMProvider = prov
	}
	if p.LLMBaseURL != nil {
		cfg.LLMBaseURL = strings.TrimSpace(*p.LLMBaseURL)
	}
	if p.LLMModel != nil {
		cfg.LLMModel = strings.TrimSpace(*p.LLMModel)
	}
	if p.Admins != nil {
		cfg.Admins = cleanSubs(*p.Admins)
	}

	if err := h.quota.SetConfig(ctx, cfg); err != nil {
		httpx.Fail(c, http.StatusInternalServerError, "quota_error", "保存配额配置失败。")
		return
	}
	httpx.OK(c, cfg)
}

// AdminUsage (GET /api/admin/usage?day=YYYY-MM-DD) returns the day's global
// metered count + per-user usage (with ban flags), sorted by usage desc. `day`
// defaults to today (UTC).
func (h *Handlers) AdminUsage(c *gin.Context) {
	day := strings.TrimSpace(c.Query("day"))
	if day == "" {
		day = quota.Day(time.Now())
	} else if _, err := time.Parse("2006-01-02", day); err != nil {
		httpx.Fail(c, http.StatusBadRequest, "bad_request", "day 需为 YYYY-MM-DD 格式。")
		return
	}
	ctx := c.Request.Context()
	cfg, err := h.quota.GetConfig(ctx)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, "quota_error", "读取配额配置失败。")
		return
	}
	global, users, err := h.quota.AdminUsage(ctx, day)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, "quota_error", "读取用量失败。")
		return
	}
	sort.Slice(users, func(i, j int) bool {
		if users[i].Used != users[j].Used {
			return users[i].Used > users[j].Used
		}
		return users[i].Sub < users[j].Sub
	})
	httpx.OK(c, gin.H{
		"day":          day,
		"globalUsed":   global,
		"globalLimit":  cfg.GlobalDailyLimit,
		"perUserLimit": cfg.PerUserDailyLimit,
		"users":        users,
	})
}

// AdminBan / AdminUnban (POST /api/admin/users/:sub/ban|unban) toggle a user's
// free-tier ban. Bans are read live at reserve time (not cached like config),
// so a banned user is denied (429 banned) on their next metered call immediately.
func (h *Handlers) AdminBan(c *gin.Context)   { h.setBan(c, true) }
func (h *Handlers) AdminUnban(c *gin.Context) { h.setBan(c, false) }

func (h *Handlers) setBan(c *gin.Context, banned bool) {
	sub := strings.TrimSpace(c.Param("sub"))
	if sub == "" {
		httpx.Fail(c, http.StatusBadRequest, "bad_request", "缺少用户 sub。")
		return
	}
	if err := h.quota.SetBanned(c.Request.Context(), sub, banned); err != nil {
		httpx.Fail(c, http.StatusInternalServerError, "quota_error", "更新封禁状态失败。")
		return
	}
	httpx.OK(c, gin.H{"sub": sub, "banned": banned})
}

// ── helpers ──────────────────────────────────────────────────────────────

func nonNeg(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

func validProvider(p string) bool {
	switch llm.ProviderType(p) {
	case llm.ProviderOpenAICompat, llm.ProviderAnthropic, llm.ProviderGemini:
		return true
	}
	return false
}

// cleanSubs trims, drops empties, and de-duplicates an admin/sub list while
// preserving order.
func cleanSubs(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
