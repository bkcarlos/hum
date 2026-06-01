package handlers

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bkcarlos/apple-music-llm/internal/applemusic"
	"github.com/bkcarlos/apple-music-llm/internal/httpx"
	"github.com/bkcarlos/apple-music-llm/internal/llm"
)

const (
	perTermLimit = 25  // Apple catalog search max results per request
	maxPoolSize  = 150 // candidate pool cap (F4: target 50–150, de-duplicated)
)

// DeveloperToken (GET /api/apple/developer-token) returns the JWT MusicKit JS
// uses to initialize (F1). The .p8 private key never leaves the backend.
func (h *Handlers) DeveloperToken(c *gin.Context) {
	if h.tokens == nil {
		httpx.Fail(c, http.StatusServiceUnavailable, "apple_unconfigured",
			"后端未配置 Apple 开发者凭证（APPLE_TEAM_ID / APPLE_KEY_ID / .p8）。")
		return
	}
	tok, err := h.tokens.Token()
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, "token_error", "签发 Developer Token 失败。")
		return
	}
	httpx.OK(c, gin.H{
		"token":     tok,
		"expiresAt": h.tokens.ExpiresAt().UTC().Format(time.RFC3339),
	})
}

// Search (POST /api/apple/search) builds a real candidate pool from the user's
// storefront (F4). storefront MUST be the user's own — it flows from F1 and is
// reused verbatim for ranking and playlist creation (id-consistency).
func (h *Handlers) Search(c *gin.Context) {
	if h.apple == nil {
		httpx.Fail(c, http.StatusServiceUnavailable, "apple_unconfigured", "后端未配置 Apple 开发者凭证。")
		return
	}
	var body struct {
		Storefront string      `json:"storefront"`
		Intent     *llm.Intent `json:"intent"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Fail(c, http.StatusBadRequest, "bad_request", "请求体无效。")
		return
	}
	if strings.TrimSpace(body.Storefront) == "" {
		httpx.Fail(c, http.StatusBadRequest, "bad_request", "缺少 storefront（请先授权 Apple Music）。")
		return
	}

	terms := buildSearchTerms(body.Intent)

	// Fan the per-term searches out concurrently; collect in term order so dedup
	// (and which songs survive the cap) stay deterministic.
	results := make([][]applemusic.Song, len(terms))
	errs := make([]error, len(terms))
	var wg sync.WaitGroup
	for i, term := range terms {
		wg.Add(1)
		go func(i int, term string) {
			defer wg.Done()
			results[i], errs[i] = h.apple.SearchSongs(c.Request.Context(), body.Storefront, term, perTermLimit)
		}(i, term)
	}
	wg.Wait()

	seen := make(map[string]struct{}, maxPoolSize)
	pool := make([]applemusic.Song, 0, maxPoolSize)
	var firstErr error
	for i := range results {
		if errs[i] != nil {
			if firstErr == nil {
				firstErr = errs[i]
			}
			continue
		}
		for _, s := range results[i] {
			if _, dup := seen[s.ID]; dup {
				continue
			}
			seen[s.ID] = struct{}{}
			pool = append(pool, s)
		}
	}
	if len(pool) > maxPoolSize {
		pool = pool[:maxPoolSize]
	}

	if len(pool) == 0 {
		if firstErr != nil {
			httpx.Fail(c, http.StatusBadGateway, "apple_error", "Apple Music 检索失败："+firstErr.Error())
			return
		}
		httpx.Fail(c, http.StatusNotFound, "empty_pool", "没有搜到符合条件的歌曲，请调整描述。")
		return
	}
	httpx.OK(c, gin.H{"storefront": body.Storefront, "candidates": pool, "count": len(pool)})
}

// CreatePlaylist (POST /api/apple/playlists) writes a private playlist into the
// user's library (F7). Requires the Music User Token header.
func (h *Handlers) CreatePlaylist(c *gin.Context) {
	if h.apple == nil {
		httpx.Fail(c, http.StatusServiceUnavailable, "apple_unconfigured", "后端未配置 Apple 开发者凭证。")
		return
	}
	userToken := strings.TrimSpace(c.GetHeader(musicUserTokenHeader))
	if userToken == "" {
		httpx.Fail(c, http.StatusUnauthorized, "no_user_token", "请先连接并授权 Apple Music。")
		return
	}
	var body struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		SongIDs     []string `json:"songIds"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Fail(c, http.StatusBadRequest, "bad_request", "请求体无效。")
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		httpx.Fail(c, http.StatusBadRequest, "bad_request", "请填写歌单名。")
		return
	}
	if len(body.SongIDs) == 0 {
		httpx.Fail(c, http.StatusBadRequest, "bad_request", "请至少勾选一首歌。")
		return
	}
	pl, err := h.apple.CreatePlaylist(c.Request.Context(), userToken, body.Name, body.Description, body.SongIDs)
	if err != nil {
		httpx.Fail(c, http.StatusBadGateway, "apple_error", "创建歌单失败："+err.Error())
		return
	}
	httpx.OK(c, pl)
}

// buildSearchTerms derives a handful of catalog search queries from an Intent,
// widening the candidate pool. Heuristic and intentionally simple for the
// scaffold — tune in M2 when wiring real relevance.
func buildSearchTerms(intent *llm.Intent) []string {
	if intent == nil {
		return []string{"top songs"}
	}
	var terms []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" {
			terms = append(terms, s)
		}
	}

	// Primary combined query.
	combined := append(append(append([]string{}, intent.Genres...), intent.Moods...), intent.Keywords...)
	add(strings.Join(combined, " "))

	// Per-genre queries, lightly seasoned with the first mood/instrument.
	var flavor string
	if len(intent.Moods) > 0 {
		flavor = intent.Moods[0]
	} else if len(intent.Instruments) > 0 {
		flavor = intent.Instruments[0]
	}
	for _, g := range intent.Genres {
		add(strings.TrimSpace(g + " " + flavor))
	}
	// Seed artists each get their own query.
	for _, a := range intent.SeedArtists {
		add(a)
	}

	if len(terms) == 0 {
		add(strings.Join(intent.Instruments, " "))
	}
	if len(terms) == 0 {
		terms = []string{"top songs"}
	}
	return dedupeStrings(terms)
}

func dedupeStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := in[:0]
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
