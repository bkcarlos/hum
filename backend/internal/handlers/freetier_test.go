package handlers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/bkcarlos/hum/internal/applemusic"
	"github.com/bkcarlos/hum/internal/auth"
	"github.com/bkcarlos/hum/internal/config"
	"github.com/bkcarlos/hum/internal/quota"
)

func freeRouter(h *Handlers) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/api")
	g.POST("/auth/apple", h.AppleAuth)
	g.POST("/suggest", h.Suggest)
	g.POST("/rank", h.Rank)
	return r
}

// mockSuggestUpstream is an OpenAI-compatible endpoint returning one intent + one
// song, standing in for the server's default LLM on the free tier.
func mockSuggestUpstream() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"{\"intent\":{\"genres\":[\"jazz\"]},\"songs\":[{\"title\":\"A\",\"artist\":\"X\"}]}"}}]}`)
	}))
}

func resolvesToOne() *fakeApple {
	return &fakeApple{resolveFn: func(_ context.Context, _, title, _ string) (*applemusic.Song, bool, error) {
		return &applemusic.Song{ID: "a1", Title: title}, true, nil
	}}
}

// freeHandlers wires a free-tier Handlers backed by MemoryStore + a fixed-key
// verifier, and returns a ready-to-use session token for user "u1".
func freeHandlers(upstreamURL string, perUser int, fake *fakeApple) (*Handlers, string) {
	cfg := &config.Config{UpstreamHTTPTimeout: 5 * time.Second, DefaultLLMKey: "server-key"}
	seed := quota.Config{
		Enabled:           true,
		PerUserDailyLimit: perUser,
		LLMProvider:       "openai-compat",
		LLMBaseURL:        upstreamURL,
		LLMModel:          "m",
	}
	store := quota.NewMemoryStore(seed)
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	verifier := &auth.AppleVerifier{
		BundleID: "com.test",
		Now:      func() time.Time { return time.Now() },
		KeyFunc:  func(*jwt.Token) (any, error) { return &key.PublicKey, nil },
	}
	secret := []byte("sess-secret")
	h := New(cfg, nil, fake).WithFreeTier(store, verifier, secret)
	sess, _ := auth.IssueSession(secret, "u1", time.Hour, time.Now())
	return h, sess
}

func TestFreeTier_SuggestConsumesQuotaThen429(t *testing.T) {
	up := mockSuggestUpstream()
	defer up.Close()
	h, sess := freeHandlers(up.URL, 1, resolvesToOne()) // limit 1/day
	r := freeRouter(h)
	body := map[string]any{"storefront": "us", "text": "jazz"}
	bearer := map[string]string{"Authorization": "Bearer " + sess}

	// 1st call: allowed via free tier (no X-LLM-Api-Key).
	if w := do(r, http.MethodPost, "/api/suggest", bearer, body); w.Code != http.StatusOK {
		t.Fatalf("1st suggest: status %d body %s", w.Code, w.Body.String())
	}
	// 2nd call: quota exhausted → 429.
	w := do(r, http.MethodPost, "/api/suggest", bearer, body)
	if w.Code != http.StatusTooManyRequests || errCode(w) != "quota_exceeded" {
		t.Fatalf("2nd suggest: want 429 quota_exceeded, got %d %s", w.Code, errCode(w))
	}
}

func TestFreeTier_NoSessionNoKey(t *testing.T) {
	up := mockSuggestUpstream()
	defer up.Close()
	h, _ := freeHandlers(up.URL, 5, resolvesToOne())
	r := freeRouter(h)
	// No X-LLM-Api-Key and no Bearer → must demand sign-in.
	w := do(r, http.MethodPost, "/api/suggest", nil, map[string]any{"storefront": "us", "text": "jazz"})
	if w.Code != http.StatusUnauthorized || errCode(w) != "no_session" {
		t.Fatalf("want 401 no_session, got %d %s", w.Code, errCode(w))
	}
}

func TestFreeTier_RefundOnUpstreamError(t *testing.T) {
	// Default LLM returns 401 → SuggestSongs errors → quota must be refunded.
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"bad key"}}`)
	}))
	defer bad.Close()
	h, sess := freeHandlers(bad.URL, 2, resolvesToOne())
	r := freeRouter(h)

	w := do(r, http.MethodPost, "/api/suggest",
		map[string]string{"Authorization": "Bearer " + sess},
		map[string]any{"storefront": "us", "text": "jazz"})
	if w.Code == http.StatusOK {
		t.Fatalf("expected an upstream error, got 200")
	}
	if used, _ := h.quota.GetUserUsage(context.Background(), "u1", quota.Day(time.Now())); used != 0 {
		t.Fatalf("usage after failed call = %d, want 0 (refunded)", used)
	}
}

func TestFreeTier_BYOKStillUnlimited(t *testing.T) {
	up := mockSuggestUpstream()
	defer up.Close()
	// per-user limit 1, but a BYOK key must bypass quota entirely.
	h, _ := freeHandlers(up.URL, 1, resolvesToOne())
	r := freeRouter(h)
	byok := map[string]string{"X-LLM-Api-Key": "user-key"}
	body := map[string]any{
		"llm":        map[string]any{"provider": "openai-compat", "baseUrl": up.URL, "model": "m"},
		"storefront": "us", "text": "jazz",
	}
	for i := 1; i <= 3; i++ {
		if w := do(r, http.MethodPost, "/api/suggest", byok, body); w.Code != http.StatusOK {
			t.Fatalf("BYOK call %d: status %d body %s", i, w.Code, w.Body.String())
		}
	}
	// And the free-tier counter was never touched.
	if used, _ := h.quota.GetUserUsage(context.Background(), "u1", quota.Day(time.Now())); used != 0 {
		t.Fatalf("BYOK should not consume free quota, usage=%d", used)
	}
}

func TestFreeTier_AppleAuthIssuesSession(t *testing.T) {
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	secret := []byte("sess-secret")
	cfg := &config.Config{UpstreamHTTPTimeout: 5 * time.Second, DefaultLLMKey: "server-key"}
	store := quota.NewMemoryStore(quota.Config{Enabled: true, PerUserDailyLimit: 5, LLMProvider: "openai-compat", LLMBaseURL: "http://x", LLMModel: "m"})
	verifier := &auth.AppleVerifier{
		BundleID: "com.test",
		Now:      func() time.Time { return now },
		KeyFunc:  func(*jwt.Token) (any, error) { return &key.PublicKey, nil },
	}
	h := New(cfg, nil, nil).WithFreeTier(store, verifier, secret)
	r := freeRouter(h)

	// Build a token like Apple's, signed with our test key.
	claims := jwt.RegisteredClaims{
		Issuer:    "https://appleid.apple.com",
		Subject:   "apple-user-1",
		Audience:  jwt.ClaimStrings{"com.test"},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = "k1"
	signed, _ := tok.SignedString(key)

	w := do(r, http.MethodPost, "/api/auth/apple", nil, map[string]any{"identityToken": signed})
	if w.Code != http.StatusOK {
		t.Fatalf("auth/apple status %d body %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Session string `json:"session"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.Session == "" {
		t.Fatal("expected a session token")
	}
	if sub, err := auth.VerifySession(secret, resp.Data.Session, time.Now()); err != nil || sub != "apple-user-1" {
		t.Fatalf("issued session verify: sub=%q err=%v", sub, err)
	}

	// A garbage identity token is rejected.
	w = do(r, http.MethodPost, "/api/auth/apple", nil, map[string]any{"identityToken": "not-a-jwt"})
	if w.Code != http.StatusUnauthorized || errCode(w) != "apple_auth_failed" {
		t.Fatalf("bad token: want 401 apple_auth_failed, got %d %s", w.Code, errCode(w))
	}
}
