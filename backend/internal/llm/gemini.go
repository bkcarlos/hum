package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
)

// gemini implements Provider for Google Gemini's generateContent API.
//
// Security note: Gemini also accepts the key as a ?key= query parameter, but we
// deliberately pass it via the x-goog-api-key HEADER so the secret never lands
// in a URL (and thus never in proxy/CDN access logs) — see F0 leakage hardening.
type gemini struct {
	core
	cfg  Config
	http *http.Client
}

func newGemini(cfg Config) Provider {
	p := &gemini{cfg: cfg, http: safeHTTPClient(cfg.Timeout)}
	p.core = core{chat: p.chat}
	return p
}

func (p *gemini) chat(ctx context.Context, system, user string) (string, error) {
	model := strings.TrimPrefix(p.cfg.Model, "models/")
	url := strings.TrimRight(p.cfg.BaseURL, "/") + "/v1beta/models/" + model + ":generateContent"
	payload := map[string]any{
		"system_instruction": map[string]any{
			"parts": []map[string]string{{"text": system}},
		},
		"contents": []map[string]any{
			{"role": "user", "parts": []map[string]string{{"text": user}}},
		},
		"generationConfig": map[string]any{"temperature": 0.3},
	}
	headers := map[string]string{"x-goog-api-key": p.cfg.APIKey}

	body, err := postJSON(ctx, p.http, url, headers, payload, ProviderGemini)
	if err != nil {
		return "", err
	}
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", &APIError{Kind: ErrUpstream, Provider: ProviderGemini, Message: "无法解析 Gemini 响应：" + err.Error()}
	}
	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return "", &APIError{Kind: ErrUpstream, Provider: ProviderGemini, Message: "Gemini 返回空结果（可能触发了安全过滤）。"}
	}
	var sb strings.Builder
	for _, part := range out.Candidates[0].Content.Parts {
		sb.WriteString(part.Text)
	}
	return sb.String(), nil
}

// ListModels calls GET {base}/v1beta/models and keeps only models that support
// generateContent (dropping embedding/aqa/etc.), stripping the "models/" prefix
// so the id matches what chat() expects. The key travels in the x-goog-api-key
// header, never the URL — consistent with the security note above.
func (p *gemini) ListModels(ctx context.Context) ([]ModelInfo, error) {
	url := strings.TrimRight(p.cfg.BaseURL, "/") + "/v1beta/models?pageSize=1000"
	headers := map[string]string{"x-goog-api-key": p.cfg.APIKey}
	body, err := getJSON(ctx, p.http, url, headers, ProviderGemini)
	if err != nil {
		return nil, err
	}
	var out struct {
		Models []struct {
			Name                       string   `json:"name"`
			DisplayName                string   `json:"displayName"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, &APIError{Kind: ErrUpstream, Provider: ProviderGemini, Message: "无法解析模型列表：" + err.Error()}
	}
	models := make([]ModelInfo, 0, len(out.Models))
	for _, m := range out.Models {
		if !slices.Contains(m.SupportedGenerationMethods, "generateContent") {
			continue // skip embedding / aqa / etc. — we only chat
		}
		if id := strings.TrimPrefix(m.Name, "models/"); id != "" {
			models = append(models, ModelInfo{ID: id, DisplayName: m.DisplayName})
		}
	}
	return models, nil
}
