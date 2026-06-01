package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// anthropic implements Provider for Claude's native Messages API:
// POST {base}/v1/messages, auth via x-api-key + anthropic-version.
type anthropic struct {
	core
	cfg  Config
	http *http.Client
}

const anthropicVersion = "2023-06-01"

func newAnthropic(cfg Config) Provider {
	p := &anthropic{cfg: cfg, http: &http.Client{Timeout: cfg.Timeout}}
	p.core = core{chat: p.chat}
	return p
}

func (p *anthropic) chat(ctx context.Context, system, user string) (string, error) {
	url := strings.TrimRight(p.cfg.BaseURL, "/") + "/v1/messages"
	payload := map[string]any{
		"model":       p.cfg.Model,
		"max_tokens":  2048,
		"temperature": 0.3,
		"system":      system,
		"messages": []map[string]any{
			{"role": "user", "content": user},
		},
	}
	headers := map[string]string{
		"x-api-key":         p.cfg.APIKey,
		"anthropic-version": anthropicVersion,
	}

	body, err := postJSON(ctx, p.http, url, headers, payload, ProviderAnthropic)
	if err != nil {
		return "", err
	}
	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", &APIError{Kind: ErrUpstream, Provider: ProviderAnthropic, Message: "无法解析 Claude 响应：" + err.Error()}
	}
	var sb strings.Builder
	for _, c := range out.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	if sb.Len() == 0 {
		return "", &APIError{Kind: ErrUpstream, Provider: ProviderAnthropic, Message: "Claude 返回空结果。"}
	}
	return sb.String(), nil
}
