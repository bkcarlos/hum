package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestClientIP_TrustedProxyHops(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defer SetTrustedProxyHops(0) // reset the package global after the test

	ctx := func(xff, remoteAddr string) *gin.Context {
		req := httptest.NewRequest("GET", "/", nil)
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}
		req.RemoteAddr = remoteAddr
		c, eng := gin.CreateTestContext(httptest.NewRecorder())
		_ = eng.SetTrustedProxies(nil) // match production: don't trust proxy headers
		c.Request = req
		return c
	}

	// hops=0: XFF ignored, the direct peer is the client (local dev).
	SetTrustedProxyHops(0)
	if got := clientIP(ctx("1.1.1.1", "203.0.113.9:5555")); got != "203.0.113.9" {
		t.Errorf("hops=0: clientIP = %q, want 203.0.113.9 (direct peer)", got)
	}

	// hops=1 (Cloud Run): the real client is the LAST XFF entry (platform-appended);
	// a client-spoofed value to its left must be ignored.
	SetTrustedProxyHops(1)
	if got := clientIP(ctx("9.9.9.9, 203.0.113.7", "10.0.0.1:5555")); got != "203.0.113.7" {
		t.Errorf("hops=1 spoofed-prefix: clientIP = %q, want 203.0.113.7", got)
	}
	if got := clientIP(ctx("203.0.113.5", "10.0.0.1:5555")); got != "203.0.113.5" {
		t.Errorf("hops=1 single-entry: clientIP = %q, want 203.0.113.5", got)
	}
	// hops=1 but no XFF → fall back to the direct peer.
	if got := clientIP(ctx("", "203.0.113.1:5555")); got != "203.0.113.1" {
		t.Errorf("hops=1 no-xff: clientIP = %q, want 203.0.113.1 (fallback)", got)
	}
}
