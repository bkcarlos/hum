package quota

import (
	"context"
	"sync"
	"testing"
	"time"
)

func cfg() Config {
	return Config{Enabled: true, PerUserDailyLimit: 3, GlobalDailyLimit: 5}
}

func TestReserve_PerUserLimit(t *testing.T) {
	s := NewMemoryStore(cfg())
	ctx, day := context.Background(), "2026-06-06"

	for i := 1; i <= 3; i++ {
		d, _ := s.Reserve(ctx, "alice", day, cfg())
		if !d.Allowed {
			t.Fatalf("reserve %d: want allowed, got reason %q", i, d.Reason)
		}
		if d.UserUsed != i {
			t.Fatalf("reserve %d: userUsed=%d want %d", i, d.UserUsed, i)
		}
	}
	// 4th must be denied with user_exhausted, and must NOT increment.
	d, _ := s.Reserve(ctx, "alice", day, cfg())
	if d.Allowed || d.Reason != ReasonUser {
		t.Fatalf("4th reserve: want denied user_exhausted, got allowed=%v reason=%q", d.Allowed, d.Reason)
	}
	if got, _ := s.GetUserUsage(ctx, "alice", day); got != 3 {
		t.Fatalf("usage after denial = %d, want 3 (no overshoot)", got)
	}
}

func TestReserve_PerUserIsolated(t *testing.T) {
	s := NewMemoryStore(cfg())
	ctx, day := context.Background(), "2026-06-06"
	for i := 0; i < 3; i++ {
		s.Reserve(ctx, "alice", day, cfg())
	}
	// bob has his own bucket.
	if d, _ := s.Reserve(ctx, "bob", day, cfg()); !d.Allowed {
		t.Fatalf("bob should have his own quota, got reason %q", d.Reason)
	}
}

func TestReserve_GlobalCapBeforeUser(t *testing.T) {
	c := Config{Enabled: true, PerUserDailyLimit: 100, GlobalDailyLimit: 2}
	s := NewMemoryStore(c)
	ctx, day := context.Background(), "2026-06-06"
	s.Reserve(ctx, "a", day, c)
	s.Reserve(ctx, "b", day, c)
	// Global cap (2) reached; a fresh user is still blocked globally.
	d, _ := s.Reserve(ctx, "c", day, c)
	if d.Allowed || d.Reason != ReasonGlobal {
		t.Fatalf("want global_exhausted, got allowed=%v reason=%q", d.Allowed, d.Reason)
	}
}

func TestReserve_DisabledAndBanned(t *testing.T) {
	ctx, day := context.Background(), "2026-06-06"

	off := Config{Enabled: false, PerUserDailyLimit: 10}
	s := NewMemoryStore(off)
	if d, _ := s.Reserve(ctx, "alice", day, off); d.Allowed || d.Reason != ReasonDisabled {
		t.Fatalf("disabled: want denied disabled, got allowed=%v reason=%q", d.Allowed, d.Reason)
	}

	s2 := NewMemoryStore(cfg())
	s2.SetBanned(ctx, "mallory", true)
	if d, _ := s2.Reserve(ctx, "mallory", day, cfg()); d.Allowed || d.Reason != ReasonBanned {
		t.Fatalf("banned: want denied banned, got allowed=%v reason=%q", d.Allowed, d.Reason)
	}
	// Unbanning restores access.
	s2.SetBanned(ctx, "mallory", false)
	if d, _ := s2.Reserve(ctx, "mallory", day, cfg()); !d.Allowed {
		t.Fatalf("after unban: want allowed, got reason %q", d.Reason)
	}
}

func TestRefund(t *testing.T) {
	s := NewMemoryStore(cfg())
	ctx, day := context.Background(), "2026-06-06"
	s.Reserve(ctx, "alice", day, cfg())
	s.Reserve(ctx, "alice", day, cfg())
	if err := s.Refund(ctx, "alice", day); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetUserUsage(ctx, "alice", day); got != 1 {
		t.Fatalf("user usage after refund = %d, want 1", got)
	}
	if got, _ := s.GetGlobalUsage(ctx, day); got != 1 {
		t.Fatalf("global usage after refund = %d, want 1", got)
	}
}

func TestReserve_DayRollover(t *testing.T) {
	s := NewMemoryStore(cfg())
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		s.Reserve(ctx, "alice", "2026-06-06", cfg())
	}
	// New day → fresh quota.
	if d, _ := s.Reserve(ctx, "alice", "2026-06-07", cfg()); !d.Allowed {
		t.Fatalf("new day should reset quota, got reason %q", d.Reason)
	}
}

func TestReserve_ConcurrentNoOvershoot(t *testing.T) {
	c := Config{Enabled: true, PerUserDailyLimit: 50, GlobalDailyLimit: 50}
	s := NewMemoryStore(c)
	ctx, day := context.Background(), "2026-06-06"

	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if d, _ := s.Reserve(ctx, "alice", day, c); d.Allowed {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowed != 50 {
		t.Fatalf("concurrent reserves allowed=%d, want exactly 50 (cap)", allowed)
	}
}

func TestDay(t *testing.T) {
	got := Day(time.Date(2026, 6, 6, 23, 30, 0, 0, time.FixedZone("x", 3*3600)))
	if got != "2026-06-06" {
		t.Fatalf("Day = %q, want 2026-06-06 (UTC)", got)
	}
}
