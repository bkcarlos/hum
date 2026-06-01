// Package config loads runtime configuration from environment variables
// (optionally seeded from a local .env file for development).
//
// Deliberately absent: any user LLM API key. Those are BYOK and never live
// in server config, env, or storage — they arrive per request and are
// discarded immediately (see docs/requirements.md F0).
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
	ApplePrivateKeyPath string
	AppleTokenTTL       time.Duration

	// Apple Music API base (override for testing / regional proxies).
	AppleAPIBase string

	// Server.
	Port                string
	CORSAllowedOrigins  []string
	UpstreamHTTPTimeout time.Duration

	// Catalog search result cache lifetime (0 disables caching).
	SearchCacheTTL time.Duration
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
		AppleTokenTTL:       time.Duration(envInt("APPLE_TOKEN_TTL_HOURS", 4320)) * time.Hour,
		AppleAPIBase:        envStr("APPLE_API_BASE", "https://api.music.apple.com"),
		Port:                envStr("PORT", "8080"),
		CORSAllowedOrigins:  splitCSV(envStr("CORS_ALLOWED_ORIGINS", "http://localhost:5173")),
		UpstreamHTTPTimeout: time.Duration(envInt("UPSTREAM_TIMEOUT_SECONDS", 30)) * time.Second,
		SearchCacheTTL:      time.Duration(envInt("SEARCH_CACHE_TTL_SECONDS", 600)) * time.Second,
	}
	return cfg, nil
}

// AppleConfigured reports whether the Apple Developer Token can be minted.
func (c *Config) AppleConfigured() bool {
	return c.AppleTeamID != "" && c.AppleKeyID != "" && c.ApplePrivateKeyPath != ""
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
	return fmt.Sprintf("Config{port=%s, appleConfigured=%t, origins=%v, ttl=%s}",
		c.Port, c.AppleConfigured(), c.CORSAllowedOrigins, c.AppleTokenTTL)
}
