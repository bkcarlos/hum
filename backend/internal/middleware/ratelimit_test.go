package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimiter_BurstThenRefill(t *testing.T) {
	rl := NewRateLimiter(10, 3) // burst 3, refill 10/s
	now := time.Now()
	for i := 0; i < 3; i++ {
		if !rl.allow("1.2.3.4", now) {
			t.Fatalf("burst token %d should pass", i)
		}
	}
	if rl.allow("1.2.3.4", now) {
		t.Fatal("4th immediate request should be limited")
	}
	// 0.2s later → ~2 tokens refilled (10/s * 0.2)
	if !rl.allow("1.2.3.4", now.Add(200*time.Millisecond)) {
		t.Fatal("should pass after refill")
	}
}

func TestRateLimiter_PerIPIsolated(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	now := time.Now()
	if !rl.allow("a", now) {
		t.Fatal("ip a first request should pass")
	}
	if rl.allow("a", now) {
		t.Fatal("ip a second request should be limited")
	}
	if !rl.allow("b", now) {
		t.Fatal("ip b has its own bucket and should pass")
	}
}

// Middleware: limit hit returns 429 + rate_limit; disabled (rps<=0) passes through.
func TestRateLimiter_Middleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rl := NewRateLimiter(1, 1)
	mw := rl.Middleware()
	hit429 := false
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/x", nil)
		mw(c)
		if w.Code == http.StatusTooManyRequests {
			hit429 = true
		}
	}
	if !hit429 {
		t.Fatal("second request within the same instant should hit 429")
	}

	// rps<=0 → pass-through, never aborts.
	pass := NewRateLimiter(0, 0).Middleware()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/x", nil)
	pass(c)
	if c.IsAborted() {
		t.Fatal("disabled limiter must not abort")
	}
}
