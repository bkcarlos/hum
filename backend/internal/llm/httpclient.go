package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
)

const maxRespBytes = 1 << 20 // 1 MiB cap on upstream response reads

// postJSON sends a JSON payload and returns the raw 2xx body, or a normalized
// *APIError. headers carries provider auth (the BYOK key) — these are set on the
// outbound request only and are never logged here.
func postJSON(ctx context.Context, client *http.Client, url string, headers map[string]string, payload any, p ProviderType) ([]byte, error) {
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, &APIError{Kind: ErrBadRequest, Provider: p, Message: "无法序列化请求体：" + err.Error()}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return nil, &APIError{Kind: ErrBadRequest, Provider: p, Message: "无效的请求地址：" + err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, networkError(p, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
	if resp.StatusCode/100 != 2 {
		return nil, normalizeHTTPError(p, resp.StatusCode, body)
	}
	return body, nil
}

// maxListBytes caps model-list responses, which can run large (e.g. OpenRouter
// lists hundreds of models) — bigger than maxRespBytes, which sizes chat replies.
const maxListBytes = 4 << 20 // 4 MiB

// getJSON issues a GET and returns the raw 2xx body, or a normalized *APIError.
// Like postJSON, headers carry the BYOK key and are set on the outbound request
// only — never logged here. Used by the best-effort ListModels endpoints.
func getJSON(ctx context.Context, client *http.Client, url string, headers map[string]string, p ProviderType) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, &APIError{Kind: ErrBadRequest, Provider: p, Message: "无效的请求地址：" + err.Error()}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, networkError(p, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxListBytes))
	if resp.StatusCode/100 != 2 {
		return nil, normalizeHTTPError(p, resp.StatusCode, body)
	}
	return body, nil
}
