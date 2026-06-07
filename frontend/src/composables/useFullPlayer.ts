import { ref } from 'vue'
import type { Song } from '@/types'
import {
  onFullPlaybackChange,
  onNowPlayingChange,
  pauseFull,
  playFullTracks,
  resumeFull,
  skipNextFull,
  skipPrevFull,
} from '@/services/musickit'

// Full-track playback for Apple Music subscribers (F6). Mirrors usePreviewPlayer's
// shape (currentId / playing / toggle / next / prev) so the UI can dispatch to
// whichever player matches the current mode（连上=完整 / 未连=30s 试听）.
// Module-level refs = one shared instance.
const currentId = ref('')
const playing = ref(false)
let ids: string[] = []
let listening = false

async function ensureListeners() {
  if (listening) return
  listening = true
  await onFullPlaybackChange((p) => (playing.value = p))
  await onNowPlayingChange((id) => {
    if (id) currentId.value = id // 自动续播时跟随当前曲
  })
}

function setQueue(songs: Song[]) {
  ids = songs.map((s) => s.id)
}

/** Toggle full playback of a song. Returns false if it couldn't play (non-subscriber
 *  / region mismatch / error) so the caller can fall back to the 30s preview. */
async function toggle(song: Song): Promise<boolean> {
  await ensureListeners()
  // 同一首：暂停 / 续播。
  if (currentId.value === song.id) {
    try {
      if (playing.value) await pauseFull()
      else await resumeFull()
      return true
    } catch {
      return false
    }
  }
  // 换一首：从该曲起播完整队列。乐观设 currentId（挡二次点击重播），失败回滚。
  const i = ids.indexOf(song.id)
  currentId.value = song.id
  playing.value = true
  try {
    await playFullTracks(ids, i < 0 ? 0 : i)
    return true
  } catch {
    currentId.value = ''
    playing.value = false
    return false
  }
}

async function next() {
  try {
    await skipNextFull()
  } catch {
    /* ignore */
  }
}

async function prev() {
  try {
    await skipPrevFull()
  } catch {
    /* ignore */
  }
}

function stop() {
  void pauseFull()
  playing.value = false
  currentId.value = ''
}

export function useFullPlayer() {
  return { currentId, playing, setQueue, toggle, next, prev, stop }
}
