package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/bkcarlos/hum/internal/httpx"
)

// adminSubKey is where AdminOnly stashes the verified admin's Apple sub so
// downstream admin handlers (C3) can read it via c.GetString(adminSubKey).
const adminSubKey = "adminSub"

// Me (GET /api/auth/me) returns the caller's Apple sub and whether they are an
// admin. Any valid free-tier session is accepted — even non-admins — because the
// app uses isAdmin to decide whether to surface the admin section, and the
// operator uses sub to bootstrap the first admin (paste it into the config doc's
// admins allowlist). isAdmin is best-effort: returning sub reliably is what
// matters here; the real access gate is AdminOnly, which fails closed.
func (h *Handlers) Me(c *gin.Context) {
	if !h.authReady() {
		httpx.Fail(c, http.StatusServiceUnavailable, "free_tier_unconfigured", "登录暂未开放。")
		return
	}
	sub, err := h.sessionSub(c)
	if err != nil {
		httpx.Fail(c, http.StatusUnauthorized, "no_session", "请先登录（Sign in with Apple）。")
		return
	}
	isAdmin := false
	if cfg, cerr := h.quota.GetConfig(c.Request.Context()); cerr == nil {
		isAdmin = cfg.IsAdmin(sub)
	}
	email, _ := h.quota.GetUserEmail(c.Request.Context(), sub)
	httpx.OK(c, gin.H{"sub": sub, "email": email, "isAdmin": isAdmin})
}

// AdminOnly guards the admin API: it requires a valid free-tier session whose
// Apple sub is in the live config's admin allowlist, else 401 (no/invalid
// session) or 403 (not an admin). The allowlist is read from the store (≤30s
// cache in prod), so granting/revoking an admin in the console takes effect
// without a restart. It fails closed: any store error denies access.
func (h *Handlers) AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !h.authReady() {
			httpx.Fail(c, http.StatusServiceUnavailable, "free_tier_unconfigured", "管理功能暂未开放。")
			c.Abort()
			return
		}
		sub, err := h.sessionSub(c)
		if err != nil {
			httpx.Fail(c, http.StatusUnauthorized, "no_session", "请先登录。")
			c.Abort()
			return
		}
		cfg, err := h.quota.GetConfig(c.Request.Context())
		if err != nil {
			httpx.Fail(c, http.StatusInternalServerError, "quota_error", "读取管理员配置失败。")
			c.Abort()
			return
		}
		if !cfg.IsAdmin(sub) {
			httpx.Fail(c, http.StatusForbidden, "forbidden", "需要管理员权限。")
			c.Abort()
			return
		}
		c.Set(adminSubKey, sub)
		c.Next()
	}
}

// AdminMe (GET /api/admin/me) confirms admin access: the admin UI calls it on
// load to gate the page (a non-admin is stopped by AdminOnly with 403 before
// reaching this handler).
func (h *Handlers) AdminMe(c *gin.Context) {
	sub := c.GetString(adminSubKey)
	email, _ := h.quota.GetUserEmail(c.Request.Context(), sub)
	httpx.OK(c, gin.H{"sub": sub, "email": email, "isAdmin": true})
}
