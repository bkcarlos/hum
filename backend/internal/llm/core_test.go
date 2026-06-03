package llm

import (
	"context"
	"strings"
	"testing"
)

func TestRankSongs_DropsFabricatedIDs(t *testing.T) {
	// Model returns a real id ("1") plus a fabricated one ("999"), wrapped in a
	// ```json fence. The golden rule says only pool ids may survive.
	c := core{chat: func(_ context.Context, _, _ string) (string, error) {
		return "```json\n{\"playlist_name\":\"X\",\"description\":\"d\"," +
			"\"songs\":[{\"id\":\"1\",\"reason\":\"good\"},{\"id\":\"999\",\"reason\":\"fake\"}]}\n```", nil
	}}
	candidates := []Candidate{{ID: "1", Title: "Real", Artist: "A"}}

	res, err := c.RankSongs(context.Background(), &Intent{}, candidates, "")
	if err != nil {
		t.Fatalf("RankSongs: %v", err)
	}
	if len(res.Songs) != 1 || res.Songs[0].ID != "1" {
		t.Fatalf("expected only id=1 to survive, got %+v", res.Songs)
	}
	if res.PlaylistName != "X" {
		t.Errorf("playlist name = %q, want X", res.PlaylistName)
	}
}

func TestRankSongs_ErrorsWhenNothingValid(t *testing.T) {
	c := core{chat: func(_ context.Context, _, _ string) (string, error) {
		return `{"playlist_name":"X","description":"d","songs":[{"id":"nope","reason":"x"}]}`, nil
	}}
	_, err := c.RankSongs(context.Background(), &Intent{}, []Candidate{{ID: "1"}}, "")
	if err == nil {
		t.Fatal("expected error when no candidate ids survive")
	}
}

func TestSuggestSongs_ParsesIntentAndSongs(t *testing.T) {
	// Model returns a structured intent + concrete song suggestions in a ```json fence.
	c := core{chat: func(_ context.Context, _, _ string) (string, error) {
		return "```json\n{\"intent\":{\"genres\":[\"jazz\"],\"tempo\":\"slow\"}," +
			"\"songs\":[{\"title\":\"Blue in Green\",\"artist\":\"Miles Davis\"}," +
			"{\"title\":\"So What\",\"artist\":\"Miles Davis\"}]}\n```", nil
	}}
	res, err := c.SuggestSongs(context.Background(), "雨天爵士", []string{"Bill Evans"})
	if err != nil {
		t.Fatalf("SuggestSongs: %v", err)
	}
	if len(res.Suggestions) != 2 || res.Suggestions[0].Title != "Blue in Green" || res.Suggestions[0].Artist != "Miles Davis" {
		t.Fatalf("suggestions = %+v", res.Suggestions)
	}
	if len(res.Intent.Genres) != 1 || res.Intent.Genres[0] != "jazz" {
		t.Errorf("intent genres = %+v", res.Intent.Genres)
	}
	// Explicit seed carried through even when the model omits it from the intent.
	if len(res.Intent.SeedArtists) != 1 || res.Intent.SeedArtists[0] != "Bill Evans" {
		t.Errorf("seed not carried through: %+v", res.Intent.SeedArtists)
	}
}

func TestRankSongs_PromptIncludesAttributes(t *testing.T) {
	var gotUser string
	c := core{chat: func(_ context.Context, _, user string) (string, error) {
		gotUser = user
		return `{"playlist_name":"X","description":"d","songs":[{"id":"1","reason":"r"}]}`, nil
	}}
	cands := []Candidate{{ID: "1", Title: "T", Artist: "A", Year: "2020", HasLyrics: false, ContentRating: "explicit"}}
	if _, err := c.RankSongs(context.Background(), &Intent{}, cands, "去掉有歌词的"); err != nil {
		t.Fatalf("RankSongs: %v", err)
	}
	for _, want := range []string{"year=2020", "lyrics=no", "rating=explicit"} {
		if !strings.Contains(gotUser, want) {
			t.Errorf("ranking prompt missing %q\n%s", want, gotUser)
		}
	}
}

func TestExtractJSON(t *testing.T) {
	cases := []struct{ in, want string }{
		{`{"a":1}`, `{"a":1}`},
		{"```json\n{\"a\":1}\n```", `{"a":1}`},
		{"sure, here:\n{\"a\":1}\nhope that helps", `{"a":1}`},
		{"```\n{\"a\":[1,2]}\n```", `{"a":[1,2]}`},
	}
	for _, tc := range cases {
		if got := extractJSON(tc.in); got != tc.want {
			t.Errorf("extractJSON(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
