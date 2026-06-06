import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { ApiError } from '@/types'
import { authorize as mkAuthorize, unauthorize as mkUnauthorize } from '@/services/musickit'

// Apple Music storefront ids are ISO-3166 alpha-2 (lowercase). Infer one from the
// browser locale's region subtag so recommendations + 30s previews work WITHOUT
// connecting Apple Music (catalog search uses the server Developer Token; previews
// use previewUrl). Connecting later replaces this with the account's real
// storefront. Falls back to 'us'.
function inferStorefront(): string {
  try {
    const langs = navigator.languages?.length ? navigator.languages : [navigator.language]
    for (const l of langs) {
      const m = /[-_]([A-Za-z]{2})(?:$|[-_])/.exec(l || '')
      if (m) return m[1].toLowerCase()
    }
  } catch {
    /* no navigator (SSR) */
  }
  return 'us'
}

// F1: the Music User Token is session-only. We deliberately keep it in memory
// (this store) and never write it to localStorage/sessionStorage/DB — closing
// or refreshing the tab requires re-authorization (§5.3). The storefront, by
// contrast, is non-sensitive and defaults to an inferred value.
export const useAppleStore = defineStore('apple', () => {
  const authorized = ref(false)
  const storefront = ref(inferStorefront()) // inferred until connected; replaced on connect
  const userToken = ref('') // session-only, never persisted
  const connecting = ref(false)
  const error = ref('')

  // Connect Apple Music (MusicKit). Needed only for full playback + building
  // playlists in the user's library — NOT for recommendations or 30s previews.
  // Returns whether authorization succeeded, so callers can connect-then-act.
  async function connect(): Promise<boolean> {
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
    return authorized.value
  }

  async function disconnect() {
    try {
      await mkUnauthorize()
    } catch {
      // ignore
    }
    authorized.value = false
    userToken.value = ''
    storefront.value = inferStorefront() // fall back to inferred, not empty
  }

  return { authorized, storefront, userToken, connecting, error, connect, disconnect }
})
