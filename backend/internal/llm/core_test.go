package llm

import (
	"context"
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
