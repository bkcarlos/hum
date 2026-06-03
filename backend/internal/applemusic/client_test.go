package applemusic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestClient(t *testing.T, baseURL string, cacheTTL time.Duration) *Client {
	t.Helper()
	_, path := writeTempP8(t) // helper from token_test.go
	tm, err := NewTokenManager("TEAM", "KID", path, 48*time.Hour)
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	return NewClient(tm, 5*time.Second, baseURL, cacheTTL)
}

func TestSearchSongs_ParsesAndAuthorizes(t *testing.T) {
	var auth, path, query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, path, query = r.Header.Get("Authorization"), r.URL.Path, r.URL.RawQuery
		_, _ = io.WriteString(w, `{"results":{"songs":{"data":[{"id":"123","attributes":{`+
			`"name":"Blue in Green","artistName":"Miles Davis","albumName":"Kind of Blue",`+
			`"genreNames":["Jazz"],"durationInMillis":327000,`+
			`"hasLyrics":true,"contentRating":"explicit","releaseDate":"1959-08-17",`+
			`"isrc":"USSM15900001","composerName":"Bill Evans",`+
			`"artwork":{"url":"https://ex/{w}x{h}.jpg"},"previews":[{"url":"https://ex/p.m4a"}]}}]}}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, 0)
	songs, err := c.SearchSongs(context.Background(), "us", "jazz piano", 25)
	if err != nil {
		t.Fatalf("SearchSongs: %v", err)
	}
	if len(songs) != 1 {
		t.Fatalf("len(songs) = %d, want 1", len(songs))
	}
	s := songs[0]
	if s.ID != "123" || s.Title != "Blue in Green" || s.Artist != "Miles Davis" {
		t.Errorf("song = %+v", s)
	}
	if s.ArtworkURL != "https://ex/240x240.jpg" {
		t.Errorf("artwork = %q, want rendered 240x240", s.ArtworkURL)
	}
	if s.PreviewURL == "" {
		t.Error("expected a preview URL")
	}
	if !s.HasLyrics || s.ContentRating != "explicit" || s.ReleaseDate != "1959-08-17" || s.ISRC != "USSM15900001" || s.Composer != "Bill Evans" {
		t.Errorf("extended attrs not parsed: %+v", s)
	}
	if !strings.HasPrefix(auth, "Bearer ") {
		t.Errorf("Authorization = %q, want Bearer <token>", auth)
	}
	if path != "/v1/catalog/us/search" {
		t.Errorf("path = %q", path)
	}
	if !strings.Contains(query, "types=songs") || !strings.Contains(query, "limit=25") {
		t.Errorf("query = %q", query)
	}
}

func TestSearchSongs_UsesCache(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = io.WriteString(w, `{"results":{"songs":{"data":[{"id":"1","attributes":{"name":"n","artistName":"a"}}]}}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, time.Minute) // caching on
	_, _ = c.SearchSongs(context.Background(), "us", "jazz", 25)
	_, _ = c.SearchSongs(context.Background(), "us", "jazz", 25)
	if calls != 1 {
		t.Errorf("upstream called %d times, want 1 (cache hit expected)", calls)
	}
}

func TestResolveSong_VerifiesMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"results":{"songs":{"data":[`+
			`{"id":"1","attributes":{"name":"Blue in Green","artistName":"Miles Davis"}},`+
			`{"id":"2","attributes":{"name":"So What","artistName":"Miles Davis"}}]}}}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, 0)

	// Exact title + artist resolves to the matching track.
	s, ok, err := c.ResolveSong(context.Background(), "us", "Blue in Green", "Miles Davis")
	if err != nil || !ok || s == nil || s.ID != "1" {
		t.Fatalf("expected match id=1, got ok=%v song=%+v err=%v", ok, s, err)
	}
	// A title absent from the catalog results is dropped (golden rule: Apple decides).
	if _, ok, _ := c.ResolveSong(context.Background(), "us", "A Tune That Does Not Exist", "Miles Davis"); ok {
		t.Errorf("expected no match for a title absent from results")
	}
}

func TestCreatePlaylist_BuildsRequest(t *testing.T) {
	var userToken, path, method string
	var payload struct {
		Attributes struct {
			Name        string
			Description string
		}
		Relationships struct {
			Tracks struct {
				Data []struct {
					ID   string
					Type string
				}
			}
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userToken, path, method = r.Header.Get("Music-User-Token"), r.URL.Path, r.Method
		_ = json.NewDecoder(r.Body).Decode(&payload)
		_, _ = io.WriteString(w, `{"data":[{"id":"p.abc","attributes":{"name":"雨夜爵士"}}]}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, 0)
	pl, err := c.CreatePlaylist(context.Background(), "user-tok-123", "雨夜爵士", "desc", []string{"1", "2"})
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}
	if pl.ID != "p.abc" || pl.Name != "雨夜爵士" || !strings.Contains(pl.URL, "p.abc") {
		t.Errorf("playlist = %+v", pl)
	}
	if userToken != "user-tok-123" {
		t.Errorf("Music-User-Token = %q", userToken)
	}
	if method != http.MethodPost || path != "/v1/me/library/playlists" {
		t.Errorf("%s %s", method, path)
	}
	if len(payload.Relationships.Tracks.Data) != 2 || payload.Relationships.Tracks.Data[0].Type != "songs" {
		t.Errorf("tracks = %+v", payload.Relationships.Tracks.Data)
	}
	if payload.Attributes.Name != "雨夜爵士" {
		t.Errorf("attributes.name = %q", payload.Attributes.Name)
	}
}
