// Package llm is the BYOK (bring-your-own-key) LLM layer.
//
// The golden rule (docs/requirements.md §2.1): the LLM is NEVER the source of
// truth for which songs exist. It (1) parses natural language into an Intent,
// (2) ranks a pool of REAL Apple Music candidates, and (3) under Option A may
// PROPOSE specific songs by name — but every proposal must be resolved against
// the real catalog (see applemusic.Client.ResolveSong) and dropped if it does
// not exist, so a fabricated/misattributed track never reaches the user.
// Three native adapters cover the first batch of providers behind one interface:
//
//	OpenAICompat — OpenAI, DeepSeek, Qwen/DashScope, Moonshot, GLM, OpenRouter, …
//	Anthropic    — Claude (/v1/messages, x-api-key + anthropic-version)
//	Gemini       — Google Gemini (generateContent)
//
// Security: cfg.APIKey is request-scoped. It is never logged, cached, or
// persisted; it lives only for the lifetime of a single Provider built per
// request and is discarded when the request ends.
package llm

import (
	"context"
	"fmt"
	"time"
)

type ProviderType string

const (
	ProviderOpenAICompat ProviderType = "openai-compat"
	ProviderAnthropic    ProviderType = "anthropic"
	ProviderGemini       ProviderType = "gemini"
)

// Config describes a single LLM call target. APIKey is the BYOK secret.
type Config struct {
	Provider ProviderType
	BaseURL  string // provider-specific; the factory fills sane defaults if empty
	Model    string
	APIKey   string // request-scoped secret — never log or persist this
	Timeout  time.Duration
}

// Intent is the structured search condition parsed from natural language (F3).
type Intent struct {
	Moods       []string `json:"moods"`
	Genres      []string `json:"genres"`
	Instruments []string `json:"instruments"`
	Tempo       string   `json:"tempo"` // e.g. "slow" | "medium" | "fast" | ""
	Keywords    []string `json:"keywords"`
	SeedArtists []string `json:"seed_artists"`
}

// Candidate is one real Apple Music track fed back to the LLM for ranking (F5).
// Year/HasLyrics/ContentRating come from the catalog and let the ranker honor
// refinements precisely (e.g. "去掉有歌词的" → lyrics, "不要露骨的" → rating).
type Candidate struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Artist        string   `json:"artist"`
	Album         string   `json:"album,omitempty"`
	Genres        []string `json:"genres,omitempty"`
	Year          string   `json:"year,omitempty"`          // release year, for "newer/older"
	HasLyrics     bool     `json:"hasLyrics"`               // false ⇒ instrumental
	ContentRating string   `json:"contentRating,omitempty"` // "explicit" ⇒ filterable
}

// RankedSong is one LLM-selected track plus its recommendation reason.
type RankedSong struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// RankResult is the ranking output: a named playlist and ordered picks (F5).
type RankResult struct {
	PlaylistName string       `json:"playlist_name"`
	Description  string       `json:"description"`
	Songs        []RankedSong `json:"songs"`
}

// SongSuggestion is one LLM-proposed track (title + artist) to be resolved
// against the real Apple catalog (Option A): the LLM proposes, Apple verifies.
type SongSuggestion struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
}

// SuggestResult is the LLM's first-pass output (Option A): a structured intent
// for display plus concrete real songs it recommends. Every suggestion MUST be
// resolved against the catalog before reaching the user (golden rule preserved).
type SuggestResult struct {
	Intent      Intent           `json:"intent"`
	Suggestions []SongSuggestion `json:"songs"`
}

// ExampleHints carries lightweight, non-identifying personalization signals for
// generating empty-state example prompts ("千人千面"): the current context
// (time/scene/locale) and the user's recent local tastes. No account or PII.
type ExampleHints struct {
	Context string   `json:"context"`
	Tastes  []string `json:"tastes"`
}

// ModelInfo is one selectable model returned by ListModels: an id (sent as the
// model name) plus an optional human label. Best-effort discovery — the config
// UI merges these with the static presets and still allows typing a name.
type ModelInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName,omitempty"`
}

// Provider is the single internal interface every adapter implements.
type Provider interface {
	// ParseIntent turns free text (+ optional seed artists) into an Intent.
	ParseIntent(ctx context.Context, text string, seedArtists []string) (*Intent, error)
	// RankSongs selects & orders from candidates. Implementations MUST drop any
	// returned id that is not present in candidates (golden-rule enforcement).
	RankSongs(ctx context.Context, intent *Intent, candidates []Candidate, instruction string) (*RankResult, error)
	// SuggestSongs proposes specific real songs for the request (Option A), plus a
	// structured intent for display. Callers MUST resolve each suggestion against
	// the catalog and drop anything that does not exist there (golden rule).
	SuggestSongs(ctx context.Context, text string, seedArtists []string) (*SuggestResult, error)
	// SuggestExamples generates short natural-language example prompts personalized
	// to the given context + recent tastes (empty-state inspiration; names no songs).
	SuggestExamples(ctx context.Context, hints ExampleHints, count int) ([]string, error)
	// ListModels fetches the provider's available model ids for the configured
	// key/BaseURL (F0 convenience). Best-effort: not every OpenAI-compatible
	// gateway supports it, so callers MUST fall back to manual entry on error.
	ListModels(ctx context.Context) ([]ModelInfo, error)
	// Ping issues a minimal request to validate the key / connectivity (F0 test).
	Ping(ctx context.Context) error
}

// New builds the adapter for cfg.Provider, applying default Base URLs/models.
func New(cfg Config) (Provider, error) {
	cfg = withDefaults(cfg)
	switch cfg.Provider {
	case ProviderOpenAICompat:
		return newOpenAICompat(cfg), nil
	case ProviderAnthropic:
		return newAnthropic(cfg), nil
	case ProviderGemini:
		return newGemini(cfg), nil
	case "":
		return nil, fmt.Errorf("llm: provider is required")
	default:
		return nil, fmt.Errorf("llm: unknown provider %q", cfg.Provider)
	}
}

func withDefaults(cfg Config) Config {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.BaseURL == "" {
		switch cfg.Provider {
		case ProviderOpenAICompat:
			cfg.BaseURL = "https://api.openai.com/v1"
		case ProviderAnthropic:
			cfg.BaseURL = "https://api.anthropic.com"
		case ProviderGemini:
			cfg.BaseURL = "https://generativelanguage.googleapis.com"
		}
	}
	return cfg
}
