// Package quota enforces the free-tier usage policy for requests that use the
// server's default LLM key (i.e. no BYOK key). Limits live in a Store so they can
// be changed at runtime without a restart (Firestore in prod, in-memory in dev/
// tests). Counting is atomic per (user, day) and globally per day. BYOK requests
// never pass through here — they are unlimited and pay with the user's own key.
package quota

import (
	"context"
	"errors"
	"time"
)

// Config is the runtime-adjustable free-tier policy. It is stored (Firestore) and
// read live, so changing any value takes effect without a restart.
type Config struct {
	Enabled           bool `json:"enabled" firestore:"enabled"`
	PerUserDailyLimit int  `json:"perUserDailyLimit" firestore:"perUserDailyLimit"` // 0 = no per-user cap
	GlobalDailyLimit  int  `json:"globalDailyLimit" firestore:"globalDailyLimit"`   // 0 = no global cap
	// Default LLM used for free-tier (no BYOK) requests. The API key itself is NOT
	// here — it is a server secret injected from the environment / Secret Manager.
	LLMProvider string `json:"llmProvider" firestore:"llmProvider"`
	LLMBaseURL  string `json:"llmBaseUrl" firestore:"llmBaseUrl"`
	LLMModel    string `json:"llmModel" firestore:"llmModel"`
	// Admins is the allowlist of Apple `sub`s permitted to use the admin API/UI.
	// It lives in this same live config doc so it can be edited in the console
	// without a restart. Bootstrap the first admin by reading your sub from
	// GET /api/auth/me and adding it here. Empty = no admins (admin routes 403).
	Admins []string `json:"admins" firestore:"admins"`
}

// IsAdmin reports whether sub is in the admin allowlist (exact match).
func (c Config) IsAdmin(sub string) bool {
	if sub == "" {
		return false
	}
	for _, a := range c.Admins {
		if a == sub {
			return true
		}
	}
	return false
}

// Reason explains why a reserve was denied (empty when allowed).
type Reason string

const (
	ReasonOK       Reason = ""
	ReasonDisabled Reason = "disabled"         // free tier turned off
	ReasonBanned   Reason = "banned"           // this user is banned
	ReasonGlobal   Reason = "global_exhausted" // global daily cap hit (protects your bill)
	ReasonUser     Reason = "user_exhausted"   // this user's daily cap hit
)

// Decision is the outcome of a reserve attempt, with counters for the UI/429 body.
type Decision struct {
	Allowed     bool   `json:"allowed"`
	Reason      Reason `json:"reason,omitempty"`
	UserUsed    int    `json:"userUsed"`
	UserLimit   int    `json:"userLimit"`
	GlobalUsed  int    `json:"globalUsed"`
	GlobalLimit int    `json:"globalLimit"`
}

// UserUsage is one user's metered usage on a given day plus their ban state and
// (when known) the Apple email, for the admin usage/users view.
type UserUsage struct {
	Sub    string `json:"sub"`
	Email  string `json:"email,omitempty"`
	Used   int    `json:"used"`
	Banned bool   `json:"banned"`
}

// ErrNotFound is returned by stores when a document/config doesn't exist yet.
var ErrNotFound = errors.New("quota: not found")

// Store persists policy config, per-user/global daily usage, and bans.
//
// Reserve MUST be atomic: check config+ban+limits and, if allowed, record exactly
// one use (user and global) in a single transaction, so concurrent requests can't
// overshoot the cap.
type Store interface {
	GetConfig(ctx context.Context) (Config, error)
	SetConfig(ctx context.Context, c Config) error

	// Reserve atomically checks the (passed, possibly-cached) cfg + ban + limits
	// for sub on day and, if allowed, records one use. The default LLM key is used
	// downstream only when Decision.Allowed is true.
	Reserve(ctx context.Context, sub, day string, cfg Config) (Decision, error)
	// Refund returns one reserved use — call when the downstream LLM request fails,
	// so a failed request doesn't burn the user's quota.
	Refund(ctx context.Context, sub, day string) error

	GetUserUsage(ctx context.Context, sub, day string) (int, error)
	GetGlobalUsage(ctx context.Context, day string) (int, error)

	// AdminUsage returns the global metered count for day plus per-user usage with
	// ban state — the union of users who used quota that day and all banned users
	// (unordered; the caller sorts). For the admin dashboard.
	AdminUsage(ctx context.Context, day string) (global int, users []UserUsage, err error)

	IsBanned(ctx context.Context, sub string) (bool, error)
	SetBanned(ctx context.Context, sub string, banned bool) error

	// SetUserEmail records the Apple email for a sub (captured at login, when the
	// email scope is granted), so the admin view can show emails instead of opaque
	// subs. GetUserEmail returns "" when unknown.
	SetUserEmail(ctx context.Context, sub, email string) error
	GetUserEmail(ctx context.Context, sub string) (string, error)
}

// Day is the canonical day bucket key (UTC), shared by callers and stores so a
// "daily" window is consistent regardless of server timezone.
func Day(t time.Time) string { return t.UTC().Format("2006-01-02") }
