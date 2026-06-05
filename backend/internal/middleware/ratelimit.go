package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bkcarlos/hum/internal/httpx"
)

// visitor is a token bucket for a single client IP.
type visitor struct {
	tokens   float64
	lastSeen time.Time
}

// RateLimiter is a per-IP token-bucket limiter: `rps` tokens refill per second,
// up to `burst` capacity. rps<=0 disables limiting entirely. Safe for concurrent use.
type RateLimiter struct {
	rps      float64
	burst    float64
	mu       sync.Mutex
	visitors map[string]*visitor
}

// NewRateLimiter builds a limiter. rps<=0 makes Middleware a pass-through.
func NewRateLimiter(rps, burst int) *RateLimiter {
	return &RateLimiter{
		rps:      float64(rps),
		burst:    float64(burst),
		visitors: make(map[string]*visitor),
	}
}

// allow consumes one token for ip at time now, refilling by elapsed time.
func (rl *RateLimiter) allow(ip string, now time.Time) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Lazy GC: keep the map bounded under many distinct IPs (no background goroutine).
	if len(rl.visitors) > 10_000 {
		for k, vv := range rl.visitors {
			if now.Sub(vv.lastSeen) > 10*time.Minute {
				delete(rl.visitors, k)
			}
		}
	}

	v := rl.visitors[ip]
	if v == nil {
		v = &visitor{tokens: rl.burst, lastSeen: now}
		rl.visitors[ip] = v
	}
	v.tokens += now.Sub(v.lastSeen).Seconds() * rl.rps
	if v.tokens > rl.burst {
		v.tokens = rl.burst
	}
	v.lastSeen = now
	if v.tokens >= 1 {
		v.tokens--
		return true
	}
	return false
}

// Middleware enforces the per-IP limit. On limit it writes a 429 rate_limit
// error and aborts; rps<=0 is a pass-through.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	if rl.rps <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		if !rl.allow(c.ClientIP(), time.Now()) {
			httpx.Fail(c, http.StatusTooManyRequests, "rate_limit", "请求过于频繁，请稍后再试。")
			c.Abort()
			return
		}
		c.Next()
	}
}
