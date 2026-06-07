// Package handlers wires HTTP endpoints to the LLM and Apple Music layers.
package handlers

import (
	"context"

	"github.com/bkcarlos/hum/internal/applemusic"
	"github.com/bkcarlos/hum/internal/auth"
	"github.com/bkcarlos/hum/internal/config"
	"github.com/bkcarlos/hum/internal/quota"
)

// llmAPIKeyHeader is the ONLY place the BYOK user key travels. It is read per
// request, passed straight to a provider, and never persisted or logged.
const llmAPIKeyHeader = "X-LLM-Api-Key"

// musicUserTokenHeader carries the per-request Apple Music User Token (F1).
const musicUserTokenHeader = "Music-User-Token"

// AppleService is the catalog/library surface the handlers depend on. The real
// *applemusic.Client satisfies it; tests inject a fake. Keeping it an interface
// lets us unit-test Search dedup/cap without a live Apple account.
type AppleService interface {
	SearchSongs(ctx context.Context, storefront, term string, limit int) ([]applemusic.Song, error)
	ResolveSong(ctx context.Context, storefront, title, artist string) (*applemusic.Song, bool, error)
	CreatePlaylist(ctx context.Context, userToken, name, description string, catalogSongIDs []string) (*applemusic.Playlist, error)
}

// Handlers holds shared dependencies. tokens/apple are nil when Apple
// credentials are not configured — those endpoints then return 503 with a
// clear message instead of crashing the server.
type Handlers struct {
	cfg    *config.Config
	tokens *applemusic.TokenManager
	apple  AppleService

	// Free tier (optional). When set via WithFreeTier, suggest/rank accept a Sign
	// in with Apple session + the server's default key under quota. nil → BYOK-only
	// (suggest/rank require X-LLM-Api-Key, exactly as before).
	quota         quota.Store
	appleAuth     *auth.AppleVerifier
	sessionSecret []byte
}

func New(cfg *config.Config, tokens *applemusic.TokenManager, apple AppleService) *Handlers {
	return &Handlers{cfg: cfg, tokens: tokens, apple: apple}
}

// WithFreeTier wires the Sign in with Apple free tier (server default key under
// quota). Returns the same *Handlers for chaining. Leave unset for BYOK-only.
func (h *Handlers) WithFreeTier(store quota.Store, verifier *auth.AppleVerifier, sessionSecret []byte) *Handlers {
	h.quota = store
	h.appleAuth = verifier
	h.sessionSecret = sessionSecret
	return h
}

// authReady reports whether Sign in with Apple identity + the admin backstage are
// wired (quota store + Apple verifier + session secret). These do NOT need the
// server LLM key — login and admin are decoupled from the free-tier LLM config.
// The free-tier recommendation path additionally needs an LLM key (admin-set
// config key or the env DEFAULT_LLM_API_KEY), checked inline in resolveProvider.
func (h *Handlers) authReady() bool {
	return h.quota != nil && h.appleAuth != nil && len(h.sessionSecret) > 0 && h.cfg != nil
}
