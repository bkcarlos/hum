import { computed, ref } from 'vue'
import { useSessionStore } from '@/stores/session'
import { useAppleStore } from '@/stores/apple'
import { exchangeAppleToken, getAppleWebConfig, getMe } from '@/api/client'
import { appleSignIn, isAppleCancel, type AppleWebConfig } from '@/services/appleSignIn'
import type { ApiError } from '@/types'

// Shared free-tier login flow, used by BOTH the topbar primary CTA and the 接入设置
// dialog so the web-config is fetched once and login state stays consistent.
// Module-level refs = a single shared instance across callers.
const webCfg = ref<AppleWebConfig | null>(null)
const checked = ref(false)
const loading = ref(false)
const error = ref('')

export function useAppleLogin() {
  const session = useSessionStore()
  const apple = useAppleStore()

  // Fetch the (non-secret) web Sign in with Apple config once. enabled:false /
  // endpoint absent ⇒ web login isn't set up server-side (no Services ID).
  async function ensureConfig() {
    if (checked.value) return
    checked.value = true
    try {
      const cfg = await getAppleWebConfig()
      if (cfg.enabled && cfg.clientId) {
        webCfg.value = { clientId: cfg.clientId, redirectUri: cfg.redirectUri ?? '', scope: cfg.scope ?? '' }
      }
    } catch {
      /* web Apple login unavailable */
    }
  }

  // Sign in with Apple → exchange for our session → store it (+ email), then
  // auto-connect Apple Music (best-effort; a non-subscriber/cancel won't undo the
  // login). Sets session.mode = 'free'.
  async function login() {
    await ensureConfig()
    if (!webCfg.value || loading.value) return
    loading.value = true
    error.value = ''
    try {
      const idToken = await appleSignIn(webCfg.value)
      const { session: tok } = await exchangeAppleToken(idToken)
      let sub = ''
      let email = ''
      try {
        const me = await getMe(tok)
        sub = me.sub
        email = me.email
      } catch {
        /* sub/email are display-only */
      }
      session.setSession(tok, sub, email)
      session.setMode('free')
      if (!apple.authorized) await apple.connect()
    } catch (e) {
      if (!isAppleCancel(e)) {
        error.value = (e as ApiError).message || (e as Error).message || 'Apple 登录失败，请重试。'
      }
    } finally {
      loading.value = false
    }
  }

  return {
    webCfg,
    available: computed(() => !!webCfg.value),
    loading,
    error,
    ensureConfig,
    login,
  }
}
