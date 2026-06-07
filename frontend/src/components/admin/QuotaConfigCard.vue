<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  NButton,
  NDynamicTags,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NSelect,
  NSpin,
  NSwitch,
  useMessage,
} from 'naive-ui'
import { getAdminConfig, updateAdminConfig, testAdminLLM } from '@/api/client'
import { useAdminStore } from '@/stores/admin'
import type { ApiError, ProviderType, QuotaConfig } from '@/types'

const admin = useAdminStore()
const message = useMessage()

const loading = ref(true)
const saving = ref(false)
const cfg = ref<QuotaConfig | null>(null)
const newKey = ref('') // typed-but-unsaved server LLM key; blank = leave unchanged
const testing = ref(false)
const testOk = ref<boolean | null>(null)
const testMsg = ref('')

const providerOptions: { label: string; value: ProviderType }[] = [
  { label: 'OpenAI 兼容（openai-compat）', value: 'openai-compat' },
  { label: 'Anthropic', value: 'anthropic' },
  { label: 'Gemini', value: 'gemini' },
]

async function load() {
  loading.value = true
  try {
    cfg.value = await getAdminConfig(admin.session)
  } catch (e) {
    message.error((e as ApiError).message)
  } finally {
    loading.value = false
  }
}

async function save() {
  if (!cfg.value) return
  saving.value = true
  try {
    // The whole config is sent; the server merges + preserves anything omitted.
    // The LLM key is only sent when newly typed (blank = leave the existing one).
    const patch: Partial<QuotaConfig> & { llmApiKey?: string } = { ...cfg.value }
    if (newKey.value.trim()) patch.llmApiKey = newKey.value.trim()
    cfg.value = await updateAdminConfig(admin.session, patch)
    newKey.value = ''
    message.success('配置已保存，立即生效（Firestore ≤30s 缓存）。')
  } catch (e) {
    message.error((e as ApiError).message)
  } finally {
    saving.value = false
  }
}

// Save the current form (incl. a newly-typed key) then ping the LLM, so we test
// exactly what's shown.
async function onTest() {
  if (!cfg.value) return
  testing.value = true
  testOk.value = null
  try {
    const patch: Partial<QuotaConfig> & { llmApiKey?: string } = { ...cfg.value }
    if (newKey.value.trim()) patch.llmApiKey = newKey.value.trim()
    cfg.value = await updateAdminConfig(admin.session, patch)
    newKey.value = ''
    await testAdminLLM(admin.session)
    testOk.value = true
    testMsg.value = '连接成功，默认 LLM 可用。'
  } catch (e) {
    testOk.value = false
    testMsg.value = (e as ApiError).message
  } finally {
    testing.value = false
  }
}

onMounted(load)
</script>

<template>
  <section class="sec">
    <header class="sec-head">
      <div>
        <h1>配额配置</h1>
        <p class="desc">改动即时生效，无需重启</p>
      </div>
      <div class="actions" v-if="cfg">
        <n-button :disabled="saving" @click="load">重置</n-button>
        <n-button type="primary" :loading="saving" @click="save">保存</n-button>
      </div>
    </header>

    <div v-if="loading" class="center"><n-spin /></div>
    <template v-else-if="cfg">
      <div class="card">
        <div class="group-label">免费档额度</div>
        <n-form label-placement="left" :label-width="120" :show-feedback="false">
          <n-form-item label="免费档开关">
            <n-switch v-model:value="cfg.enabled" />
            <span class="inline-hint">关闭后所有免费档请求一律 429（BYOK 不受影响）</span>
          </n-form-item>
          <n-form-item label="每人每日额度">
            <n-input-number v-model:value="cfg.perUserDailyLimit" :min="0" style="width: 150px" />
            <span class="inline-hint">0 = 不限；一次推荐 ≈ 2</span>
          </n-form-item>
          <n-form-item label="全局每日额度">
            <n-input-number v-model:value="cfg.globalDailyLimit" :min="0" style="width: 150px" />
            <span class="inline-hint">0 = 不限（护账单用）</span>
          </n-form-item>
        </n-form>
      </div>

      <div class="card">
        <div class="group-label">默认 LLM（免费档用服务端自有 key）</div>
        <n-form label-placement="left" :label-width="120" :show-feedback="false">
          <n-form-item label="Provider">
            <n-select v-model:value="cfg.llmProvider" :options="providerOptions" style="width: 300px" />
          </n-form-item>
          <n-form-item label="Base URL">
            <n-input v-model:value="cfg.llmBaseUrl" placeholder="https://…" />
          </n-form-item>
          <n-form-item label="Model">
            <n-input v-model:value="cfg.llmModel" placeholder="模型名" />
          </n-form-item>
          <n-form-item label="API Key">
            <n-space vertical :size="4" style="width: 100%">
              <n-input
                v-model:value="newKey"
                type="password"
                show-password-on="click"
                :placeholder="cfg.llmKeySet ? '已配置 · 留空不变，输入则替换' : '未配置 · 粘贴服务端 LLM Key'"
              />
              <n-text depth="3" style="font-size: 12px">
                服务端自有 key,存于后端、不会回显;留空保持不变。也可改用 Cloud Run Secret。
              </n-text>
            </n-space>
          </n-form-item>
          <n-form-item label="连通性">
            <n-space align="center" :size="10">
              <n-button size="small" :loading="testing" @click="onTest">保存并测试</n-button>
              <n-text v-if="testOk !== null" :type="testOk ? 'success' : 'error'" style="font-size: 12px">
                {{ testMsg }}
              </n-text>
            </n-space>
          </n-form-item>
        </n-form>
      </div>

      <div class="card">
        <div class="group-label">管理员（Apple sub 白名单）</div>
        <n-dynamic-tags v-model:value="cfg.admins" />
        <p class="card-hint">移除自己会即时失去后台访问；可在 GCP 控制台的 humQuota/config 文档重新加回。</p>
      </div>
    </template>
  </section>
</template>

<style scoped>
.sec-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 22px;
}
.sec-head h1 {
  font-size: 22px;
  font-weight: 700;
  margin: 0;
  color: #1d1d1f;
}
.desc {
  font-size: 13px;
  color: #86868b;
  margin: 4px 0 0;
}
.actions {
  display: flex;
  gap: 10px;
}
.card {
  background: #fff;
  border: 1px solid #ececef;
  border-radius: 16px;
  padding: 20px 22px;
  margin-bottom: 16px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03);
}
.group-label {
  font-size: 13px;
  font-weight: 600;
  color: #1d1d1f;
  margin-bottom: 16px;
}
.inline-hint {
  font-size: 12px;
  color: #98989d;
  margin-left: 12px;
}
.card-hint {
  font-size: 12px;
  color: #98989d;
  margin: 12px 0 0;
}
.center {
  display: flex;
  justify-content: center;
  padding: 60px 0;
}
</style>
