package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// BodyLimit caps the request body at max bytes. It wraps c.Request.Body with
// http.MaxBytesReader so an oversized body fails the handler's JSON bind (→ 400)
// instead of being read into memory wholesale. This bounds the cost of any single
// request — notably /rank, whose candidate list is otherwise fed verbatim into the
// (server-paid, on the free tier) LLM prompt.
func BodyLimit(max int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if max > 0 {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max)
		}
		c.Next()
	}
}
