// Package middleware contains Gin middleware.
//
// SECURITY POLICY (docs/requirements.md F0): the user's LLM API key arrives in
// the X-LLM-Api-Key header and MUST NEVER be written to any log. This logger
// records only method, path, status, latency, and client IP — never headers or
// bodies. The SensitiveHeaders list below is the canonical "never log" set; any
// future logging that touches headers must run them through RedactHeaders.
package middleware

import (
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// SensitiveHeaders must never appear in logs, traces, or APM snapshots.
var SensitiveHeaders = []string{
	"X-Llm-Api-Key",    // BYOK user LLM key (canonical header, MIME-cased)
	"Authorization",    // Apple Developer Token (Bearer) / OpenAI-style keys
	"X-Api-Key",        // Anthropic-style keys
	"X-Goog-Api-Key",   // Gemini keys
	"Music-User-Token", // Apple Music User Token
}

// Logger logs request lines without ever touching headers or bodies. An optional
// client-supplied X-Request-Id is logged (sanitized) so a client's diagnostic log
// can be matched to server lines; the id is opaque, non-sensitive, and never
// trusted verbatim.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path // query intentionally omitted (may carry tokens)
		rid := sanitizeRequestID(c.GetHeader("X-Request-Id"))
		c.Next()
		args := []any{
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"ip", c.ClientIP(),
		}
		if rid != "" {
			args = append(args, "request_id", rid)
		}
		slog.Info("request", args...)
	}
}

// sanitizeRequestID makes a client-supplied correlation id safe to log: it keeps
// only printable, non-space ASCII (so a crafted header can't inject newlines or
// control chars into a log line) and caps the length. The id is opaque and
// non-sensitive — used only to match client logs to server lines.
func sanitizeRequestID(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r > 0x20 && r < 0x7f { // printable ASCII, excludes space + control
			b.WriteRune(r)
		}
		if b.Len() >= 64 {
			break
		}
	}
	return b.String()
}

// RedactHeaders returns a copy of headers with every SensitiveHeaders value
// masked. Use this anywhere headers might otherwise be serialized into output.
func RedactHeaders(h map[string][]string) map[string][]string {
	masked := map[string]struct{}{}
	for _, name := range SensitiveHeaders {
		masked[strings.ToLower(name)] = struct{}{}
	}
	out := make(map[string][]string, len(h))
	for k, v := range h {
		if _, hit := masked[strings.ToLower(k)]; hit {
			out[k] = []string{"[REDACTED]"}
			continue
		}
		out[k] = v
	}
	return out
}
