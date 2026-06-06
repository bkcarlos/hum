<script setup lang="ts">
import { ref } from 'vue'
import { NButton, NInput } from 'naive-ui'
import { useAdminStore } from '@/stores/admin'
import { adminMe } from '@/api/client'
import type { ApiError } from '@/types'

const emit = defineEmits<{ authed: [] }>()
const admin = useAdminStore()

const token = ref('')
const error = ref('')
const loading = ref(false)
const showHelp = ref(false)

async function submit() {
  const t = token.value.trim()
  if (!t || loading.value) return
  loading.value = true
  error.value = ''
  admin.setSession(t)
  try {
    const { sub } = await adminMe(admin.session)
    admin.sub = sub
    token.value = ''
    emit('authed')
  } catch (e) {
    const err = e as ApiError
    // 403 = valid session that isn't an admin; else a bad/expired token.
    error.value =
      err.code === 'forbidden'
        ? '该 Apple 账号不在管理员名单内。'
        : '会话无效或已过期，请粘贴有效的会话令牌。'
    admin.clear()
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-bg">
    <div class="login-card">
      <div class="head">
        <img class="logo" src="/logo.svg" alt="Hum" width="46" height="46" />
        <div class="titles">
          <div class="t1">Hum 管理后台</div>
          <div class="t2">管理免费额度 · 用量 · 封禁</div>
        </div>
      </div>

      <label class="field-label">会话令牌（Session Token）</label>
      <n-input
        v-model:value="token"
        type="textarea"
        :rows="3"
        placeholder="粘贴 Sign in with Apple 登录后签发的 session…"
        @keyup.enter="submit"
      />
      <div v-if="error" class="err">{{ error }}</div>

      <n-button
        type="primary"
        block
        size="large"
        :loading="loading"
        :disabled="!token.trim()"
        style="margin-top: 16px"
        @click="submit"
      >
        登录
      </n-button>

      <button class="help-toggle" type="button" @click="showHelp = !showHelp">
        {{ showHelp ? '收起说明' : '怎么获取令牌？' }}
      </button>
      <div v-if="showHelp" class="help">
        用 Sign in with Apple 登录后，由 <code>POST /api/auth/apple</code> 换取 session 令牌。<br />
        首个管理员：用该 session 调 <code>GET /api/auth/me</code> 拿到自己的 <code>sub</code>，写进 Firestore
        <code>humQuota/config</code> 文档的 <code>admins</code> 数组即可。<br />
        <span class="soon">网页版 Sign in with Apple 登录即将支持（需配置 Services ID）。</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.login-bg {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: radial-gradient(1200px 600px at 50% -10%, #ffe6ea 0%, #f5f5f7 55%);
}
.login-card {
  width: 440px;
  max-width: 100%;
  background: #fff;
  border: 1px solid #ececef;
  border-radius: 22px;
  padding: 32px;
  box-shadow: 0 18px 50px rgba(0, 0, 0, 0.08);
}
.head {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 24px;
}
.logo {
  border-radius: 11px;
}
.t1 {
  font-size: 19px;
  font-weight: 700;
  color: #1d1d1f;
}
.t2 {
  font-size: 13px;
  color: #86868b;
  margin-top: 3px;
}
.field-label {
  display: block;
  font-size: 13px;
  font-weight: 600;
  color: #1d1d1f;
  margin-bottom: 8px;
}
.err {
  color: #d03050;
  font-size: 13px;
  margin-top: 10px;
}
.help-toggle {
  display: block;
  margin: 16px auto 0;
  background: none;
  border: none;
  color: #fa2d48;
  font-size: 13px;
  cursor: pointer;
  padding: 4px;
}
.help {
  margin-top: 12px;
  padding: 14px 16px;
  background: #f7f7f9;
  border-radius: 12px;
  font-size: 12px;
  line-height: 1.8;
  color: #6e6e73;
}
.help .soon {
  color: #98989d;
}
code {
  background: #00000010;
  padding: 1px 5px;
  border-radius: 4px;
  font-size: 11px;
  color: #1d1d1f;
}
</style>
