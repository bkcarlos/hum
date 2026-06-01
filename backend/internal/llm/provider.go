// Package llm is the BYOK (bring-your-own-key) LLM layer.
//
// The golden rule (docs/requirements.md §2.1): the LLM never invents songs.
// It only (1) parses natural language into a structured Intent, and (2) ranks
// a pool of REAL Apple Music candidates. Three native adapters cover the first
// batch of providers behind one internal interface:
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
type Candidate struct {
	ID     string   `json:"id"`
	Title  string   `json:"title"`
	Artist string   `json:"artist"`
	Album  string   `json:"album,omitempty"`
	Genres []string `json:"genres,omitempty"`
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

// Provider is the single internal interface every adapter implements.
type Provider interface {
	// ParseIntent turns free text (+ optional seed artists) into an Intent.
	ParseIntent(ctx context.Context, text string, seedArtists []string) (*Intent, error)
	// RankSongs selects & orders from candidates. Implementations MUST drop any
	// returned id that is not present in candidates (golden-rule enforcement).
	RankSongs(ctx context.Context, intent *Intent, candidates []Candidate, instruction string) (*RankResult, error)
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
