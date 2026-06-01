package llm

import (
	"fmt"
	"strings"
)

// ErrorKind is a provider-agnostic classification of upstream failures, so the
// frontend can show one clear message regardless of which vendor was used (F0).
type ErrorKind string

const (
	ErrAuth          ErrorKind = "auth"            // key missing/invalid/forbidden
	ErrQuota         ErrorKind = "quota"           // out of credit / billing
	ErrRateLimit     ErrorKind = "rate_limit"      // throttled
	ErrModelNotFound ErrorKind = "model_not_found" // unknown/unavailable model
	ErrBadRequest    ErrorKind = "bad_request"     // malformed request
	ErrUpstream      ErrorKind = "upstream"        // provider 5xx / unknown
	ErrNetwork       ErrorKind = "network"         // could not reach provider
)

// APIError is a normalized LLM provider error. Message is safe to show users
// and NEVER contains the API key.
type APIError struct {
	Kind     ErrorKind
	Status   int
	Provider ProviderType
	Message  string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s error (%s): %s", e.Provider, e.Kind, e.Message)
}

// networkError wraps a transport-level failure (DNS, TLS, timeout, region block).
func networkError(p ProviderType, err error) *APIError {
	return &APIError{Kind: ErrNetwork, Provider: p, Message: "无法连接到该服务商，请检查网络、Base URL 或区域可达性（" + err.Error() + "）"}
}

// normalizeHTTPError maps a non-2xx response to a friendly, classified error.
// body is the (truncated) response payload — used only to refine the message;
// it is the provider's body, never our request (which is where the key lives).
func normalizeHTTPError(p ProviderType, status int, body []byte) *APIError {
	snippet := strings.TrimSpace(string(body))
	if len(snippet) > 500 {
		snippet = snippet[:500]
	}
	low := strings.ToLower(snippet)

	switch {
	case status == 401 || status == 403:
		return &APIError{ErrAuth, status, p, "API Key 无效或无权限，请检查 Key 是否正确、是否对应该服务商。"}
	case status == 404 && (strings.Contains(low, "model") || strings.Contains(low, "not found")):
		return &APIError{ErrModelNotFound, status, p, "模型不存在或当前 Key 无权访问，请检查模型名。"}
	case status == 429 && (strings.Contains(low, "quota") || strings.Contains(low, "insufficient") || strings.Contains(low, "balance") || strings.Contains(low, "billing")):
		return &APIError{ErrQuota, status, p, "额度不足或计费异常，请检查账户余额。"}
	case status == 429:
		return &APIError{ErrRateLimit, status, p, "请求过于频繁（限流），请稍后重试。"}
	case status == 400 || status == 422:
		return &APIError{ErrBadRequest, status, p, "请求被服务商拒绝（参数无效）：" + snippet}
	case status >= 500:
		return &APIError{ErrUpstream, status, p, "服务商暂时不可用（" + fmt.Sprint(status) + "），请稍后重试。"}
	default:
		return &APIError{ErrUpstream, status, p, fmt.Sprintf("调用失败（HTTP %d）：%s", status, snippet)}
	}
}
