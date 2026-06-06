<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { NAlert, NSpin } from 'naive-ui'
import { useAdminStore } from '@/stores/admin'
import { adminMe, exchangeAppleToken, getAppleWebConfig } from '@/api/client'
import { appleSignIn, isAppleCancel, type AppleWebConfig } from '@/services/appleSignIn'
import type { ApiError } from '@/types'

const emit = defineEmits<{ authed: [] }>()
const admin = useAdminStore()

const error = ref('')
const appleLoading = ref(false)
const checking = ref(true) // fetching web-config on mount

// Web Sign in with Apple availability (server tells us if a Services ID is set).
// We intentionally support NO other login method: a session is a bearer token, so
// we never let one be pasted/handled by hand — it's only ever minted by Apple.
const webCfg = ref<AppleWebConfig | null>(null)

// Store the session, confirm it's an admin, then proceed. 403 = a valid session
// that isn't an admin; anything else = a bad/expired session.
async function finishWithSession(session: string) {
  admin.setSession(session)
  try {
    const { sub } = await adminMe(admin.session)
    admin.sub = sub
    emit('authed')
  } catch (e) {
    const err = e as ApiError
    error.value =
      err.code === 'forbidden' ? '该 Apple 账号不在管理员名单内。' : '会话无效或已过期，请重新登录。'
    admin.clear()
  }
}

async function onApple() {
  if (!webCfg.value || appleLoading.value) return
  appleLoading.value = true
  error.value = ''
  try {
    const idToken = await appleSignIn(webCfg.value)
    const { session } = await exchangeAppleToken(idToken)
    await finishWithSession(session)
  } catch (e) {
    if (!isAppleCancel(e)) {
      error.value = (e as ApiError).message || (e as Error).message || 'Apple 登录失败，请重试。'
    }
  } finally {
    appleLoading.value = false
  }
}

onMounted(async () => {
  try {
    const cfg = await getAppleWebConfig()
    if (cfg.enabled && cfg.clientId) {
      webCfg.value = { clientId: cfg.clientId, redirectUri: cfg.redirectUri ?? '', scope: cfg.scope ?? '' }
    }
  } catch {
    // endpoint absent / free tier off → no web Apple login configured.
  } finally {
    checking.value = false
  }
})
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

      <div v-if="checking" class="center"><n-spin /></div>

      <!-- Apple is the only login method. -->
      <template v-else-if="webCfg">
        <button class="apple-btn" :disabled="appleLoading" @click="onApple">
          <svg class="apple-logo" viewBox="0 0 384 512" aria-hidden="true">
            <path
              fill="currentColor"
              d="M318.7 268.7c-.2-36.7 16.4-64.4 50-84.8-18.8-26.9-47.2-41.7-84.7-44.6-35.5-2.8-74.3 20.7-88.5 20.7-15 0-49.4-19.7-76.4-19.7C63.3 141.2 4 184.8 4 273.5q0 39.3 14.4 81.2c12.8 36.7 59 126.7 107.2 125.2 25.2-.6 43-17.9 75.8-17.9 31.8 0 48.3 17.9 76.4 17.9 48.6-.7 90.4-82.5 102.6-119.3-65.2-30.7-61.7-90-61.7-91.9zm-56.6-164.2c27.3-32.4 24.8-61.9 24-72.5-24.1 1.4-52 16.4-67.9 34.9-17.5 19.8-27.8 44.3-25.6 71.9 26.1 2 49.9-11.4 69.5-34.3z"
            />
          </svg>
          <span>{{ appleLoading ? '登录中…' : '通过 Apple 登录' }}</span>
        </button>
        <div v-if="error" class="err">{{ error }}</div>
      </template>

      <!-- Services ID not configured yet → no login is possible on web. -->
      <n-alert v-else type="info" :bordered="false">
        网页登录需先在 Apple 开发者后台配置 Services ID（见 docs/freetier-roadmap.md §6.B）并设置
        <code>APPLE_WEB_CLIENT_ID</code>。配置后这里会出现「通过 Apple 登录」。
      </n-alert>

      <div class="help">
        只支持 Sign in with Apple 登录（不接受手动粘贴令牌）。首个管理员引导：登录后用
        <code>/api/auth/me</code> 拿到自己的 <code>sub</code>，写进 Firestore <code>humQuota/config</code> 的
        <code>admins</code> 即可；也可直接在 GCP 控制台编辑该文档。
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
.center {
  display: flex;
  justify-content: center;
  padding: 16px 0;
}
.apple-btn {
  width: 100%;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  background: #000;
  color: #fff;
  border: none;
  border-radius: 12px;
  font-size: 16px;
  font-weight: 500;
  cursor: pointer;
}
.apple-btn:disabled {
  opacity: 0.6;
  cursor: default;
}
.apple-logo {
  width: 17px;
  height: 17px;
  margin-top: -2px;
}
.err {
  color: #d03050;
  font-size: 13px;
  margin-top: 12px;
}
.help {
  margin-top: 18px;
  padding: 14px 16px;
  background: #f7f7f9;
  border-radius: 12px;
  font-size: 12px;
  line-height: 1.8;
  color: #6e6e73;
}
code {
  background: #00000010;
  padding: 1px 5px;
  border-radius: 4px;
  font-size: 11px;
  color: #1d1d1f;
}
</style>
