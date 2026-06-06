// Command server is the Go orchestration backend (Gin). It mints the Apple
// Developer Token, proxies BYOK LLM calls (key used once, never stored/logged),
// and talks to Apple Music for catalog search + playlist creation.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/bkcarlos/hum/internal/applemusic"
	"github.com/bkcarlos/hum/internal/auth"
	"github.com/bkcarlos/hum/internal/config"
	"github.com/bkcarlos/hum/internal/handlers"
	"github.com/bkcarlos/hum/internal/httpx"
	"github.com/bkcarlos/hum/internal/middleware"
	"github.com/bkcarlos/hum/internal/quota"
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
		pem, perr := cfg.ApplePrivateKeyPEM()
		if perr != nil {
			slog.Warn("Apple Music disabled: cannot read private key", "err", perr)
		} else if tokens, err = applemusic.NewTokenManagerFromPEM(cfg.AppleTeamID, cfg.AppleKeyID, pem, cfg.AppleTokenTTL); err != nil {
			slog.Warn("Apple Music disabled: invalid developer key", "err", err)
		} else {
			apple = applemusic.NewClient(tokens, cfg.UpstreamHTTPTimeout, cfg.AppleAPIBase, cfg.SearchCacheTTL)
			slog.Info("Apple Music enabled")
		}
	} else {
		slog.Warn("Apple credentials not set — /api/apple/* returns 503 until configured (see backend/.env.example)")
	}

	h := handlers.New(cfg, tokens, apple)

	// Free tier (Sign in with Apple + server default key under quota) is optional.
	// Off unless DEFAULT_LLM_API_KEY / SESSION_SECRET / APPLE_BUNDLE_ID / model are
	// set — then suggest/rank also accept a Bearer session metered by quota.
	freeTierOn := false
	if cfg.FreeTierConfigured() {
		seed := quota.Config{
			Enabled:           cfg.FreeTierEnabled,
			PerUserDailyLimit: cfg.FreeTierPerUser,
			GlobalDailyLimit:  cfg.FreeTierGlobal,
			LLMProvider:       cfg.DefaultLLMProvider,
			LLMBaseURL:        cfg.DefaultLLMBaseURL,
			LLMModel:          cfg.DefaultLLMModel,
			Admins:            cfg.AdminAppleSubs,
		}
		var store quota.Store
		if cfg.FirestoreProject != "" {
			initCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			fs, ferr := quota.NewFirestoreStore(initCtx, cfg.FirestoreProject, seed)
			cancel()
			if ferr != nil {
				slog.Error("free tier: Firestore init failed, falling back to in-memory", "err", ferr)
				store = quota.NewMemoryStore(seed)
			} else {
				store = fs
				slog.Info("free tier: using Firestore quota store", "project", cfg.FirestoreProject)
			}
		} else {
			store = quota.NewMemoryStore(seed)
			slog.Warn("free tier: using in-memory quota (single-instance; set FIRESTORE_PROJECT for prod)")
		}
		h = h.WithFreeTier(store, auth.NewAppleVerifier(cfg.AppleBundleID, cfg.UpstreamHTTPTimeout), []byte(cfg.SessionSecret))
		freeTierOn = true
		slog.Info("free tier enabled", "perUserDaily", cfg.FreeTierPerUser, "globalDaily", cfg.FreeTierGlobal, "seededAdmins", len(cfg.AdminAppleSubs))
	} else {
		slog.Info("free tier disabled — suggest/rank are BYOK-only (set DEFAULT_LLM_API_KEY/SESSION_SECRET/APPLE_BUNDLE_ID/DEFAULT_LLM_MODEL to enable)")
	}

	r := gin.New()
	_ = r.SetTrustedProxies(nil) // don't trust any proxy headers by default
	r.Use(gin.Recovery())
	r.Use(middleware.Logger())
	r.Use(cors.New(cors.Config{
		AllowOrigins: cfg.CORSAllowedOrigins,
		AllowMethods: []string{"GET", "POST", "OPTIONS"},
		// Custom headers: BYOK key, Music User Token, and the free-tier session Bearer.
		AllowHeaders:     []string{"Origin", "Content-Type", "X-LLM-Api-Key", "Music-User-Token", "Authorization"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	api := r.Group("/api")
	api.Use(middleware.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst).Middleware())
	{
		api.GET("/health", func(c *gin.Context) { httpx.OK(c, gin.H{"status": "ok"}) })

		// Apple Music
		api.GET("/apple/developer-token", h.DeveloperToken)
		api.POST("/apple/search", h.Search)
		api.POST("/apple/playlists", h.CreatePlaylist)

		// BYOK LLM
		api.POST("/llm/test", h.TestLLM)
		api.POST("/llm/models", h.Models) // best-effort live model list for the config UI
		api.POST("/intent", h.ParseIntent)
		api.POST("/rank", h.Rank)
		api.POST("/suggest", h.Suggest)   // Option A: LLM proposes songs → resolved against Apple
		api.POST("/examples", h.Examples) // personalized empty-state example prompts

		// Free tier: exchange a Sign in with Apple identity token for a session.
		if freeTierOn {
			api.POST("/auth/apple", h.AppleAuth)
			api.GET("/auth/me", h.Me) // who am I (+ isAdmin) — bootstrap + UI nav

			// Admin API (C3+) sits behind the Apple-sub allowlist in the live
			// config. AdminOnly verifies the Bearer session's sub ∈ Admins.
			admin := api.Group("/admin")
			admin.Use(h.AdminOnly())
			admin.GET("/me", h.AdminMe) // gate canary the admin UI hits on load
			admin.GET("/config", h.AdminGetConfig)
			admin.POST("/config", h.AdminUpdateConfig) // POST (not PUT): GET/POST-only API
			admin.GET("/usage", h.AdminUsage)
			admin.POST("/users/:sub/ban", h.AdminBan)
			admin.POST("/users/:sub/unban", h.AdminUnban)
		}
	}

	// 方案 1 (single-service): also serve the built frontend from WebDir with SPA
	// fallback. Same-origin, so no CORS is involved in production. Empty in dev.
	if cfg.WebDir != "" {
		registerFrontend(r, cfg.WebDir)
		slog.Info("serving frontend", "dir", cfg.WebDir)
	}

	addr := ":" + cfg.Port
	slog.Info("server starting", "addr", addr, "config", cfg.String())
	if err := r.Run(addr); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

// registerFrontend serves the built SPA from dir for any non-/api route,
// falling back to index.html for client-side routes. Cleaning the request path
// against root before joining prevents directory traversal outside dir.
func registerFrontend(r *gin.Engine, dir string) {
	index := filepath.Join(dir, "index.html")
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api") {
			httpx.Fail(c, http.StatusNotFound, "not_found", "未知接口。")
			return
		}
		full := filepath.Join(dir, filepath.FromSlash(path.Clean("/"+p)))
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			c.File(full)
			return
		}
		c.File(index) // SPA fallback
	})
}
