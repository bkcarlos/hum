<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { NAlert, NButton, NCard, NInput, NSpace, NSpin, NText } from 'naive-ui'
import { useAdminStore } from '@/stores/admin'
import { adminMe } from '@/api/client'
import type { ApiError } from '@/types'
import QuotaConfigCard from './QuotaConfigCard.vue'
import UsageCard from './UsageCard.vue'

const admin = useAdminStore()

type Phase = 'checking' | 'login' | 'ready'
const phase = ref<Phase>('checking')
const tokenInput = ref('')
const loginError = ref('')
const verifying = ref(false)

// Verify the stored session against the admin gate. 403 = a valid session that
// isn't an admin; anything else = bad/expired token. Either way, back to login.
async function verify() {
  verifying.value = true
  loginError.value = ''
  try {
    const { sub } = await adminMe(admin.session)
    admin.sub = sub
    phase.value = 'ready'
  } catch (e) {
    const err = e as ApiError
    loginError.value =
      err.code === 'forbidden'
        ? '该 Apple 账号不在管理员名单内。'
        : '会话无效或已过期，请重新粘贴有效的会话令牌。'
    admin.clear()
    phase.value = 'login'
  } finally {
    verifying.value = false
  }
}

function onLogin() {
  const t = tokenInput.value.trim()
  if (!t) return
  admin.setSession(t)
  tokenInput.value = ''
  verify()
}

function onLogout() {
  admin.clear()
  phase.value = 'login'
}

onMounted(() => {
  if (admin.hasSession) verify()
  else phase.value = 'login'
})
</script>

<template>
  <div class="admin">
    <header class="bar">
      <div class="title">Hum · 管理后台</div>
      <n-space v-if="phase === 'ready'" align="center" :size="12">
        <n-text depth="3" style="font-size: 12px; font-family: monospace">{{ admin.sub }}</n-text>
        <n-button size="small" quaternary @click="onLogout">退出</n-button>
      </n-space>
    </header>

    <main class="body">
      <div v-if="phase === 'checking'" class="center"><n-spin /></div>

      <n-card v-else-if="phase === 'login'" title="管理员登录" class="login">
        <n-space vertical :size="12">
          <n-text depth="3" style="font-size: 13px; line-height: 1.6">
            粘贴 Sign in with Apple 登录后由 <code>/api/auth/apple</code> 签发的 session 令牌。
            首个管理员引导：用该 session 调 <code>/api/auth/me</code> 拿到自己的 <code>sub</code>，
            写进 Firestore <code>humQuota/config</code> 文档的 <code>admins</code> 数组即可。
          </n-text>
          <n-input
            v-model:value="tokenInput"
            type="textarea"
            :rows="3"
            placeholder="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9…"
          />
          <n-alert v-if="loginError" type="error">{{ loginError }}</n-alert>
          <n-button type="primary" :loading="verifying" :disabled="!tokenInput.trim()" @click="onLogin">
            登录
          </n-button>
        </n-space>
      </n-card>

      <div v-else class="grid">
        <QuotaConfigCard />
        <UsageCard />
      </div>
    </main>
  </div>
</template>

<style scoped>
.admin {
  min-height: 100vh;
  background: #f5f5f7;
}
.bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 24px;
  background: #fff;
  border-bottom: 1px solid #ececec;
}
.title {
  font-weight: 700;
  font-size: 17px;
}
.body {
  max-width: 980px;
  margin: 0 auto;
  padding: 24px;
}
.center {
  display: flex;
  justify-content: center;
  padding: 80px 0;
}
.login {
  max-width: 560px;
  margin: 40px auto;
}
.grid {
  display: grid;
  gap: 20px;
}
code {
  background: #00000010;
  padding: 1px 5px;
  border-radius: 4px;
  font-size: 12px;
}
</style>
