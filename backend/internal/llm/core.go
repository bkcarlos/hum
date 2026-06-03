package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// chatFn is the one provider-specific primitive each adapter supplies: send a
// system + user prompt, get back the assistant's text. ParseIntent/RankSongs/
// SuggestSongs/Ping are written once on top of it, so adapters only implement
// transport.
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

const suggestSystem = `You are a music expert. From the user's request, recommend SPECIFIC, REAL songs that actually exist, and extract a short structured intent for display.
Output JSON ONLY — no prose, no code fences. Schema:
{"intent":{"moods":[],"genres":[],"instruments":[],"tempo":"","keywords":[],"seed_artists":[]},"songs":[{"title":"","artist":""}]}
Rules:
- "songs": 20-30 real songs that genuinely fit the request. Use the exact song title and its primary performing artist.
- Do NOT invent songs or misattribute artists; if unsure a track exists, omit it. Diversify artists; no duplicates.
- Prefer widely-available tracks (likely on Apple Music) and honor any seed artists/songs the user gives.
- "intent.tempo" is one of "slow","medium","fast", or "". Keep intent arrays concise (<=6 each).
- Preserve the user's language in intent free-text; keep song titles and artist names in their original/native form.`

// SuggestSongs is the Option A first pass: ask the LLM to name real songs (plus a
// structured intent for display). The names are only PROPOSALS — the caller must
// resolve each against the catalog so fabricated/misattributed tracks are dropped.
func (c core) SuggestSongs(ctx context.Context, text string, seedArtists []string) (*SuggestResult, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "User request:\n%s\n", strings.TrimSpace(text))
	if len(seedArtists) > 0 {
		fmt.Fprintf(&b, "\nSeed artists/songs to lean toward: %s\n", strings.Join(seedArtists, ", "))
	}
	raw, err := c.chat(ctx, suggestSystem, b.String())
	if err != nil {
		return nil, err
	}
	var res SuggestResult
	if err := json.Unmarshal([]byte(extractJSON(raw)), &res); err != nil {
		return nil, fmt.Errorf("llm: could not parse suggestions JSON: %w", err)
	}
	// Carry through explicit seeds even if the model omitted them from the intent.
	if len(res.Intent.SeedArtists) == 0 && len(seedArtists) > 0 {
		res.Intent.SeedArtists = seedArtists
	}
	return &res, nil
}

const exampleSystem = `You write SHORT, natural-language music-request examples a user could type into a "describe what you want to hear" box.
Output JSON ONLY — no prose, no code fences. Schema: {"examples":["",""]}
Rules:
- Each example is ONE concise phrase describing a mood / scene / style, like "适合雨天加班的慵懒爵士，别太吵". Aim for 8-22 characters. NEVER name a song or artist.
- Make them feel tailored to the given context and recent tastes; vary the mood/genre/scene across them; no duplicates.
- Write in the user's language (default Chinese unless the context clearly indicates otherwise).`

// SuggestExamples generates personalized empty-state example prompts ("千人千面")
// from a context string + recent taste tokens. Pure inspiration text — no songs.
func (c core) SuggestExamples(ctx context.Context, hints ExampleHints, count int) ([]string, error) {
	if count <= 0 {
		count = 4
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Generate %d example prompts.\n", count)
	if s := strings.TrimSpace(hints.Context); s != "" {
		fmt.Fprintf(&b, "Current context: %s\n", s)
	}
	if len(hints.Tastes) > 0 {
		fmt.Fprintf(&b, "The user has recently leaned toward: %s. Bias toward these but keep variety.\n", strings.Join(hints.Tastes, ", "))
	}
	raw, err := c.chat(ctx, exampleSystem, b.String())
	if err != nil {
		return nil, err
	}
	var res struct {
		Examples []string `json:"examples"`
	}
	if err := json.Unmarshal([]byte(extractJSON(raw)), &res); err != nil {
		return nil, fmt.Errorf("llm: could not parse examples JSON: %w", err)
	}
	out := make([]string, 0, count)
	for _, e := range res.Examples {
		if e = strings.TrimSpace(e); e != "" {
			out = append(out, e)
		}
		if len(out) >= count {
			break
		}
	}
	return out, nil
}

const rankSystem = `You are a music curator. From a fixed pool of REAL candidate songs, select and order the best matches for the user's intent, then name the playlist.
Output JSON ONLY — no prose, no code fences. Schema:
{"playlist_name":"","description":"","songs":[{"id":"<candidate id>","reason":"<short why, in the user's language>"}]}
HARD RULES:
- Every "id" MUST be copied verbatim from the candidate pool. NEVER invent an id or a song.
- Pick 12-25 songs unless the pool is smaller. Order best-fit first.
- "reason" is one short sentence. "playlist_name" is evocative and concise.
- Each candidate may carry year/lyrics/rating. Honor refinement instructions precisely with them: instrumental or "去掉有歌词的" → keep only lyrics=no; "不要露骨的" / no explicit → drop rating=explicit; newer/older → use year.`

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
		if s.Year != "" {
			fmt.Fprintf(&b, " | year=%s", s.Year)
		}
		if s.HasLyrics {
			b.WriteString(" | lyrics=yes")
		} else {
			b.WriteString(" | lyrics=no")
		}
		if s.ContentRating != "" {
			fmt.Fprintf(&b, " | rating=%s", s.ContentRating)
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
