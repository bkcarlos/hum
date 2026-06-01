// Package httpx holds small HTTP response helpers shared by handlers.
package httpx

import "github.com/gin-gonic/gin"

// OK writes {"data": ...} with 200.
func OK(c *gin.Context, data any) {
	c.JSON(200, gin.H{"data": data})
}

// Fail writes {"error":{"code","message"}} with the given status. `code` is a
// stable machine-readable string (e.g. "auth", "rate_limit"); message is shown
// to the user and MUST NOT contain secrets.
func Fail(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}
