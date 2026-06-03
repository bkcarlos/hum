package applemusic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/bkcarlos/hum/internal/cache"
)

// DefaultAPIBase is the public Apple Music API host.
const DefaultAPIBase = "https://api.music.apple.com"

// Song is a trimmed catalog track returned to the frontend / LLM ranker.
// IDs are storefront-specific: the SAME storefront must be used for search,
// preview, and playlist creation (docs/requirements.md F4 id-consistency).
type Song struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Artist        string   `json:"artist"`
	Album         string   `json:"album"`
	Genres        []string `json:"genres"`
	DurationMs    int      `json:"durationMs"`
	ArtworkURL    string   `json:"artworkUrl"`
	PreviewURL    string   `json:"previewUrl"`
	ReleaseDate   string   `json:"releaseDate,omitempty"`   // e.g. "1959-08-17" or "1959"
	ContentRating string   `json:"contentRating,omitempty"` // "clean" | "explicit" | ""
	HasLyrics     bool     `json:"hasLyrics"`               // false ⇒ likely instrumental
	ISRC          string   `json:"isrc,omitempty"`          // recording code; key for 3rd-party audio features
	Composer      string   `json:"composer,omitempty"`
}

// Playlist is the result of a library playlist creation (F7).
type Playlist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Client talks to the Apple Music HTTP API using a Developer Token (and, for
// library writes, a per-request Music User Token). Catalog search results are
// cached per (storefront, term, limit) to cut API calls (F4).
type Client struct {
	tokens  *TokenManager
	http    *http.Client
	baseURL string
	cache   *cache.TTL[[]Song]
}

// NewClient builds a client. baseURL falls back to DefaultAPIBase when empty;
// cacheTTL <= 0 disables search caching.
func NewClient(tm *TokenManager, timeout time.Duration, baseURL string, cacheTTL time.Duration) *Client {
	if baseURL == "" {
		baseURL = DefaultAPIBase
	}
	return &Client{
		tokens:  tm,
		http:    &http.Client{Timeout: timeout},
		baseURL: strings.TrimRight(baseURL, "/"),
		cache:   cache.NewTTL[[]Song](cacheTTL),
	}
}

// SearchSongs queries the catalog for a storefront. limit is clamped to [1,25]
// (Apple's per-request max for search).
func (c *Client) SearchSongs(ctx context.Context, storefront, term string, limit int) ([]Song, error) {
	if storefront == "" {
		return nil, fmt.Errorf("applemusic: storefront is required")
	}
	if limit <= 0 || limit > 25 {
		limit = 25
	}
	cacheKey := storefront + "\x00" + strconv.Itoa(limit) + "\x00" + term
	if hit, ok := c.cache.Get(cacheKey); ok {
		return hit, nil
	}

	q := url.Values{}
	q.Set("term", term)
	q.Set("types", "songs")
	q.Set("limit", strconv.Itoa(limit))
	endpoint := fmt.Sprintf("%s/v1/catalog/%s/search?%s", c.baseURL, url.PathEscape(storefront), q.Encode())

	body, err := c.do(ctx, http.MethodGet, endpoint, "", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Results struct {
			Songs struct {
				Data []songResource `json:"data"`
			} `json:"songs"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("applemusic: decoding search response: %w", err)
	}
	out := make([]Song, 0, len(resp.Results.Songs.Data))
	for _, r := range resp.Results.Songs.Data {
		out = append(out, r.toSong())
	}
	c.cache.Set(cacheKey, out)
	return out, nil
}

// ResolveSong grounds an LLM-proposed (title, artist) against the real catalog
// (Option A): it searches the user's storefront and returns the best track whose
// title — and, when possible, artist — matches. ok=false means nothing plausibly
// matched, so a fabricated or misattributed suggestion is dropped; Apple stays
// the source of truth for what actually exists (golden rule).
func (c *Client) ResolveSong(ctx context.Context, storefront, title, artist string) (*Song, bool, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, false, nil
	}
	term := strings.TrimSpace(artist + " " + title)
	songs, err := c.SearchSongs(ctx, storefront, term, 10)
	if err != nil {
		return nil, false, err
	}
	wantTitle, wantArtist := normalizeMatch(title), normalizeMatch(artist)
	var titleOnly *Song
	for i := range songs {
		if !titleMatches(normalizeMatch(songs[i].Title), wantTitle) {
			continue
		}
		if wantArtist != "" && strings.Contains(normalizeMatch(songs[i].Artist), wantArtist) {
			return &songs[i], true, nil // strongest: title + artist agree
		}
		if titleOnly == nil {
			titleOnly = &songs[i]
		}
	}
	if titleOnly != nil {
		return titleOnly, true, nil // title matched; artist may differ (e.g. cross-language name)
	}
	return nil, false, nil
}

// normalizeMatch lowercases and strips everything but letters/digits (spaces and
// punctuation included), so "Blue in Green" ~ "blueingreen"; CJK is preserved.
func normalizeMatch(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// titleMatches treats equality or either-contains-other as a match, tolerating
// suffixes like " (Live)" / " - Remastered".
func titleMatches(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return a == b || strings.Contains(a, b) || strings.Contains(b, a)
}

// CreatePlaylist creates a private playlist in the user's library and adds the
// given catalog song IDs. Requires the user's Music User Token.
func (c *Client) CreatePlaylist(ctx context.Context, userToken, name, description string, catalogSongIDs []string) (*Playlist, error) {
	if userToken == "" {
		return nil, fmt.Errorf("applemusic: music user token is required to create a playlist")
	}
	type rel struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	tracks := make([]rel, 0, len(catalogSongIDs))
	for _, id := range catalogSongIDs {
		tracks = append(tracks, rel{ID: id, Type: "songs"})
	}
	payload := map[string]any{
		"attributes": map[string]string{"name": name, "description": description},
		"relationships": map[string]any{
			"tracks": map[string]any{"data": tracks},
		},
	}
	raw, _ := json.Marshal(payload)

	body, err := c.do(ctx, http.MethodPost, c.baseURL+"/v1/me/library/playlists", userToken, raw)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				Name string `json:"name"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("applemusic: decoding create-playlist response: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("applemusic: create-playlist returned no data")
	}
	p := resp.Data[0]
	return &Playlist{
		ID:   p.ID,
		Name: p.Attributes.Name,
		// Best-effort deep link; library playlists are private (F7).
		URL: "https://music.apple.com/library/playlist/" + p.ID,
	}, nil
}

// do performs an Apple Music request with a fresh Developer Token. When
// userToken is non-empty it is attached as the Music-User-Token header.
func (c *Client) do(ctx context.Context, method, endpoint, userToken string, body []byte) ([]byte, error) {
	devToken, err := c.tokens.Token()
	if err != nil {
		return nil, err
	}
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+devToken)
	if userToken != "" {
		req.Header.Set("Music-User-Token", userToken)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("applemusic: request failed: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("applemusic: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}

// songResource mirrors the relevant subset of an Apple Music song object.
type songResource struct {
	ID         string `json:"id"`
	Attributes struct {
		Name             string   `json:"name"`
		ArtistName       string   `json:"artistName"`
		AlbumName        string   `json:"albumName"`
		GenreNames       []string `json:"genreNames"`
		DurationInMillis int      `json:"durationInMillis"`
		ReleaseDate      string   `json:"releaseDate"`
		ContentRating    string   `json:"contentRating"`
		HasLyrics        bool     `json:"hasLyrics"`
		ISRC             string   `json:"isrc"`
		ComposerName     string   `json:"composerName"`
		Artwork          struct {
			URL string `json:"url"`
		} `json:"artwork"`
		Previews []struct {
			URL string `json:"url"`
		} `json:"previews"`
	} `json:"attributes"`
}

func (r songResource) toSong() Song {
	s := Song{
		ID:            r.ID,
		Title:         r.Attributes.Name,
		Artist:        r.Attributes.ArtistName,
		Album:         r.Attributes.AlbumName,
		Genres:        r.Attributes.GenreNames,
		DurationMs:    r.Attributes.DurationInMillis,
		ArtworkURL:    renderArtwork(r.Attributes.Artwork.URL, 240, 240),
		ReleaseDate:   r.Attributes.ReleaseDate,
		ContentRating: r.Attributes.ContentRating,
		HasLyrics:     r.Attributes.HasLyrics,
		ISRC:          r.Attributes.ISRC,
		Composer:      r.Attributes.ComposerName,
	}
	if len(r.Attributes.Previews) > 0 {
		s.PreviewURL = r.Attributes.Previews[0].URL
	}
	return s
}

// renderArtwork fills Apple's {w}/{h} artwork URL template with a pixel size.
func renderArtwork(tmpl string, w, h int) string {
	if tmpl == "" {
		return ""
	}
	tmpl = strings.ReplaceAll(tmpl, "{w}", strconv.Itoa(w))
	tmpl = strings.ReplaceAll(tmpl, "{h}", strconv.Itoa(h))
	return tmpl
}
