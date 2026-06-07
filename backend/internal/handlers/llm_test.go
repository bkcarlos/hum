package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bkcarlos/hum/internal/llm"
)

// The server-side LLM Base URL must never leak to free-tier users (no BYOK key),
// but is fine to echo back to BYOK users (it's their own config). Detail always
// goes to logs regardless.
func TestWriteLLMError_ScrubsServerDetailForFreeTier(t *testing.T) {
	gin.SetMode(gin.TestMode)
	apiErr := &llm.APIError{
		Kind:     llm.ErrNetwork,
		Provider: llm.ProviderOpenAICompat,
		Message:  "无法连接到该服务商，请检查网络、Base URL 或区域可达性，或稍后重试。",
		Detail:   `Post "https://api2.tabcode.cc/claude/office2/v1/messages": context deadline exceeded`,
	}

	// 免费档（无 X-LLM-Api-Key）→ 绝不泄露服务端 Base URL。
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/suggest", nil)
	writeLLMError(c, apiErr)
	if strings.Contains(w.Body.String(), "tabcode.cc") {
		t.Fatalf("free-tier leaked server Base URL: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "无法连接到该服务商") {
		t.Fatalf("free-tier missing clean message: %s", w.Body.String())
	}

	// BYOK（带 X-LLM-Api-Key）→ 回显细节（用户自己的 Base URL）。
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest("POST", "/api/suggest", nil)
	c2.Request.Header.Set(llmAPIKeyHeader, "sk-user-key")
	writeLLMError(c2, apiErr)
	if !strings.Contains(w2.Body.String(), "tabcode.cc") {
		t.Fatalf("BYOK should echo detail: %s", w2.Body.String())
	}
}
