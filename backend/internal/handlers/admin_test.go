package handlers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/bkcarlos/hum/internal/auth"
	"github.com/bkcarlos/hum/internal/config"
	"github.com/bkcarlos/hum/internal/quota"
)

// adminHandlers wires a free-tier Handlers (MemoryStore) seeded with the given
// admin subs, and returns the session secret so tests can mint sessions for any
// sub. No Apple network / upstream LLM is needed for the admin surface.
func adminHandlers(admins ...string) (*Handlers, []byte) {
	cfg := &config.Config{UpstreamHTTPTimeout: 5 * time.Second, DefaultLLMKey: "server-key"}
	store := quota.NewMemoryStore(quota.Config{
		Enabled: true, PerUserDailyLimit: 5,
		LLMProvider: "openai-compat", LLMBaseURL: "http://x", LLMModel: "m",
		Admins: admins,
	})
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	verifier := &auth.AppleVerifier{
		Audiences: []string{"com.test"},
		Now:       time.Now,
		KeyFunc:   func(*jwt.Token) (any, error) { return &key.PublicKey, nil },
	}
	secret := []byte("sess-secret")
	return New(cfg, nil, nil).WithFreeTier(store, verifier, secret), secret
}

func adminRouter(h *Handlers) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/api")
	g.GET("/auth/me", h.Me)
	a := g.Group("/admin")
	a.Use(h.AdminOnly())
	a.GET("/me", h.AdminMe)
	a.GET("/config", h.AdminGetConfig)
	a.POST("/config", h.AdminUpdateConfig)
	a.GET("/usage", h.AdminUsage)
	a.POST("/users/:sub/ban", h.AdminBan)
	a.POST("/users/:sub/unban", h.AdminUnban)
	return r
}

func bearer(session string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + session}
}

// meBody decodes the {data:{sub,isAdmin}} payload shared by /auth/me + /admin/me.
func meBody(w *httptest.ResponseRecorder) (sub string, isAdmin bool) {
	var resp struct {
		Data struct {
			Sub     string `json:"sub"`
			IsAdmin bool   `json:"isAdmin"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return resp.Data.Sub, resp.Data.IsAdmin
}

func TestMe_ReturnsSubAndAdminFlag(t *testing.T) {
	h, secret := adminHandlers("admin-sub")
	r := adminRouter(h)

	// A non-admin session: 200 with its own sub and isAdmin=false.
	s1, _ := auth.IssueSession(secret, "u1", time.Hour, time.Now())
	w := do(r, http.MethodGet, "/api/auth/me", bearer(s1), nil)
	if sub, isAdmin := meBody(w); w.Code != http.StatusOK || sub != "u1" || isAdmin {
		t.Fatalf("non-admin /auth/me: code=%d sub=%q isAdmin=%v", w.Code, sub, isAdmin)
	}

	// An admin session: isAdmin=true.
	s2, _ := auth.IssueSession(secret, "admin-sub", time.Hour, time.Now())
	w = do(r, http.MethodGet, "/api/auth/me", bearer(s2), nil)
	if sub, isAdmin := meBody(w); w.Code != http.StatusOK || sub != "admin-sub" || !isAdmin {
		t.Fatalf("admin /auth/me: code=%d sub=%q isAdmin=%v", w.Code, sub, isAdmin)
	}

	// No session at all: 401.
	w = do(r, http.MethodGet, "/api/auth/me", nil, nil)
	if w.Code != http.StatusUnauthorized || errCode(w) != "no_session" {
		t.Fatalf("anon /auth/me: want 401 no_session, got %d %s", w.Code, errCode(w))
	}
}

func TestAdminOnly_GuardsAndIsDynamic(t *testing.T) {
	h, secret := adminHandlers() // no admins yet
	r := adminRouter(h)
	s1, _ := auth.IssueSession(secret, "u1", time.Hour, time.Now())

	// No session → 401.
	if w := do(r, http.MethodGet, "/api/admin/me", nil, nil); w.Code != http.StatusUnauthorized || errCode(w) != "no_session" {
		t.Fatalf("anon /admin/me: want 401 no_session, got %d %s", w.Code, errCode(w))
	}
	// Valid session but not an admin → 403.
	if w := do(r, http.MethodGet, "/api/admin/me", bearer(s1), nil); w.Code != http.StatusForbidden || errCode(w) != "forbidden" {
		t.Fatalf("non-admin /admin/me: want 403 forbidden, got %d %s", w.Code, errCode(w))
	}

	// Promote u1 live (no restart) → 200, and the handler echoes the admin's sub.
	ctx := context.Background()
	cfg, _ := h.quota.GetConfig(ctx)
	cfg.Admins = []string{"u1"}
	if err := h.quota.SetConfig(ctx, cfg); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}
	w := do(r, http.MethodGet, "/api/admin/me", bearer(s1), nil)
	if sub, isAdmin := meBody(w); w.Code != http.StatusOK || sub != "u1" || !isAdmin {
		t.Fatalf("promoted /admin/me: code=%d sub=%q isAdmin=%v", w.Code, sub, isAdmin)
	}

	// Demote u1 live → 403 again.
	cfg.Admins = nil
	_ = h.quota.SetConfig(ctx, cfg)
	if w := do(r, http.MethodGet, "/api/admin/me", bearer(s1), nil); w.Code != http.StatusForbidden {
		t.Fatalf("demoted /admin/me: want 403, got %d", w.Code)
	}
}

func TestAdmin_DisabledWhenFreeTierOff(t *testing.T) {
	// No WithFreeTier → admin surface is unavailable (503), not a 401/403.
	r := adminRouter(New(baseCfg(), nil, nil))
	if w := do(r, http.MethodGet, "/api/auth/me", nil, nil); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("/auth/me off: want 503, got %d", w.Code)
	}
	if w := do(r, http.MethodGet, "/api/admin/me", nil, nil); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("/admin/me off: want 503, got %d", w.Code)
	}
}
