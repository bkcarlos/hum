package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bkcarlos/hum/internal/applemusic"
	"github.com/bkcarlos/hum/internal/config"
	"github.com/bkcarlos/hum/internal/llm"
)

// ── test scaffolding ────────────────────────────────────────────────────
type fakeApple struct {
	searchFn  func(ctx context.Context, storefront, term string, limit int) ([]applemusic.Song, error)
	resolveFn func(ctx context.Context, storefront, title, artist string) (*applemusic.Song, bool, error)
	createFn  func(ctx context.Context, userToken, name, desc string, ids []string) (*applemusic.Playlist, error)
}

func (f *fakeApple) SearchSongs(ctx context.Context, sf, term string, limit int) ([]applemusic.Song, error) {
	return f.searchFn(ctx, sf, term, limit)
}
func (f *fakeApple) ResolveSong(ctx context.Context, sf, title, artist string) (*applemusic.Song, bool, error) {
	return f.resolveFn(ctx, sf, title, artist)
}
func (f *fakeApple) CreatePlaylist(ctx context.Context, ut, n, d string, ids []string) (*applemusic.Playlist, error) {
	return f.createFn(ctx, ut, n, d, ids)
}

func router(h *Handlers) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/api")
	g.POST("/intent", h.ParseIntent)
	g.POST("/llm/test", h.TestLLM)
	g.POST("/llm/models", h.Models)
	g.GET("/apple/developer-token", h.DeveloperToken)
	g.POST("/apple/search", h.Search)
	g.POST("/suggest", h.Suggest)
	g.POST("/examples", h.Examples)
	g.POST("/apple/playlists", h.CreatePlaylist)
	return r
}

func do(r *gin.Engine, method, path string, headers map[string]string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func errCode(w *httptest.ResponseRecorder) string {
	var m struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return m.Error.Code
}

func baseCfg() *config.Config { return &config.Config{UpstreamHTTPTimeout: 5 * time.Second} }

// ── LLM path ────────────────────────────────────────────────────────────
func TestParseIntent_RequiresKey(t *testing.T) {
	r := router(New(baseCfg(), nil, nil))
	body := map[string]any{
		"llm":  map[string]any{"provider": "openai-compat", "baseUrl": "https://x", "model": "m"},
		"text": "雨天爵士",
	}
	w := do(r, http.MethodPost, "/api/intent", nil, body) // no X-LLM-Api-Key
	if w.Code != http.StatusBadRequest || errCode(w) != "no_key" {
		t.Fatalf("got %d %s, want 400 no_key", w.Code, errCode(w))
	}
}

func TestParseIntent_OK(t *testing.T) {
	// Mock OpenAI-compatible upstream returns a valid intent JSON in content.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"{\"moods\":[],\"genres\":[\"爵士\"],\"instruments\":[],\"tempo\":\"slow\",\"keywords\":[],\"seed_artists\":[]}"}}]}`)
	}))
	defer srv.Close()

	r := router(New(baseCfg(), nil, nil))
	body := map[string]any{
		"llm":  map[string]any{"provider": "openai-compat", "baseUrl": srv.URL, "model": "m"},
		"text": "雨天爵士",
	}
	w := do(r, http.MethodPost, "/api/intent", map[string]string{"X-LLM-Api-Key": "k"}, body)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data llm.Intent `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data.Genres) != 1 || resp.Data.Genres[0] != "爵士" {
		t.Errorf("intent = %+v", resp.Data)
	}
}

// ── Option A: suggest → resolve ─────────────────────────────────────────
func TestSuggest_ResolvesAndDedupes(t *testing.T) {
	// Mock OpenAI-compatible upstream returns an intent + 3 song suggestions.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"{\"intent\":{\"genres\":[\"jazz\"]},\"songs\":[{\"title\":\"A\",\"artist\":\"X\"},{\"title\":\"B\",\"artist\":\"Y\"},{\"title\":\"C\",\"artist\":\"Z\"}]}"}}]}`)
	}))
	defer srv.Close()

	fake := &fakeApple{
		resolveFn: func(_ context.Context, _, title, _ string) (*applemusic.Song, bool, error) {
			switch title {
			case "C":
				return nil, false, nil // unresolved → dropped
			case "B":
				return &applemusic.Song{ID: "shared", Title: "B"}, true, nil
			default: // "A" — shares an id with B, exercising dedup
				return &applemusic.Song{ID: "shared", Title: "A"}, true, nil
			}
		},
	}
	r := router(New(baseCfg(), nil, fake))
	body := map[string]any{
		"llm":        map[string]any{"provider": "openai-compat", "baseUrl": srv.URL, "model": "m"},
		"storefront": "us",
		"text":       "雨天爵士",
	}
	w := do(r, http.MethodPost, "/api/suggest", map[string]string{"X-LLM-Api-Key": "k"}, body)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Candidates []struct {
				ID string `json:"id"`
			} `json:"candidates"`
			Suggested  int      `json:"suggested"`
			Resolved   int      `json:"resolved"`
			Unresolved []string `json:"unresolved"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.Suggested != 3 {
		t.Errorf("suggested = %d, want 3", resp.Data.Suggested)
	}
	if len(resp.Data.Candidates) != 1 {
		t.Errorf("candidates = %d, want 1 (A & B share an id; C unresolved)", len(resp.Data.Candidates))
	}
	if len(resp.Data.Unresolved) != 1 {
		t.Errorf("unresolved = %v, want 1 (C)", resp.Data.Unresolved)
	}
}

func TestExamples_RequiresKey(t *testing.T) {
	r := router(New(baseCfg(), nil, nil))
	body := map[string]any{
		"llm":     map[string]any{"provider": "openai-compat", "baseUrl": "https://x", "model": "m"},
		"context": "周五深夜",
	}
	w := do(r, http.MethodPost, "/api/examples", nil, body) // no X-LLM-Api-Key
	if w.Code != http.StatusBadRequest || errCode(w) != "no_key" {
		t.Fatalf("got %d %s, want 400 no_key", w.Code, errCode(w))
	}
}

func TestModels_RequiresKey(t *testing.T) {
	r := router(New(baseCfg(), nil, nil))
	body := map[string]any{"llm": map[string]any{"provider": "openai-compat", "baseUrl": "https://x"}}
	w := do(r, http.MethodPost, "/api/llm/models", nil, body) // no X-LLM-Api-Key
	if w.Code != http.StatusBadRequest || errCode(w) != "no_key" {
		t.Fatalf("got %d %s, want 400 no_key", w.Code, errCode(w))
	}
}

func TestModels_OK_NoModelNeeded(t *testing.T) {
	// The list endpoint must work WITHOUT a model in the body (we're discovering them).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":[{"id":"gpt-4o"},{"id":"gpt-4o-mini"}]}`)
	}))
	defer srv.Close()

	r := router(New(baseCfg(), nil, nil))
	body := map[string]any{"llm": map[string]any{"provider": "openai-compat", "baseUrl": srv.URL}}
	w := do(r, http.MethodPost, "/api/llm/models", map[string]string{"X-LLM-Api-Key": "k"}, body)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Models []struct {
				ID string `json:"id"`
			} `json:"models"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data.Models) != 2 || resp.Data.Models[0].ID != "gpt-4o" {
		t.Fatalf("models = %+v", resp.Data.Models)
	}
}

// ── Apple guards ────────────────────────────────────────────────────────
func TestDeveloperToken_Unconfigured(t *testing.T) {
	r := router(New(baseCfg(), nil, nil)) // tokens nil
	w := do(r, http.MethodGet, "/api/apple/developer-token", nil, nil)
	if w.Code != http.StatusServiceUnavailable || errCode(w) != "apple_unconfigured" {
		t.Fatalf("got %d %s, want 503 apple_unconfigured", w.Code, errCode(w))
	}
}

func TestSearch_Unconfigured(t *testing.T) {
	r := router(New(baseCfg(), nil, nil)) // apple nil
	w := do(r, http.MethodPost, "/api/apple/search", nil, map[string]any{"storefront": "us", "intent": map[string]any{}})
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503", w.Code)
	}
}

// ── Search concurrency + dedup ──────────────────────────────────────────
func TestSearch_DedupesAcrossTerms(t *testing.T) {
	fake := &fakeApple{
		searchFn: func(_ context.Context, _, term string, _ int) ([]applemusic.Song, error) {
			// Every term yields a shared "dup" plus a term-unique song.
			return []applemusic.Song{{ID: "dup", Title: "shared"}, {ID: "u:" + term, Title: term}}, nil
		},
	}
	r := router(New(baseCfg(), nil, fake))
	body := map[string]any{
		"storefront": "us",
		"intent":     map[string]any{"genres": []string{"jazz", "rock"}, "moods": []string{"calm"}},
	}
	w := do(r, http.MethodPost, "/api/apple/search", nil, body)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Count      int `json:"count"`
			Candidates []struct {
				ID string `json:"id"`
			} `json:"candidates"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	dups := 0
	for _, c := range resp.Data.Candidates {
		if c.ID == "dup" {
			dups++
		}
	}
	if dups != 1 {
		t.Errorf("shared id appeared %d times, want exactly 1 (dedup failed)", dups)
	}
	if resp.Data.Count != len(resp.Data.Candidates) {
		t.Errorf("count %d != len %d", resp.Data.Count, len(resp.Data.Candidates))
	}
}

// ── Playlist creation guards ────────────────────────────────────────────
func TestCreatePlaylist_RequiresUserToken(t *testing.T) {
	fake := &fakeApple{createFn: func(context.Context, string, string, string, []string) (*applemusic.Playlist, error) {
		t.Fatal("createFn should not be called without a user token")
		return nil, nil
	}}
	r := router(New(baseCfg(), nil, fake))
	w := do(r, http.MethodPost, "/api/apple/playlists", nil, map[string]any{"name": "x", "songIds": []string{"1"}})
	if w.Code != http.StatusUnauthorized || errCode(w) != "no_user_token" {
		t.Fatalf("got %d %s, want 401 no_user_token", w.Code, errCode(w))
	}
}

func TestCreatePlaylist_OK(t *testing.T) {
	fake := &fakeApple{createFn: func(_ context.Context, ut, name, _ string, ids []string) (*applemusic.Playlist, error) {
		if ut != "ut-1" || len(ids) != 2 {
			t.Errorf("createFn got ut=%q ids=%v", ut, ids)
		}
		return &applemusic.Playlist{ID: "p.1", Name: name, URL: "https://music.apple.com/library/playlist/p.1"}, nil
	}}
	r := router(New(baseCfg(), nil, fake))
	body := map[string]any{"name": "My AI Mix", "description": "d", "songIds": []string{"1", "2"}}
	w := do(r, http.MethodPost, "/api/apple/playlists", map[string]string{"Music-User-Token": "ut-1"}, body)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data applemusic.Playlist `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.ID != "p.1" {
		t.Errorf("playlist id = %q", resp.Data.ID)
	}
}

// ── pure helper ─────────────────────────────────────────────────────────
func TestBuildSearchTerms(t *testing.T) {
	terms := buildSearchTerms(&llm.Intent{
		Genres:      []string{"jazz"},
		Moods:       []string{"calm"},
		SeedArtists: []string{"Bill Evans"},
	})
	// combined "jazz calm" and per-genre "jazz calm" dedupe to one; seed stands alone.
	if len(terms) != 2 {
		t.Fatalf("terms = %v, want 2 (deduped)", terms)
	}
	has := func(s string) bool {
		for _, x := range terms {
			if x == s {
				return true
			}
		}
		return false
	}
	if !has("jazz calm") || !has("Bill Evans") {
		t.Errorf("terms = %v, want to contain 'jazz calm' and 'Bill Evans'", terms)
	}
}
