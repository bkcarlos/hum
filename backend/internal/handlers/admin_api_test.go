package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bkcarlos/hum/internal/auth"
	"github.com/bkcarlos/hum/internal/quota"
)

// adminSession builds free-tier handlers with "admin-1" as admin and returns a
// ready admin Bearer header.
func adminSession(t *testing.T) (*Handlers, map[string]string) {
	t.Helper()
	h, secret := adminHandlers("admin-1")
	sess, err := auth.IssueSession(secret, "admin-1", time.Hour, time.Now())
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}
	return h, bearer(sess)
}

func decodeData(w *httptest.ResponseRecorder, v any) {
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	_ = json.Unmarshal(env.Data, v)
}

func TestAdminConfig_GetAndPartialUpdatePreservesAdmins(t *testing.T) {
	h, hdr := adminSession(t)
	r := adminRouter(h)

	// GET returns the seeded config, including the admin allowlist.
	w := do(r, http.MethodGet, "/api/admin/config", hdr, nil)
	var got quota.Config
	decodeData(w, &got)
	if w.Code != http.StatusOK || got.PerUserDailyLimit != 5 || len(got.Admins) != 1 || got.Admins[0] != "admin-1" {
		t.Fatalf("GET config: code=%d cfg=%+v", w.Code, got)
	}

	// Partial update: change a limit + toggle, OMIT admins → admins must survive.
	w = do(r, http.MethodPost, "/api/admin/config", hdr, map[string]any{
		"perUserDailyLimit": 99,
		"enabled":           false,
	})
	decodeData(w, &got)
	if w.Code != http.StatusOK || got.PerUserDailyLimit != 99 || got.Enabled != false {
		t.Fatalf("POST config: code=%d cfg=%+v", w.Code, got)
	}
	if len(got.Admins) != 1 || got.Admins[0] != "admin-1" {
		t.Fatalf("admins wiped by partial update: %+v", got.Admins)
	}

	// And it persisted to the store.
	stored, _ := h.quota.GetConfig(context.Background())
	if stored.PerUserDailyLimit != 99 || len(stored.Admins) != 1 {
		t.Fatalf("not persisted: %+v", stored)
	}
}

func TestAdminConfig_RejectsBadProviderAndClampsNegatives(t *testing.T) {
	h, hdr := adminSession(t)
	r := adminRouter(h)

	if w := do(r, http.MethodPost, "/api/admin/config", hdr, map[string]any{"llmProvider": "bogus"}); w.Code != http.StatusBadRequest {
		t.Fatalf("bad provider: want 400, got %d", w.Code)
	}
	// Negative limit clamps to 0 (= no cap), not stored as negative.
	w := do(r, http.MethodPost, "/api/admin/config", hdr, map[string]any{"globalDailyLimit": -5})
	var got quota.Config
	decodeData(w, &got)
	if w.Code != http.StatusOK || got.GlobalDailyLimit != 0 {
		t.Fatalf("clamp: code=%d global=%d", w.Code, got.GlobalDailyLimit)
	}
}

func TestAdminConfig_UpdateAdminsCanRevokeSelf(t *testing.T) {
	// Editing admins through the API works (and can drop yourself — a real,
	// recoverable action, e.g. handing off; the console can always re-add).
	h, hdr := adminSession(t)
	r := adminRouter(h)
	w := do(r, http.MethodPost, "/api/admin/config", hdr, map[string]any{"admins": []string{"someone-else", "someone-else", "  "}})
	var got quota.Config
	decodeData(w, &got)
	if w.Code != http.StatusOK || len(got.Admins) != 1 || got.Admins[0] != "someone-else" {
		t.Fatalf("admins update/clean: code=%d admins=%+v", w.Code, got.Admins)
	}
	// The former admin is now locked out.
	if w := do(r, http.MethodGet, "/api/admin/config", hdr, nil); w.Code != http.StatusForbidden {
		t.Fatalf("after self-revoke: want 403, got %d", w.Code)
	}
}

func TestAdminUsage_AggregatesAndSorts(t *testing.T) {
	h, hdr := adminSession(t)
	r := adminRouter(h)
	ctx := context.Background()
	day := quota.Day(time.Now())
	cfg, _ := h.quota.GetConfig(ctx)

	// u1 uses twice, u2 once; u3 is banned with no usage.
	_, _ = h.quota.Reserve(ctx, "u1", day, cfg)
	_, _ = h.quota.Reserve(ctx, "u1", day, cfg)
	_, _ = h.quota.Reserve(ctx, "u2", day, cfg)
	_ = h.quota.SetBanned(ctx, "u3", true)

	w := do(r, http.MethodGet, "/api/admin/usage", hdr, nil)
	var got struct {
		Day        string            `json:"day"`
		GlobalUsed int               `json:"globalUsed"`
		Users      []quota.UserUsage `json:"users"`
	}
	decodeData(w, &got)
	if w.Code != http.StatusOK || got.Day != day || got.GlobalUsed != 3 {
		t.Fatalf("usage: code=%d day=%s global=%d", w.Code, got.Day, got.GlobalUsed)
	}
	if len(got.Users) != 3 || got.Users[0].Sub != "u1" || got.Users[0].Used != 2 {
		t.Fatalf("usage users (want u1=2 first): %+v", got.Users)
	}
	// The banned, zero-usage user is present and flagged.
	var foundBan bool
	for _, u := range got.Users {
		if u.Sub == "u3" {
			foundBan = u.Banned && u.Used == 0
		}
	}
	if !foundBan {
		t.Fatalf("banned u3 missing/!flagged: %+v", got.Users)
	}

	// A malformed day is rejected.
	if w := do(r, http.MethodGet, "/api/admin/usage?day=2026/06/07", hdr, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("bad day: want 400, got %d", w.Code)
	}
}

func TestEmail_InMeAndUsage(t *testing.T) {
	h, hdr := adminSession(t) // admin "admin-1"
	ctx := context.Background()
	_ = h.quota.SetUserEmail(ctx, "admin-1", "boss@example.com")
	_ = h.quota.SetUserEmail(ctx, "u1", "u1@example.com")
	cfg, _ := h.quota.GetConfig(ctx)
	_, _ = h.quota.Reserve(ctx, "u1", quota.Day(time.Now()), cfg)
	r := adminRouter(h)

	// /admin/me carries the admin's own email.
	w := do(r, http.MethodGet, "/api/admin/me", hdr, nil)
	var me struct {
		Email string `json:"email"`
	}
	decodeData(w, &me)
	if me.Email != "boss@example.com" {
		t.Fatalf("admin/me email = %q, want boss@example.com", me.Email)
	}

	// /admin/usage carries each user's email.
	w = do(r, http.MethodGet, "/api/admin/usage", hdr, nil)
	var us struct {
		Users []quota.UserUsage `json:"users"`
	}
	decodeData(w, &us)
	found := false
	for _, u := range us.Users {
		if u.Sub == "u1" && u.Email == "u1@example.com" {
			found = true
		}
	}
	if !found {
		t.Fatalf("usage missing u1 email: %+v", us.Users)
	}
}

func TestAdminBanUnban_TakesEffectAtReserve(t *testing.T) {
	h, hdr := adminSession(t)
	r := adminRouter(h)
	ctx := context.Background()
	day := quota.Day(time.Now())
	cfg, _ := h.quota.GetConfig(ctx)

	// Ban u9 via the API → its next reserve is denied with ReasonBanned.
	if w := do(r, http.MethodPost, "/api/admin/users/u9/ban", hdr, nil); w.Code != http.StatusOK {
		t.Fatalf("ban: code=%d body=%s", w.Code, w.Body.String())
	}
	if dec, _ := h.quota.Reserve(ctx, "u9", day, cfg); dec.Allowed || dec.Reason != quota.ReasonBanned {
		t.Fatalf("after ban: allowed=%v reason=%q", dec.Allowed, dec.Reason)
	}
	// Unban → allowed again.
	if w := do(r, http.MethodPost, "/api/admin/users/u9/unban", hdr, nil); w.Code != http.StatusOK {
		t.Fatalf("unban: code=%d", w.Code)
	}
	if dec, _ := h.quota.Reserve(ctx, "u9", day, cfg); !dec.Allowed {
		t.Fatalf("after unban: want allowed, got reason=%q", dec.Reason)
	}
}
