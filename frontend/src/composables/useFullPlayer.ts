import { ref } from 'vue'
import type { Song } from '@/types'
import {
  onFullLoadingChange,
  onFullPlaybackChange,
  onNowPlayingChange,
  onPlaybackTimeChange,
  pauseFull,
  playFullTracks,
  resumeFull,
  seekFull,
  skipNextFull,
  skipPrevFull,
} from '@/services/musickit'

// Full-track playback for Apple Music subscribers (F6). Mirrors usePreviewPlayer's
// shape (currentId / playing / loading / progress / duration / toggle / seek /
// next / prev) so the UI can dispatch to whichever player matches the current
// mode（连上=完整 / 未连=30s 试听）. Module-level refs = one shared instance.
const currentId = ref('')
const playing = ref(false)
const loading = ref(false) // 点击后到出声之间 / 缓冲（非下载文件）
const progress = ref(0) // 当前播放位置（秒）
const duration = ref(0) // 当前曲总时长（秒）
let ids: string[] = []
let listening = false

async function ensureListeners() {
  if (listening) return
  listening = true
  await onFullPlaybackChange((p) => (playing.value = p))
  await onFullLoadingChange((l) => (loading.value = l))
  await onPlaybackTimeChange((t, d) => {
    progress.value = t
    duration.value = d
  })
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
  progress.value = 0
  duration.value = 0
  loading.value = true
  playing.value = true
  try {
    await playFullTracks(ids, i < 0 ? 0 : i)
    return true
  } catch {
    currentId.value = ''
    playing.value = false
    loading.value = false
    return false
  }
}

/** Seek full playback to t seconds (drag-to-seek on the progress bar). */
async function seek(t: number) {
  try {
    await seekFull(t)
    progress.value = t
  } catch {
    /* ignore */
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
  loading.value = false
  currentId.value = ''
  progress.value = 0
}

export function useFullPlayer() {
  return { currentId, playing, loading, progress, duration, setQueue, toggle, seek, next, prev, stop }
}
