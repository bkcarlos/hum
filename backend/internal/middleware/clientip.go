package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// trustedProxyHops is the number of trusted proxies that APPEND to
// X-Forwarded-For in front of the app (set once at startup). 0 disables XFF
// parsing — the direct peer is the client (local dev / no proxy).
var trustedProxyHops int

// SetTrustedProxyHops configures XFF-based client-IP resolution. On Cloud Run
// pass 1: the platform appends the real client IP as the LAST XFF entry, so the
// resolved IP is the real client and is spoofing-resistant (a client-supplied
// X-Forwarded-For value sits to the LEFT of the platform-appended one).
func SetTrustedProxyHops(n int) { trustedProxyHops = n }

// clientIP returns the IP used for per-IP rate limiting + request logs. With
// trustedProxyHops>0 it takes the hops-th entry from the END of X-Forwarded-For
// (the platform-appended client IP); otherwise — and on any malformed header — it
// falls back to gin's direct-peer ClientIP (SetTrustedProxies(nil) in main).
func clientIP(c *gin.Context) string {
	if trustedProxyHops > 0 {
		if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if idx := len(parts) - trustedProxyHops; idx >= 0 && idx < len(parts) {
				if ip := strings.TrimSpace(parts[idx]); ip != "" {
					return ip
				}
			}
		}
	}
	return c.ClientIP()
}
