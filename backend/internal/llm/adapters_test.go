package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ── shared fixtures ─────────────────────────────────────────────────────
const intentJSON = `{"moods":["慵懒"],"genres":["爵士"],"instruments":["钢琴"],"tempo":"slow","keywords":["雨天"],"seed_artists":[]}`
const rankJSON = `{"playlist_name":"雨夜爵士","description":"d","songs":[{"id":"1","reason":"r"},{"id":"x","reason":"fabricated"}]}`

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
func openAIResp(content string) string {
	return mustJSON(map[string]any{"choices": []map[string]any{{"message": map[string]any{"content": content}}}})
}
func anthropicResp(content string) string {
	return mustJSON(map[string]any{"content": []map[string]any{{"type": "text", "text": content}}})
}
func geminiResp(content string) string {
	return mustJSON(map[string]any{"candidates": []map[string]any{{"content": map[string]any{"parts": []map[string]any{{"text": content}}}}}})
}

func cfg(provider ProviderType, baseURL string) Config {
	return Config{Provider: provider, BaseURL: baseURL, Model: "test-model", APIKey: "secret-key", Timeout: 5 * time.Second}
}

// assertIntent checks the canonical fixture parsed correctly.
func assertIntent(t *testing.T, in *Intent) {
	t.Helper()
	if len(in.Genres) != 1 || in.Genres[0] != "爵士" {
		t.Errorf("genres = %v, want [爵士]", in.Genres)
	}
	if in.Tempo != "slow" {
		t.Errorf("tempo = %q, want slow", in.Tempo)
	}
}

// ── OpenAI-compatible ───────────────────────────────────────────────────
func TestOpenAICompat_RequestAndParse(t *testing.T) {
	var auth, path, model string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		path = r.URL.Path
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		model = body.Model
		_, _ = io.WriteString(w, openAIResp(intentJSON))
	}))
	defer srv.Close()

	p := newOpenAICompat(cfg(ProviderOpenAICompat, srv.URL))
	in, err := p.ParseIntent(context.Background(), "雨天慵懒爵士钢琴", nil)
	if err != nil {
		t.Fatalf("ParseIntent: %v", err)
	}
	if auth != "Bearer secret-key" {
		t.Errorf("Authorization = %q", auth)
	}
	if path != "/chat/completions" {
		t.Errorf("path = %q", path)
	}
	if model != "test-model" {
		t.Errorf("model = %q", model)
	}
	assertIntent(t, in)
}

func TestOpenAICompat_RankFiltersFabricated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, openAIResp(rankJSON))
	}))
	defer srv.Close()

	p := newOpenAICompat(cfg(ProviderOpenAICompat, srv.URL))
	res, err := p.RankSongs(context.Background(), &Intent{}, []Candidate{{ID: "1", Title: "t", Artist: "a"}}, "")
	if err != nil {
		t.Fatalf("RankSongs: %v", err)
	}
	// rankJSON includes a fabricated id "x" that must be dropped.
	if len(res.Songs) != 1 || res.Songs[0].ID != "1" {
		t.Fatalf("expected only id=1, got %+v", res.Songs)
	}
}

func TestOpenAICompat_ErrorNormalization(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   ErrorKind
	}{
		{http.StatusUnauthorized, `{"error":{"message":"invalid api key"}}`, ErrAuth},
		{http.StatusTooManyRequests, `{"error":{"message":"Rate limit reached"}}`, ErrRateLimit},
		{http.StatusTooManyRequests, `{"error":{"message":"You exceeded your insufficient_quota"}}`, ErrQuota},
		{http.StatusNotFound, `{"error":{"message":"The model does not exist"}}`, ErrModelNotFound},
		{http.StatusInternalServerError, `boom`, ErrUpstream},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = io.WriteString(w, tc.body)
		}))
		p := newOpenAICompat(cfg(ProviderOpenAICompat, srv.URL))
		err := p.Ping(context.Background())
		srv.Close()

		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Errorf("status %d: want *APIError, got %v", tc.status, err)
			continue
		}
		if apiErr.Kind != tc.want {
			t.Errorf("status %d: kind = %s, want %s", tc.status, apiErr.Kind, tc.want)
		}
	}
}

// ── Anthropic ───────────────────────────────────────────────────────────
func TestAnthropic_RequestAndParse(t *testing.T) {
	var apiKey, version, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey = r.Header.Get("x-api-key")
		version = r.Header.Get("anthropic-version")
		path = r.URL.Path
		_, _ = io.WriteString(w, anthropicResp(intentJSON))
	}))
	defer srv.Close()

	p := newAnthropic(cfg(ProviderAnthropic, srv.URL))
	in, err := p.ParseIntent(context.Background(), "雨天慵懒爵士钢琴", nil)
	if err != nil {
		t.Fatalf("ParseIntent: %v", err)
	}
	if apiKey != "secret-key" {
		t.Errorf("x-api-key = %q", apiKey)
	}
	if version != anthropicVersion {
		t.Errorf("anthropic-version = %q", version)
	}
	if path != "/v1/messages" {
		t.Errorf("path = %q", path)
	}
	assertIntent(t, in)
}

// ── Gemini ──────────────────────────────────────────────────────────────
func TestGemini_RequestAndParse(t *testing.T) {
	var apiKeyHeader, path, rawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKeyHeader = r.Header.Get("x-goog-api-key")
		path = r.URL.Path
		rawQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, geminiResp(intentJSON))
	}))
	defer srv.Close()

	c := cfg(ProviderGemini, srv.URL)
	c.Model = "models/gemini-test" // the "models/" prefix must be stripped
	p := newGemini(c)
	in, err := p.ParseIntent(context.Background(), "雨天慵懒爵士钢琴", nil)
	if err != nil {
		t.Fatalf("ParseIntent: %v", err)
	}
	if apiKeyHeader != "secret-key" {
		t.Errorf("x-goog-api-key = %q", apiKeyHeader)
	}
	if path != "/v1beta/models/gemini-test:generateContent" {
		t.Errorf("path = %q", path)
	}
	// Security: the key must NOT be smuggled in the URL (no ?key=...).
	if strings.Contains(rawQuery, "secret-key") {
		t.Errorf("API key leaked into URL query: %q", rawQuery)
	}
	assertIntent(t, in)
}

// ── ListModels (best-effort model discovery) ────────────────────────────
func TestOpenAICompat_ListModels(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = io.WriteString(w, mustJSON(map[string]any{"object": "list", "data": []map[string]any{
			{"id": "gpt-4o"}, {"id": "gpt-4o-mini"},
		}}))
	}))
	defer srv.Close()

	p := newOpenAICompat(cfg(ProviderOpenAICompat, srv.URL))
	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if path != "/models" {
		t.Errorf("path = %q, want /models", path)
	}
	if len(models) != 2 || models[0].ID != "gpt-4o" {
		t.Fatalf("models = %+v", models)
	}
}

func TestAnthropic_ListModels(t *testing.T) {
	var apiKey, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey = r.Header.Get("x-api-key")
		path = r.URL.Path
		_, _ = io.WriteString(w, mustJSON(map[string]any{"data": []map[string]any{
			{"id": "claude-sonnet-4-6", "display_name": "Claude Sonnet 4.6"},
			{"id": "claude-opus-4-8", "display_name": "Claude Opus 4.8"},
		}}))
	}))
	defer srv.Close()

	p := newAnthropic(cfg(ProviderAnthropic, srv.URL))
	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if apiKey != "secret-key" {
		t.Errorf("x-api-key = %q", apiKey)
	}
	if path != "/v1/models" {
		t.Errorf("path = %q, want /v1/models", path)
	}
	if len(models) != 2 || models[1].DisplayName != "Claude Opus 4.8" {
		t.Fatalf("models = %+v", models)
	}
}

func TestGemini_ListModels(t *testing.T) {
	var apiKeyHeader, rawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKeyHeader = r.Header.Get("x-goog-api-key")
		rawQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, mustJSON(map[string]any{"models": []map[string]any{
			{"name": "models/gemini-2.0-flash", "displayName": "Gemini 2.0 Flash", "supportedGenerationMethods": []string{"generateContent", "countTokens"}},
			{"name": "models/text-embedding-004", "displayName": "Embedding", "supportedGenerationMethods": []string{"embedContent"}},
		}}))
	}))
	defer srv.Close()

	p := newGemini(cfg(ProviderGemini, srv.URL))
	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if apiKeyHeader != "secret-key" {
		t.Errorf("x-goog-api-key = %q", apiKeyHeader)
	}
	if strings.Contains(rawQuery, "secret-key") {
		t.Errorf("API key leaked into URL query: %q", rawQuery)
	}
	// Only the generateContent model survives, with the "models/" prefix stripped.
	if len(models) != 1 || models[0].ID != "gemini-2.0-flash" {
		t.Fatalf("models = %+v, want [gemini-2.0-flash]", models)
	}
}
