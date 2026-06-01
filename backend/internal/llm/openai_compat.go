package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// openAICompat implements Provider for any OpenAI-style /chat/completions API:
// OpenAI, DeepSeek, Qwen/DashScope (compat endpoint), Moonshot, GLM, MiniMax,
// OpenRouter, and "custom OpenAI-compatible". BaseURL is expected to include the
// version segment (e.g. https://api.openai.com/v1).
//
// response_format is intentionally NOT set — many compatible vendors reject it.
// The system prompt asks for raw JSON and extractJSON tolerates fences/prose.
type openAICompat struct {
	core
	cfg  Config
	http *http.Client
}

func newOpenAICompat(cfg Config) Provider {
	p := &openAICompat{cfg: cfg, http: &http.Client{Timeout: cfg.Timeout}}
	p.core = core{chat: p.chat}
	return p
}

func (p *openAICompat) chat(ctx context.Context, system, user string) (string, error) {
	url := strings.TrimRight(p.cfg.BaseURL, "/") + "/chat/completions"
	payload := map[string]any{
		"model":       p.cfg.Model,
		"temperature": 0.3,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	}
	headers := map[string]string{"Authorization": "Bearer " + p.cfg.APIKey}

	body, err := postJSON(ctx, p.http, url, headers, payload, ProviderOpenAICompat)
	if err != nil {
		return "", err
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", &APIError{Kind: ErrUpstream, Provider: ProviderOpenAICompat, Message: "无法解析服务商响应：" + err.Error()}
	}
	if len(out.Choices) == 0 {
		return "", &APIError{Kind: ErrUpstream, Provider: ProviderOpenAICompat, Message: "服务商返回空结果。"}
	}
	return out.Choices[0].Message.Content, nil
}
