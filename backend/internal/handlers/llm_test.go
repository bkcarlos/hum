package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/bkcarlos/hum/internal/llm"
)

// 原始上游细节（含 Base URL / 响应体）绝不外露给任何用户——免费档（服务端自有
// Base URL，红线）与 BYOK（用户自己的私有网关地址）都只看归一化文案，Detail 仅进日志。
func TestWriteLLMError_NeverLeaksDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	apiErr := &llm.APIError{
		Kind:     llm.ErrNetwork,
		Provider: llm.ProviderOpenAICompat,
		Message:  "无法连接到该服务商，请检查网络、Base URL 或区域可达性，或稍后重试。",
		Detail:   `Post "https://api2.tabcode.cc/claude/office2/v1/messages": context deadline exceeded`,
	}

	assertClean := func(t *testing.T, body string) {
		t.Helper()
		if strings.Contains(body, "tabcode.cc") {
			t.Fatalf("leaked Base URL / detail to user: %s", body)
		}
		if !strings.Contains(body, "无法连接到该服务商") {
			t.Fatalf("missing clean message: %s", body)
		}
	}

	// 免费档（无 X-LLM-Api-Key）。
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/suggest", nil)
	writeLLMError(c, apiErr)
	assertClean(t, w.Body.String())

	// BYOK（带 X-LLM-Api-Key）——同样不回显 Base URL / 细节。
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest("POST", "/api/suggest", nil)
	c2.Request.Header.Set(llmAPIKeyHeader, "sk-user-key")
	writeLLMError(c2, apiErr)
	assertClean(t, w2.Body.String())
}
