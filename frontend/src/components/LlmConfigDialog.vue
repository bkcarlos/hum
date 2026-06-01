<script setup lang="ts">
import { computed, ref } from 'vue'
import { NModal, NCard, NForm, NFormItem, NSelect, NInput, NButton, NSpace, NAlert, NText } from 'naive-ui'
import { PROVIDER_PRESETS, presetById } from '@/data/providers'
import { useLlmConfigStore } from '@/stores/llmConfig'
import { testLlm } from '@/api/client'
import type { ApiError } from '@/types'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ 'update:show': [boolean] }>()
const visible = computed({
  get: () => props.show,
  set: (v) => emit('update:show', v),
})

const llm = useLlmConfigStore()

const presetOptions = PROVIDER_PRESETS.map((p) => ({ label: p.label, value: p.id }))
const modelOptions = computed(() => (presetById(llm.presetId)?.models ?? []).map((m) => ({ label: m, value: m })))

const testing = ref(false)
const testOk = ref<boolean | null>(null)
const testMsg = ref('')

function resetTest() {
  testOk.value = null
  testMsg.value = ''
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

function onClear() {
  llm.clear()
  resetTest()
}
</script>

<template>
  <n-modal v-model:show="visible">
    <n-card style="width: 560px; max-width: 92vw" title="LLM 设置（自带 Key · BYOK）" :bordered="false" role="dialog">
      <n-form label-placement="top" size="medium">
        <n-form-item label="Provider">
          <n-select
            :value="llm.presetId"
            :options="presetOptions"
            @update:value="(id: string) => { llm.applyPreset(id); resetTest() }"
          />
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
          <n-input v-model:value="llm.baseUrl" placeholder="https://…" @update:value="resetTest" />
        </n-form-item>

        <n-form-item label="模型">
          <n-select
            v-model:value="llm.model"
            filterable
            tag
            placeholder="选择或输入模型名"
            :options="modelOptions"
            @update:value="resetTest"
          />
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
