import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

const LS_KEY = 'hum.admin.session'

/** Admin session token — the Sign in with Apple Bearer issued by /auth/apple.
 *  Persisted in this browser's localStorage so a refresh keeps you signed in.
 *  It is a credential (treat like a password); "退出" clears it. The signed-in
 *  Apple `sub` is filled after the token is verified against /admin/me. */
export const useAdminStore = defineStore('admin', () => {
  const session = ref<string>(localStorage.getItem(LS_KEY) ?? '')
  const sub = ref<string>('')
  const hasSession = computed(() => session.value !== '')

  function setSession(token: string) {
    session.value = token.trim()
    if (session.value) localStorage.setItem(LS_KEY, session.value)
    else localStorage.removeItem(LS_KEY)
  }

  function clear() {
    session.value = ''
    sub.value = ''
    localStorage.removeItem(LS_KEY)
  }

  return { session, sub, hasSession, setSession, clear }
})
