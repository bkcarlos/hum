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

// ── Full-track playback (subscribers, F6) ─────────────────────────────────
// MusicKit handles the subscription gate itself: a non-subscriber's play()
// rejects, which callers catch and fall back to the 30s preview.

const MK_PLAYING = 2 // MusicKit.PlaybackStates.playing

/** Queue the given catalog song ids and start full-track playback at startIndex. */
export async function playFullTracks(songIds: string[], startIndex = 0): Promise<void> {
  const mk = await ensureMusicKit()
  // 用 songs(catalog id 数组) 建队列时 setQueue 的 startPosition 不生效（它只对 items
  // 对象生效），会从第一首播。改为先建队列、再 changeToMediaAtIndex 跳到目标曲播放。
  await mk.setQueue({ songs: songIds })
  const i = Math.max(0, startIndex)
  if (i > 0) await mk.changeToMediaAtIndex(i)
  else await mk.play()
}

export async function skipNextFull(): Promise<void> {
  const mk = await ensureMusicKit()
  await mk.skipToNextItem()
}

export async function skipPrevFull(): Promise<void> {
  const mk = await ensureMusicKit()
  await mk.skipToPreviousItem()
}

export async function pauseFull(): Promise<void> {
  const mk = await ensureMusicKit()
  mk.pause()
}

export async function resumeFull(): Promise<void> {
  const mk = await ensureMusicKit()
  await mk.play()
}

/** Subscribe to playback-state changes; calls cb(isPlaying). Returns unsubscribe. */
export async function onFullPlaybackChange(cb: (playing: boolean) => void): Promise<() => void> {
  const mk = await ensureMusicKit()
  const handler = () => cb(mk.playbackState === MK_PLAYING)
  mk.addEventListener('playbackStateDidChange', handler)
  return () => mk.removeEventListener('playbackStateDidChange', handler)
}

/** Subscribe to now-playing-item changes; calls cb(currentCatalogId | ''). Returns unsubscribe. */
export async function onNowPlayingChange(cb: (id: string) => void): Promise<() => void> {
  const mk = await ensureMusicKit()
  const handler = () => cb(mk.nowPlayingItem?.id ?? '')
  mk.addEventListener('nowPlayingItemDidChange', handler)
  return () => mk.removeEventListener('nowPlayingItemDidChange', handler)
}
