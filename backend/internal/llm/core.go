package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// chatFn is the one provider-specific primitive each adapter supplies: send a
// system + user prompt, get back the assistant's text. ParseIntent/RankSongs/
// Ping are written once on top of it, so adapters only implement transport.
type chatFn func(ctx context.Context, system, user string) (string, error)

// core implements Provider's high-level methods over a chatFn. Adapters embed
// it and wire core.chat to their own transport method.
type core struct {
	chat chatFn
}

const intentSystem = `You convert a user's natural-language music request into a strict JSON search intent.
Output JSON ONLY — no prose, no code fences. Schema:
{"moods":[],"genres":[],"instruments":[],"tempo":"","keywords":[],"seed_artists":[]}
Rules:
- "tempo" is one of "slow","medium","fast", or "" if unclear.
- Keep arrays concise (<=6 items each); omit guesses you are unsure about.
- Preserve the user's language for free-text values (e.g. Chinese stays Chinese).
- NEVER invent song or artist names; only reflect what the user expressed.`

func (c core) ParseIntent(ctx context.Context, text string, seedArtists []string) (*Intent, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "User request:\n%s\n", strings.TrimSpace(text))
	if len(seedArtists) > 0 {
		fmt.Fprintf(&b, "\nSeed artists/songs the user gave: %s\n", strings.Join(seedArtists, ", "))
	}
	raw, err := c.chat(ctx, intentSystem, b.String())
	if err != nil {
		return nil, err
	}
	var intent Intent
	if err := json.Unmarshal([]byte(extractJSON(raw)), &intent); err != nil {
		return nil, fmt.Errorf("llm: could not parse intent JSON: %w", err)
	}
	// Carry through explicit seeds even if the model omitted them.
	if len(intent.SeedArtists) == 0 && len(seedArtists) > 0 {
		intent.SeedArtists = seedArtists
	}
	return &intent, nil
}

const rankSystem = `You are a music curator. From a fixed pool of REAL candidate songs, select and order the best matches for the user's intent, then name the playlist.
Output JSON ONLY — no prose, no code fences. Schema:
{"playlist_name":"","description":"","songs":[{"id":"<candidate id>","reason":"<short why, in the user's language>"}]}
HARD RULES:
- Every "id" MUST be copied verbatim from the candidate pool. NEVER invent an id or a song.
- Pick 12-25 songs unless the pool is smaller. Order best-fit first.
- "reason" is one short sentence. "playlist_name" is evocative and concise.`

func (c core) RankSongs(ctx context.Context, intent *Intent, candidates []Candidate, instruction string) (*RankResult, error) {
	intentJSON, _ := json.Marshal(intent)

	var b strings.Builder
	fmt.Fprintf(&b, "Intent:\n%s\n\n", intentJSON)
	if strings.TrimSpace(instruction) != "" {
		// F10 multi-turn refinement, e.g. "去掉有歌词的" / "再爵士一点".
		fmt.Fprintf(&b, "Additional refinement instruction:\n%s\n\n", strings.TrimSpace(instruction))
	}
	b.WriteString("Candidate pool (choose ONLY from these ids):\n")
	for _, s := range candidates {
		fmt.Fprintf(&b, "- id=%s | %s — %s", s.ID, s.Title, s.Artist)
		if s.Album != "" {
			fmt.Fprintf(&b, " | album=%s", s.Album)
		}
		if len(s.Genres) > 0 {
			fmt.Fprintf(&b, " | genres=%s", strings.Join(s.Genres, "/"))
		}
		b.WriteByte('\n')
	}

	raw, err := c.chat(ctx, rankSystem, b.String())
	if err != nil {
		return nil, err
	}
	var res RankResult
	if err := json.Unmarshal([]byte(extractJSON(raw)), &res); err != nil {
		return nil, fmt.Errorf("llm: could not parse ranking JSON: %w", err)
	}

	// Golden-rule enforcement: silently drop any id the model fabricated or that
	// is no longer in the pool. The frontend should never see a non-existent id.
	valid := make(map[string]struct{}, len(candidates))
	for _, s := range candidates {
		valid[s.ID] = struct{}{}
	}
	kept := res.Songs[:0]
	for _, s := range res.Songs {
		if _, ok := valid[s.ID]; ok {
			kept = append(kept, s)
		}
	}
	res.Songs = kept
	if len(res.Songs) == 0 {
		return nil, fmt.Errorf("llm: ranking returned no valid candidate ids")
	}
	return &res, nil
}

func (c core) Ping(ctx context.Context) error {
	_, err := c.chat(ctx, "You are a connectivity probe.", `Reply with exactly: ok`)
	return err
}

// extractJSON best-effort pulls the first JSON object out of a model response,
// tolerating ```json fences or stray prose around it.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "```"); i >= 0 {
		// strip a fenced block: ```json\n...\n```
		rest := s[i+3:]
		if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
			rest = rest[nl+1:]
		}
		if j := strings.Index(rest, "```"); j >= 0 {
			rest = rest[:j]
		}
		s = strings.TrimSpace(rest)
	}
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
