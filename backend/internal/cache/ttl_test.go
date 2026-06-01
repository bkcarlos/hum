package cache

import (
	"testing"
	"time"
)

func TestTTL_GetSet(t *testing.T) {
	c := NewTTL[string](time.Minute)
	if _, ok := c.Get("a"); ok {
		t.Fatal("expected miss on empty cache")
	}
	c.Set("a", "1")
	if v, ok := c.Get("a"); !ok || v != "1" {
		t.Fatalf("Get = (%q, %v), want (1, true)", v, ok)
	}
}

func TestTTL_Expiry(t *testing.T) {
	c := NewTTL[int](time.Minute)
	now := time.Unix(1_000, 0)
	c.now = func() time.Time { return now } // control the clock

	c.Set("k", 42)
	if _, ok := c.Get("k"); !ok {
		t.Fatal("expected hit before expiry")
	}
	now = now.Add(2 * time.Minute) // advance past TTL
	if _, ok := c.Get("k"); ok {
		t.Fatal("expected miss after expiry")
	}
}

func TestTTL_Disabled(t *testing.T) {
	c := NewTTL[int](0) // ttl<=0 disables caching
	c.Set("k", 1)
	if _, ok := c.Get("k"); ok {
		t.Fatal("ttl<=0 should disable caching")
	}

	var nilCache *TTL[int]
	nilCache.Set("k", 1) // must not panic
	if _, ok := nilCache.Get("k"); ok {
		t.Fatal("nil cache should always miss")
	}
}
