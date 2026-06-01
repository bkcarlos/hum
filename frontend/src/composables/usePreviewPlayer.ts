import { ref } from 'vue'
import type { Song } from '@/types'

// A single shared <audio> for 30s previews (F6). One track plays at a time;
// play/pause/skip satisfy the ToS playback-control requirement. Playback is
// always user-initiated — nothing here auto-plays on load.
const audio: HTMLAudioElement | null = typeof Audio !== 'undefined' ? new Audio() : null
const currentId = ref('')
const playing = ref(false)
let queue: Song[] = []

if (audio) {
  audio.addEventListener('ended', () => next())
  audio.addEventListener('play', () => (playing.value = true))
  audio.addEventListener('pause', () => (playing.value = false))
}

function setQueue(songs: Song[]) {
  queue = songs
}

/** Toggle a song: start it, or pause it if it is the one currently playing. */
function toggle(song: Song) {
  if (!audio || !song.previewUrl) return
  if (currentId.value === song.id && playing.value) {
    audio.pause()
    return
  }
  if (currentId.value !== song.id) {
    audio.src = song.previewUrl
    currentId.value = song.id
  }
  void audio.play().catch(() => (playing.value = false))
}

function stop() {
  if (audio) {
    audio.pause()
    audio.currentTime = 0
  }
  currentId.value = ''
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
  return { currentId, playing, setQueue, toggle, stop, next, prev }
}
