// Command server is the Go orchestration backend (Gin). It mints the Apple
// Developer Token, proxies BYOK LLM calls (key used once, never stored/logged),
// and talks to Apple Music for catalog search + playlist creation.
package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/bkcarlos/apple-music-llm/internal/applemusic"
	"github.com/bkcarlos/apple-music-llm/internal/config"
	"github.com/bkcarlos/apple-music-llm/internal/handlers"
	"github.com/bkcarlos/apple-music-llm/internal/httpx"
	"github.com/bkcarlos/apple-music-llm/internal/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("loading config", "err", err)
		os.Exit(1)
	}

	// Default to release mode; GIN_MODE=debug opts back into verbose dev logging.
	if mode := os.Getenv("GIN_MODE"); mode != "" {
		gin.SetMode(mode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Apple is optional at boot: without credentials the server still serves the
	// frontend + LLM endpoints, and /api/apple/* returns a clear 503. `apple` is
	// kept as the interface type so it stays a true nil when unconfigured.
	var (
		tokens *applemusic.TokenManager
		apple  handlers.AppleService
	)
	if cfg.AppleConfigured() {
		tokens, err = applemusic.NewTokenManager(cfg.AppleTeamID, cfg.AppleKeyID, cfg.ApplePrivateKeyPath, cfg.AppleTokenTTL)
		if err != nil {
			slog.Warn("Apple Music disabled: could not load developer key", "err", err)
		} else {
			apple = applemusic.NewClient(tokens, cfg.UpstreamHTTPTimeout, cfg.AppleAPIBase, cfg.SearchCacheTTL)
			slog.Info("Apple Music enabled")
		}
	} else {
		slog.Warn("Apple credentials not set — /api/apple/* returns 503 until configured (see backend/.env.example)")
	}

	h := handlers.New(cfg, tokens, apple)

	r := gin.New()
	_ = r.SetTrustedProxies(nil) // don't trust any proxy headers by default
	r.Use(gin.Recovery())
	r.Use(middleware.Logger())
	r.Use(cors.New(cors.Config{
		AllowOrigins: cfg.CORSAllowedOrigins,
		AllowMethods: []string{"GET", "POST", "OPTIONS"},
		// Custom headers carrying the BYOK key and Music User Token must be allowed.
		AllowHeaders:     []string{"Origin", "Content-Type", "X-LLM-Api-Key", "Music-User-Token"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) { httpx.OK(c, gin.H{"status": "ok"}) })

		// Apple Music
		api.GET("/apple/developer-token", h.DeveloperToken)
		api.POST("/apple/search", h.Search)
		api.POST("/apple/playlists", h.CreatePlaylist)

		// BYOK LLM
		api.POST("/llm/test", h.TestLLM)
		api.POST("/intent", h.ParseIntent)
		api.POST("/rank", h.Rank)
	}

	addr := ":" + cfg.Port
	slog.Info("server starting", "addr", addr, "config", cfg.String())
	if err := r.Run(addr); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
