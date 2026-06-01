import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { ApiError } from '@/types'
import { authorize as mkAuthorize, unauthorize as mkUnauthorize } from '@/services/musickit'

// F1: the Music User Token is session-only. We deliberately keep it in memory
// (this store) and never write it to localStorage/sessionStorage/DB — closing
// or refreshing the tab requires re-authorization (§5.3).
export const useAppleStore = defineStore('apple', () => {
  const authorized = ref(false)
  const storefront = ref('')
  const userToken = ref('') // session-only, never persisted
  const connecting = ref(false)
  const error = ref('')

  async function connect() {
    connecting.value = true
    error.value = ''
    try {
      const res = await mkAuthorize()
      userToken.value = res.userToken
      storefront.value = res.storefront
      authorized.value = true
    } catch (e) {
      error.value = (e as ApiError)?.message ?? '连接 Apple Music 失败。'
      authorized.value = false
    } finally {
      connecting.value = false
    }
  }

  async function disconnect() {
    try {
      await mkUnauthorize()
    } catch {
      // ignore
    }
    authorized.value = false
    userToken.value = ''
    storefront.value = ''
  }

  return { authorized, storefront, userToken, connecting, error, connect, disconnect }
})
