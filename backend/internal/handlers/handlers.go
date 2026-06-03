// Package handlers wires HTTP endpoints to the LLM and Apple Music layers.
package handlers

import (
	"context"

	"github.com/bkcarlos/hum/internal/applemusic"
	"github.com/bkcarlos/hum/internal/config"
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
}

func New(cfg *config.Config, tokens *applemusic.TokenManager, apple AppleService) *Handlers {
	return &Handlers{cfg: cfg, tokens: tokens, apple: apple}
}
