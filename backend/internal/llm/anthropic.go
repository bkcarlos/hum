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
	p := &anthropic{cfg: cfg, http: safeHTTPClient(cfg.Timeout)}
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

// ListModels calls GET {base}/v1/models (Anthropic's Models API). limit=1000
// pulls the whole (small) catalog in one page, avoiding pagination.
func (p *anthropic) ListModels(ctx context.Context) ([]ModelInfo, error) {
	url := strings.TrimRight(p.cfg.BaseURL, "/") + "/v1/models?limit=1000"
	headers := map[string]string{
		"x-api-key":         p.cfg.APIKey,
		"anthropic-version": anthropicVersion,
	}
	body, err := getJSON(ctx, p.http, url, headers, ProviderAnthropic)
	if err != nil {
		return nil, err
	}
	var out struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, &APIError{Kind: ErrUpstream, Provider: ProviderAnthropic, Message: "无法解析模型列表：" + err.Error()}
	}
	models := make([]ModelInfo, 0, len(out.Data))
	for _, m := range out.Data {
		if m.ID != "" {
			models = append(models, ModelInfo{ID: m.ID, DisplayName: m.DisplayName})
		}
	}
	return models, nil
}
