// MusicKit JS integration (F1). The SDK is loaded lazily from Apple's CDN and
// configured with a Developer Token minted by our backend. The resulting Music
// User Token lives only in memory for the session — never persisted (§5.3).
import { getDeveloperToken } from '@/api/client'

const MUSICKIT_CDN = 'https://js-cdn.music.apple.com/musickit/v3/musickit.js'

let scriptPromise: Promise<void> | null = null
let configurePromise: Promise<MusicKitInstance> | null = null

function loadScript(): Promise<void> {
  if (scriptPromise) return scriptPromise
  scriptPromise = new Promise((resolve, reject) => {
    if (window.MusicKit) return resolve()
    const el = document.createElement('script')
    el.src = MUSICKIT_CDN
    el.async = true
    // MusicKit v3 fires `musickitloaded` on document once ready.
    document.addEventListener('musickitloaded', () => resolve(), { once: true })
    el.onerror = () => reject(new Error('无法加载 MusicKit JS（请检查网络或是否被拦截）。'))
    document.head.appendChild(el)
  })
  return scriptPromise
}

/** Ensure MusicKit is loaded and configured with a fresh Developer Token. */
export async function ensureMusicKit(): Promise<MusicKitInstance> {
  if (configurePromise) return configurePromise
  configurePromise = (async () => {
    const { token } = await getDeveloperToken()
    await loadScript()
    return window.MusicKit.configure({
      developerToken: token,
      app: {
        name: import.meta.env.VITE_APP_NAME ?? 'Hum',
        build: import.meta.env.VITE_APP_BUILD ?? '0.5.0',
      },
    })
  })()
  return configurePromise
}

export interface AppleAuthResult {
  userToken: string
  storefront: string
}

/** Trigger the Apple authorization popup; returns the session user token and
 *  the user's storefront (e.g. "us", "cn"). storefront drives all subsequent
 *  catalog/playlist calls so song ids stay consistent (F4). */
export async function authorize(): Promise<AppleAuthResult> {
  const mk = await ensureMusicKit()
  const userToken = await mk.authorize()
  return { userToken, storefront: mk.storefrontId }
}

export async function unauthorize(): Promise<void> {
  const mk = await ensureMusicKit()
  await mk.unauthorize()
}
