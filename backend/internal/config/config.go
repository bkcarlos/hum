// Package config loads runtime configuration from environment variables
// (optionally seeded from a local .env file for development).
//
// Deliberately absent: any *user* LLM API key. Those are BYOK and never live
// in server config, env, or storage — they arrive per request and are
// discarded immediately (see docs/requirements.md F0).
//
// The single deliberate exception is DEFAULT_LLM_API_KEY: the server's OWN key
// that powers the Sign in with Apple free tier (quota-gated). It is injected
// from Secret Manager, never a user key, and never logged.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	// Apple Music developer credentials (used to mint the Developer Token).
	AppleTeamID         string
	AppleKeyID          string
	ApplePrivateKeyPath string // path to the .p8 file (local/dev)
	ApplePrivateKey     string // OR the .p8 PEM contents directly (cloud env var)
	AppleTokenTTL       time.Duration

	// Apple Music API base (override for testing / regional proxies).
	AppleAPIBase string

	// Server.
	Port                string
	CORSAllowedOrigins  []string
	UpstreamHTTPTimeout time.Duration

	// WebDir, when set, makes the server also serve the built frontend from this
	// directory (single-service / 方案 1). Empty in dev (Vite serves the SPA).
	WebDir string

	// Catalog search result cache lifetime (0 disables caching).
	SearchCacheTTL time.Duration

	// Per-IP rate limit (token bucket): RateLimitRPS tokens/sec refill, capacity
	// RateLimitBurst. RateLimitRPS<=0 disables limiting entirely.
	RateLimitRPS   int
	RateLimitBurst int

	// ── Free tier (Sign in with Apple + server default key, gated by quota) ──
	// All of DefaultLLMKey/SessionSecret/AppleBundleID/DefaultLLMModel must be set
	// for the free tier to turn on (see FreeTierConfigured); otherwise suggest/rank
	// stay BYOK-only. DefaultLLMKey is the SERVER's own key (Secret Manager), never
	// a user key, never logged.
	DefaultLLMKey      string
	DefaultLLMProvider string
	DefaultLLMBaseURL  string
	DefaultLLMModel    string
	SessionSecret      string // HMAC secret for our session JWTs
	AppleBundleID      string // expected `aud` of Apple identity tokens (iOS bundle id)

	// Bootstrap free-tier policy — seeds the quota store / Firestore config doc.
	// After boot the live values come from the store (changeable without restart).
	FreeTierEnabled bool
	FreeTierPerUser int // per-user metered calls/day (1 recommendation ≈ 2: suggest+rank)
	FreeTierGlobal  int // global metered calls/day (0 = no global cap)

	// FirestoreProject enables the durable Firestore quota store; empty → in-memory
	// (single-instance, dev only).
	FirestoreProject string
}

// Load reads .env (if present) into the process environment, then builds a
// Config. Missing Apple credentials are tolerated here (the server still
// boots so the frontend and BYOK/LLM endpoints work); the Apple token
// handler reports a clear error if they are absent when actually needed.
func Load() (*Config, error) {
	loadDotEnv(".env")

	cfg := &Config{
		AppleTeamID:         os.Getenv("APPLE_TEAM_ID"),
		AppleKeyID:          os.Getenv("APPLE_KEY_ID"),
		ApplePrivateKeyPath: os.Getenv("APPLE_PRIVATE_KEY_PATH"),
		ApplePrivateKey:     os.Getenv("APPLE_PRIVATE_KEY"),
		AppleTokenTTL:       time.Duration(envInt("APPLE_TOKEN_TTL_HOURS", 4320)) * time.Hour,
		AppleAPIBase:        envStr("APPLE_API_BASE", "https://api.music.apple.com"),
		Port:                envStr("PORT", "8080"),
		CORSAllowedOrigins:  splitCSV(envStr("CORS_ALLOWED_ORIGINS", "http://localhost:5173")),
		UpstreamHTTPTimeout: time.Duration(envInt("UPSTREAM_TIMEOUT_SECONDS", 30)) * time.Second,
		WebDir:              os.Getenv("WEB_DIR"),
		SearchCacheTTL:      time.Duration(envInt("SEARCH_CACHE_TTL_SECONDS", 600)) * time.Second,
		RateLimitRPS:        envInt("RATE_LIMIT_RPS", 10),
		RateLimitBurst:      envInt("RATE_LIMIT_BURST", 30),

		DefaultLLMKey:      os.Getenv("DEFAULT_LLM_API_KEY"),
		DefaultLLMProvider: envStr("DEFAULT_LLM_PROVIDER", "openai-compat"),
		DefaultLLMBaseURL:  os.Getenv("DEFAULT_LLM_BASE_URL"),
		DefaultLLMModel:    os.Getenv("DEFAULT_LLM_MODEL"),
		SessionSecret:      os.Getenv("SESSION_SECRET"),
		AppleBundleID:      os.Getenv("APPLE_BUNDLE_ID"),
		FreeTierEnabled:    envBool("FREE_TIER_ENABLED", true),
		FreeTierPerUser:    envInt("FREE_TIER_PER_USER_DAILY", 20),
		FreeTierGlobal:     envInt("FREE_TIER_GLOBAL_DAILY", 0),
		FirestoreProject:   os.Getenv("FIRESTORE_PROJECT"),
	}
	return cfg, nil
}

// FreeTierConfigured reports whether the Sign in with Apple free tier can run:
// it needs the server's own LLM key + model, a session secret, and the expected
// Apple audience (bundle id). Missing any → suggest/rank stay BYOK-only.
func (c *Config) FreeTierConfigured() bool {
	return c.DefaultLLMKey != "" && c.DefaultLLMModel != "" &&
		c.SessionSecret != "" && c.AppleBundleID != ""
}

// AppleConfigured reports whether the Apple Developer Token can be minted.
func (c *Config) AppleConfigured() bool {
	return c.AppleTeamID != "" && c.AppleKeyID != "" && (c.ApplePrivateKey != "" || c.ApplePrivateKeyPath != "")
}

// ApplePrivateKeyPEM returns the .p8 PEM bytes from APPLE_PRIVATE_KEY (preferred
// on cloud hosts) or, failing that, from the file at APPLE_PRIVATE_KEY_PATH.
// A literal "\n"-escaped env value (common when pasting a key into a dashboard)
// is unescaped; a genuine multi-line PEM is unaffected.
func (c *Config) ApplePrivateKeyPEM() ([]byte, error) {
	if c.ApplePrivateKey != "" {
		return []byte(strings.ReplaceAll(c.ApplePrivateKey, `\n`, "\n")), nil
	}
	if c.ApplePrivateKeyPath != "" {
		return os.ReadFile(c.ApplePrivateKeyPath)
	}
	return nil, fmt.Errorf("config: no Apple private key (set APPLE_PRIVATE_KEY or APPLE_PRIVATE_KEY_PATH)")
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// loadDotEnv is a tiny dependency-free .env reader: KEY=VALUE lines, # comments,
// existing process env always wins. Good enough for local dev; prod uses real env.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // no .env is fine
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}

// String renders a redacted summary safe for logging (no secrets present anyway).
func (c *Config) String() string {
	return fmt.Sprintf("Config{port=%s, appleConfigured=%t, serveFrontend=%t, origins=%v, ttl=%s}",
		c.Port, c.AppleConfigured(), c.WebDir != "", c.CORSAllowedOrigins, c.AppleTokenTTL)
}
