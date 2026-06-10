package handlers

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// A client that posts an oversized candidate pool to /rank must not have it fed
// verbatim into the (server-paid, on the free tier) LLM prompt: the handler caps
// at maxPoolSize. The mock upstream counts "id=" lines, one per candidate.
func TestRank_CapsOversizedCandidatePool(t *testing.T) {
	var sawCandidateLines int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		sawCandidateLines = strings.Count(string(b), "id=")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"{\"playlist_name\":\"p\",\"songs\":[{\"id\":\"c0\",\"reason\":\"r\"}]}"}}]}`)
	}))
	defer srv.Close()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/rank", New(baseCfg(), nil, nil).Rank)

	cands := make([]map[string]any, 0, 300)
	for i := 0; i < 300; i++ {
		cands = append(cands, map[string]any{"id": fmt.Sprintf("c%d", i), "title": "t", "artist": "a"})
	}
	body := map[string]any{
		"llm":        map[string]any{"provider": "openai-compat", "baseUrl": srv.URL, "model": "m"},
		"intent":     map[string]any{},
		"candidates": cands,
	}
	w := do(r, http.MethodPost, "/api/rank", map[string]string{"X-LLM-Api-Key": "k"}, body)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if sawCandidateLines != maxPoolSize {
		t.Fatalf("upstream prompt carried %d candidates, want capped at %d", sawCandidateLines, maxPoolSize)
	}
}
