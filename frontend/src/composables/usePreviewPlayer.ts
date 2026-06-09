import { ref } from 'vue'
import type { Song } from '@/types'

// A single shared <audio> for 30s previews (F6). One track plays at a time;
// play/pause/skip satisfy the ToS playback-control requirement. Playback is
// always user-initiated — nothing here auto-plays on load.
const audio: HTMLAudioElement | null = typeof Audio !== 'undefined' ? new Audio() : null
const currentId = ref('')
const playing = ref(false)
const loading = ref(false) // 点击后到出声之间 / 缓冲中（非下载文件，纯加载态）
const progress = ref(0) // 当前播放位置（秒）
const duration = ref(0) // 预览时长（秒，约 30）
let queue: Song[] = []

if (audio) {
  audio.addEventListener('ended', () => next())
  audio.addEventListener('playing', () => {
    playing.value = true
    loading.value = false
  })
  audio.addEventListener('pause', () => (playing.value = false))
  audio.addEventListener('waiting', () => (loading.value = true))
  audio.addEventListener('timeupdate', () => (progress.value = audio.currentTime))
  audio.addEventListener('loadedmetadata', () => (duration.value = audio.duration || 0))
  audio.addEventListener('durationchange', () => (duration.value = audio.duration || 0))
}

function setQueue(songs: Song[]) {
  queue = songs
}

/** Toggle a song: start it, or pause/resume if it is the one currently loaded. */
function toggle(song: Song) {
  if (!audio || !song.previewUrl) return
  if (currentId.value === song.id) {
    if (playing.value) audio.pause()
    else void audio.play().catch(() => (playing.value = false))
    return
  }
  audio.src = song.previewUrl
  currentId.value = song.id
  progress.value = 0
  duration.value = 0
  loading.value = true
  void audio.play().catch(() => {
    playing.value = false
    loading.value = false
  })
}

/** Seek the current preview to t seconds (drag-to-seek on the progress bar). */
function seek(t: number) {
  if (!audio) return
  audio.currentTime = Math.max(0, t)
  progress.value = audio.currentTime
}

function stop() {
  if (audio) {
    audio.pause()
    audio.currentTime = 0
  }
  currentId.value = ''
  loading.value = false
  progress.value = 0
}

function currentIndex() {
  return queue.findIndex((s) => s.id === currentId.value)
}

function next() {
  const n = queue[currentIndex() + 1]
  if (n) toggle(n)
  else stop()
}

function prev() {
  const i = currentIndex()
  if (i > 0) toggle(queue[i - 1])
}

export function usePreviewPlayer() {
  return { currentId, playing, loading, progress, duration, setQueue, toggle, seek, stop, next, prev }
}
