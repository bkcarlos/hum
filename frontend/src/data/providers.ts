import type { ProviderType } from '@/types'

/** A selectable BYOK provider preset. Selecting one fills Base URL + a default
 *  model; the user can still edit every field afterwards (F0). The model lists
 *  are sensible defaults, not an exhaustive or guaranteed-current catalog. */
export interface ProviderPreset {
  id: string
  label: string
  type: ProviderType
  baseUrl: string
  models: string[]
  /** When true, all fields are blank/free-form for unlisted vendors. */
  custom?: boolean
}

export const PROVIDER_PRESETS: ProviderPreset[] = [
  // ── OpenAI-compatible (one adapter covers all) ──────────────────────
  { id: 'openai', label: 'OpenAI', type: 'openai-compat', baseUrl: 'https://api.openai.com/v1', models: ['gpt-4o-mini', 'gpt-4o'] },
  { id: 'deepseek', label: 'DeepSeek', type: 'openai-compat', baseUrl: 'https://api.deepseek.com/v1', models: ['deepseek-chat'] },
  { id: 'qwen', label: '通义千问 (DashScope 兼容)', type: 'openai-compat', baseUrl: 'https://dashscope.aliyuncs.com/compatible-mode/v1', models: ['qwen-plus', 'qwen-turbo'] },
  { id: 'moonshot', label: 'Moonshot (Kimi)', type: 'openai-compat', baseUrl: 'https://api.moonshot.cn/v1', models: ['moonshot-v1-8k'] },
  { id: 'zhipu', label: '智谱 GLM', type: 'openai-compat', baseUrl: 'https://open.bigmodel.cn/api/paas/v4', models: ['glm-4-flash', 'glm-4'] },
  { id: 'minimax', label: 'MiniMax', type: 'openai-compat', baseUrl: 'https://api.minimax.chat/v1', models: ['abab6.5s-chat'] },
  { id: 'openrouter', label: 'OpenRouter', type: 'openai-compat', baseUrl: 'https://openrouter.ai/api/v1', models: ['openai/gpt-4o-mini'] },

  // ── Anthropic native (/v1/messages) ─────────────────────────────────
  { id: 'anthropic', label: 'Anthropic Claude', type: 'anthropic', baseUrl: 'https://api.anthropic.com', models: ['claude-sonnet-4-6', 'claude-haiku-4-5-20251001', 'claude-opus-4-8'] },

  // ── Gemini native (generateContent) ─────────────────────────────────
  { id: 'gemini', label: 'Google Gemini', type: 'gemini', baseUrl: 'https://generativelanguage.googleapis.com', models: ['gemini-2.0-flash', 'gemini-1.5-flash', 'gemini-1.5-pro'] },

  // ── Escape hatch for anything not preset ────────────────────────────
  { id: 'custom', label: '自定义 OpenAI 兼容', type: 'openai-compat', baseUrl: '', models: [], custom: true },
]

export function presetById(id: string): ProviderPreset | undefined {
  return PROVIDER_PRESETS.find((p) => p.id === id)
}
