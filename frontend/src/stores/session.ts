import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'

// The main app's LLM access has two modes:
//   'byok' — the user's own key (llmConfig store), unlimited.
//   'free' — Sign in with Apple → the server's key under a daily quota.
// This store holds the free-tier session + the chosen mode. The session is a
// credential (like the BYOK key) and lives only in this browser's localStorage.
const LS_SESSION = 'hum.session'
const LS_EMAIL = 'hum.email'
const LS_NAME = 'hum.name'
const LS_MODE = 'hum.authMode'

export type AuthMode = 'byok' | 'free'

export const useSessionStore = defineStore('session', () => {
  const session = ref<string>(localStorage.getItem(LS_SESSION) ?? '')
  const sub = ref<string>('')
  // Apple email (own, shown to the user); persisted so it survives a reload.
  const email = ref<string>(localStorage.getItem(LS_EMAIL) ?? '')
  // Apple full name — Apple only returns it on the FIRST authorization, so persist it.
  const name = ref<string>(localStorage.getItem(LS_NAME) ?? '')
  // Default to BYOK so an existing key-configured user is never bumped into a
  // login wall; newcomers can switch to 免费档 in the settings dialog.
  const mode = ref<AuthMode>(localStorage.getItem(LS_MODE) === 'free' ? 'free' : 'byok')

  const signedIn = computed(() => session.value !== '')
  // 展示名：优先 Apple 名字，否则用邮箱 @ 前的本地部分（隐去后缀）。
  const displayName = computed(() => name.value || email.value.split('@')[0] || '')

  function setSession(token: string, who = '', mail = '', nm = '') {
    session.value = token.trim()
    sub.value = who
    email.value = mail
    if (nm) name.value = nm // Apple 本次没给名字（非首登）则保留已存的
    if (session.value) {
      localStorage.setItem(LS_SESSION, session.value)
      mail ? localStorage.setItem(LS_EMAIL, mail) : localStorage.removeItem(LS_EMAIL)
      name.value ? localStorage.setItem(LS_NAME, name.value) : localStorage.removeItem(LS_NAME)
    } else {
      name.value = ''
      localStorage.removeItem(LS_SESSION)
      localStorage.removeItem(LS_EMAIL)
      localStorage.removeItem(LS_NAME)
    }
  }

  function signOut() {
    setSession('')
  }

  function setMode(m: AuthMode) {
    mode.value = m
  }

  watch(mode, (m) => localStorage.setItem(LS_MODE, m))

  return { session, sub, email, name, mode, signedIn, displayName, setSession, signOut, setMode }
})
