<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  NModal,
  NCard,
  NForm,
  NFormItem,
  NSelect,
  NInput,
  NInputGroup,
  NButton,
  NSpace,
  NAlert,
  NText,
} from 'naive-ui'
import { PROVIDER_PRESETS, presetById } from '@/data/providers'
import { useLlmConfigStore } from '@/stores/llmConfig'
import { useSessionStore } from '@/stores/session'
import { useAppleLogin } from '@/composables/useAppleLogin'
import { testLlm, listModels } from '@/api/client'
import type { ApiError, ModelInfo } from '@/types'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ 'update:show': [boolean] }>()
const visible = computed({
  get: () => props.show,
  set: (v) => emit('update:show', v),
})

const llm = useLlmConfigStore()
const session = useSessionStore()

// ── Free tier (Sign in with Apple) — shared flow, also used by the topbar CTA.
const { webCfg, loading: appleLoading, error: freeErr, login: onAppleLogin, ensureConfig } = useAppleLogin()

// ── BYOK ──────────────────────────────────────────────────────────────
const presetOptions = PROVIDER_PRESETS.map((p) => ({ label: p.label, value: p.id }))

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

onMounted(ensureConfig)
</script>

<template>
  <n-modal v-model:show="visible">
    <n-card
      style="width: 560px; max-width: 92vw"
      :title="session.mode === 'free' ? '接入设置 · 免费额度' : '接入设置 · 自带 Key'"
      :bordered="false"
      role="dialog"
    >
      <!-- The active mode is chosen via the two topbar buttons; this link is just
           a quick in-dialog switch to the other mode. -->
      <div style="margin-bottom: 16px">
        <n-button
          text
          type="primary"
          size="small"
          @click="session.setMode(session.mode === 'free' ? 'byok' : 'free')"
        >
          {{ session.mode === 'free' ? '改用「自带 Key」→' : '改用「免费额度」→' }}
        </n-button>
      </div>

      <!-- FREE TIER -->
      <div v-if="session.mode === 'free'">
        <div v-if="session.signedIn" class="signed-in">
          <n-space vertical :size="10">
            <n-text>已登录：<b>{{ session.email || session.sub || 'Apple 账号' }}</b></n-text>
            <n-text depth="3" style="font-size: 12px">
              正在使用免费额度（服务端共享 Key，按每日配额）。额度用尽会提示你改用自带 Key。
            </n-text>
            <div><n-button size="small" @click="session.signOut()">退出登录</n-button></div>
          </n-space>
        </div>
        <div v-else>
          <button v-if="webCfg" class="apple-btn" :disabled="appleLoading" @click="onAppleLogin">
            <svg class="apple-logo" viewBox="0 0 384 512" aria-hidden="true">
              <path
                fill="currentColor"
                d="M318.7 268.7c-.2-36.7 16.4-64.4 50-84.8-18.8-26.9-47.2-41.7-84.7-44.6-35.5-2.8-74.3 20.7-88.5 20.7-15 0-49.4-19.7-76.4-19.7C63.3 141.2 4 184.8 4 273.5q0 39.3 14.4 81.2c12.8 36.7 59 126.7 107.2 125.2 25.2-.6 43-17.9 75.8-17.9 31.8 0 48.3 17.9 76.4 17.9 48.6-.7 90.4-82.5 102.6-119.3-65.2-30.7-61.7-90-61.7-91.9zm-56.6-164.2c27.3-32.4 24.8-61.9 24-72.5-24.1 1.4-52 16.4-67.9 34.9-17.5 19.8-27.8 44.3-25.6 71.9 26.1 2 49.9-11.4 69.5-34.3z"
              />
            </svg>
            <span>{{ appleLoading ? '登录中…' : '通过 Apple 登录' }}</span>
          </button>
          <n-alert v-else type="info" :bordered="false">
            网页版 Apple 登录暂未开放，请切到「自带 Key」使用自己的 LLM Key。
          </n-alert>
          <n-text depth="3" style="font-size: 12px; display: block; margin-top: 12px">
            用 Apple 登录即可使用免费额度，无需自备 LLM Key。每天有使用上限；超出后可随时切到自带 Key。
          </n-text>
          <n-alert v-if="freeErr" type="error" style="margin-top: 12px">{{ freeErr }}</n-alert>
        </div>
      </div>

      <!-- BYOK -->
      <div v-else>
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

        <n-alert type="info" :bordered="false" style="margin-bottom: 4px">
          <n-text depth="3" style="font-size: 12px">
            你的 API Key 仅保存在本浏览器，调用时随请求转发给后端用于本次 LLM 调用，调用结束即丢弃；我们不在服务器保存、不记录到日志。请勿在公共设备上保存。
          </n-text>
        </n-alert>
      </div>

      <template #footer>
        <n-space justify="space-between">
          <n-button v-if="session.mode === 'byok'" quaternary type="error" @click="onClear">清除本地配置</n-button>
          <span v-else />
          <n-space>
            <n-button @click="visible = false">关闭</n-button>
            <n-button
              v-if="session.mode === 'byok'"
              type="primary"
              :loading="testing"
              :disabled="!llm.configured"
              @click="onTest"
            >
              测试连接
            </n-button>
          </n-space>
        </n-space>
      </template>
    </n-card>
  </n-modal>
</template>

<style scoped>
.apple-btn {
  width: 100%;
  height: 46px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  background: #000;
  color: #fff;
  border: none;
  border-radius: 12px;
  font-size: 15px;
  font-weight: 500;
  cursor: pointer;
}
.apple-btn:disabled {
  opacity: 0.6;
  cursor: default;
}
.apple-logo {
  width: 16px;
  height: 16px;
  margin-top: -2px;
}
.signed-in {
  padding: 4px 2px;
}
</style>
