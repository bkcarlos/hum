import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import type { LlmBody, ProviderType } from '@/types'
import { presetById } from '@/data/providers'

// F0: the user's LLM config — including the API key — is persisted ONLY in this
// browser's localStorage. It is never sent anywhere except, per request, as the
// X-LLM-Api-Key header (used once by the backend, then discarded).
const LS_KEY = 'amllm.llmConfig.v1'

interface Persisted {
  presetId: string
  provider: ProviderType
  baseUrl: string
  model: string
  apiKey: string
}

export const useLlmConfigStore = defineStore('llmConfig', () => {
  const presetId = ref('openai')
  const provider = ref<ProviderType>('openai-compat')
  const baseUrl = ref('https://api.openai.com/v1')
  const model = ref('gpt-4o-mini')
  const apiKey = ref('')
  const tested = ref(false)

  const configured = computed(
    () => apiKey.value.trim() !== '' && model.value.trim() !== '' && baseUrl.value.trim() !== '',
  )
  const body = computed<LlmBody>(() => ({
    provider: provider.value,
    baseUrl: baseUrl.value.trim(),
    model: model.value.trim(),
  }))

  function applyPreset(id: string) {
    const p = presetById(id)
    if (!p) return
    presetId.value = id
    provider.value = p.type
    baseUrl.value = p.custom ? '' : p.baseUrl
    model.value = p.custom ? '' : (p.models[0] ?? '')
    tested.value = false
  }

  function load() {
    try {
      const raw = localStorage.getItem(LS_KEY)
      if (!raw) return
      const p = JSON.parse(raw) as Partial<Persisted>
      presetId.value = p.presetId ?? presetId.value
      provider.value = p.provider ?? provider.value
      baseUrl.value = p.baseUrl ?? baseUrl.value
      model.value = p.model ?? model.value
      apiKey.value = p.apiKey ?? ''
    } catch {
      // ignore corrupt storage
    }
  }

  function persist() {
    const p: Persisted = {
      presetId: presetId.value,
      provider: provider.value,
      baseUrl: baseUrl.value,
      model: model.value,
      apiKey: apiKey.value,
    }
    localStorage.setItem(LS_KEY, JSON.stringify(p))
  }

  /** One-click local wipe (F0). Removes the key from this browser entirely. */
  function clear() {
    apiKey.value = ''
    tested.value = false
    localStorage.removeItem(LS_KEY)
  }

  // Auto-persist on any change. (clear() removes the row; the subsequent write
  // from this watcher only re-stores config with an empty apiKey — no secret.)
  watch([presetId, provider, baseUrl, model, apiKey], persist)

  return { presetId, provider, baseUrl, model, apiKey, tested, configured, body, applyPreset, load, clear }
})
