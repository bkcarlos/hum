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

// APIError is a normalized LLM provider error.
//   - Message is ALWAYS safe to show ANY user: it never contains the API key, nor
//     the (possibly server-side) Base URL, nor raw upstream internals.
//   - Detail holds the raw upstream detail (the request URL / response body snippet)
//     for SERVER LOGS and BYOK self-service ONLY. It MUST NOT reach free-tier users,
//     where the Base URL is the operator's own infrastructure. The handler
//     (writeLLMError) logs Detail always but only echoes it to BYOK requests.
type APIError struct {
	Kind     ErrorKind
	Status   int
	Provider ProviderType
	Message  string
	Detail   string
}

func (e *APIError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s error (%s): %s [%s]", e.Provider, e.Kind, e.Message, e.Detail)
	}
	return fmt.Sprintf("%s error (%s): %s", e.Provider, e.Kind, e.Message)
}

// networkError wraps a transport-level failure (DNS, TLS, timeout, region block).
// The raw error contains the request URL, so it goes into Detail (logs/BYOK), never
// the user-facing Message.
func networkError(p ProviderType, err error) *APIError {
	return &APIError{
		Kind:     ErrNetwork,
		Provider: p,
		Message:  "无法连接到该服务商，请检查网络、Base URL 或区域可达性，或稍后重试。",
		Detail:   err.Error(),
	}
}

// normalizeHTTPError maps a non-2xx response to a friendly, classified error.
// body is the (truncated) response payload — used to refine the message and, for
// the opaque cases, kept in Detail (logs/BYOK) rather than the user Message.
func normalizeHTTPError(p ProviderType, status int, body []byte) *APIError {
	snippet := strings.TrimSpace(string(body))
	if len(snippet) > 500 {
		snippet = snippet[:500]
	}
	low := strings.ToLower(snippet)

	switch {
	case status == 401 || status == 403:
		return &APIError{Kind: ErrAuth, Status: status, Provider: p, Message: "API Key 无效或无权限，请检查 Key 是否正确、是否对应该服务商。"}
	case status == 404 && (strings.Contains(low, "model") || strings.Contains(low, "not found")):
		return &APIError{Kind: ErrModelNotFound, Status: status, Provider: p, Message: "模型不存在或当前 Key 无权访问，请检查模型名。"}
	case status == 429 && (strings.Contains(low, "quota") || strings.Contains(low, "insufficient") || strings.Contains(low, "balance") || strings.Contains(low, "billing")):
		return &APIError{Kind: ErrQuota, Status: status, Provider: p, Message: "额度不足或计费异常，请检查账户余额。"}
	case status == 429:
		return &APIError{Kind: ErrRateLimit, Status: status, Provider: p, Message: "请求过于频繁（限流），请稍后重试。"}
	case status == 400 || status == 422:
		return &APIError{Kind: ErrBadRequest, Status: status, Provider: p, Message: "请求被服务商拒绝（参数无效），请检查模型名或参数。", Detail: snippet}
	case status >= 500:
		return &APIError{Kind: ErrUpstream, Status: status, Provider: p, Message: fmt.Sprintf("服务商暂时不可用（%d），请稍后重试。", status), Detail: snippet}
	default:
		return &APIError{Kind: ErrUpstream, Status: status, Provider: p, Message: fmt.Sprintf("调用失败（HTTP %d），请稍后重试。", status), Detail: snippet}
	}
}
