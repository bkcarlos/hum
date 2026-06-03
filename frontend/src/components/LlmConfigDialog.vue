<script setup lang="ts">
import { computed, ref } from 'vue'
import { NModal, NCard, NForm, NFormItem, NSelect, NInput, NInputGroup, NButton, NSpace, NAlert, NText } from 'naive-ui'
import { PROVIDER_PRESETS, presetById } from '@/data/providers'
import { useLlmConfigStore } from '@/stores/llmConfig'
import { testLlm, listModels } from '@/api/client'
import type { ApiError, ModelInfo } from '@/types'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ 'update:show': [boolean] }>()
const visible = computed({
  get: () => props.show,
  set: (v) => emit('update:show', v),
})

const llm = useLlmConfigStore()

const presetOptions = PROVIDER_PRESETS.map((p) => ({ label: p.label, value: p.id }))

// Models fetched live from the provider (/llm/models), merged with the static
// preset list. The select stays free-form (tag) so unlisted models — and
// gateways without a list endpoint — still work via manual entry.
const fetchedModels = ref<ModelInfo[]>([])
const modelOptions = computed(() => {
  const seen = new Set<string>()
  const opts: { label: string; value: string }[] = []
  for (const m of fetchedModels.value) {
    if (m.id && !seen.has(m.id)) {
      seen.add(m.id)
      opts.push({ label: m.id, value: m.id })
    }
  }
  for (const id of presetById(llm.presetId)?.models ?? []) {
    if (!seen.has(id)) {
      seen.add(id)
      opts.push({ label: id, value: id })
    }
  }
  opts.sort((a, b) => a.value.localeCompare(b.value))
  return opts
})

const testing = ref(false)
const testOk = ref<boolean | null>(null)
const testMsg = ref('')

const loadingModels = ref(false)
const modelsOk = ref<boolean | null>(null)
const modelsMsg = ref('')

// Listing only needs a key + Base URL (not a model yet).
const canFetch = computed(() => llm.apiKey.trim() !== '' && llm.baseUrl.trim() !== '')

function resetTest() {
  testOk.value = null
  testMsg.value = ''
}

function resetModels() {
  fetchedModels.value = []
  modelsOk.value = null
  modelsMsg.value = ''
}

function onPreset(id: string) {
  llm.applyPreset(id)
  resetTest()
  resetModels()
}

function onBaseUrlChange() {
  resetTest()
  resetModels()
}

async function onTest() {
  resetTest()
  testing.value = true
  try {
    await testLlm(llm.body, llm.apiKey)
    testOk.value = true
    testMsg.value = '连接成功，Key 有效。'
    llm.tested = true
  } catch (e) {
    testOk.value = false
    testMsg.value = (e as ApiError).message
    llm.tested = false
  } finally {
    testing.value = false
  }
}

async function onFetchModels() {
  modelsOk.value = null
  modelsMsg.value = ''
  loadingModels.value = true
  try {
    const { models } = await listModels(llm.body, llm.apiKey)
    fetchedModels.value = models
    modelsOk.value = true
    modelsMsg.value = models.length
      ? `已拉取 ${models.length} 个模型，可在上方下拉中选择。`
      : '该服务商未返回模型列表，请手动输入模型名。'
  } catch (e) {
    modelsOk.value = false
    const reason = (e as ApiError).message
    modelsMsg.value = reason ? `拉取失败：${reason}（可手动输入模型名）` : '拉取失败，请手动输入模型名。'
  } finally {
    loadingModels.value = false
  }
}

function onClear() {
  llm.clear()
  resetTest()
  resetModels()
}
</script>

<template>
  <n-modal v-model:show="visible">
    <n-card style="width: 560px; max-width: 92vw" title="LLM 设置（自带 Key · BYOK）" :bordered="false" role="dialog">
      <n-form label-placement="top" size="medium">
        <n-form-item label="Provider">
          <n-select :value="llm.presetId" :options="presetOptions" @update:value="onPreset" />
        </n-form-item>

        <n-form-item label="API Key">
          <n-input
            v-model:value="llm.apiKey"
            type="password"
            show-password-on="click"
            placeholder="粘贴你的 API Key"
            @update:value="resetTest"
          />
        </n-form-item>

        <n-form-item label="Base URL">
          <n-input v-model:value="llm.baseUrl" placeholder="https://…" @update:value="onBaseUrlChange" />
        </n-form-item>

        <n-form-item label="模型">
          <n-space vertical :size="4" style="width: 100%">
            <n-input-group>
              <n-select
                v-model:value="llm.model"
                filterable
                tag
                placeholder="选择或输入模型名"
                :options="modelOptions"
                @update:value="resetTest"
              />
              <n-button :loading="loadingModels" :disabled="!canFetch" @click="onFetchModels">拉取模型</n-button>
            </n-input-group>
            <n-text v-if="modelsMsg" :type="modelsOk ? 'success' : 'error'" style="font-size: 12px">
              {{ modelsMsg }}
            </n-text>
          </n-space>
        </n-form-item>
      </n-form>

      <n-alert v-if="testOk !== null" :type="testOk ? 'success' : 'error'" style="margin-bottom: 12px">
        {{ testMsg }}
      </n-alert>

      <n-alert type="info" :bordered="false" style="margin-bottom: 16px">
        <n-text depth="3" style="font-size: 12px">
          你的 API Key 仅保存在本浏览器，调用时随请求转发给后端用于本次 LLM 调用，调用结束即丢弃；我们不在服务器保存、不记录到日志。请勿在公共设备上保存。
        </n-text>
      </n-alert>

      <template #footer>
        <n-space justify="space-between">
          <n-button quaternary type="error" @click="onClear">清除本地配置</n-button>
          <n-space>
            <n-button @click="visible = false">关闭</n-button>
            <n-button type="primary" :loading="testing" :disabled="!llm.configured" @click="onTest">
              测试连接
            </n-button>
          </n-space>
        </n-space>
      </template>
    </n-card>
  </n-modal>
</template>
