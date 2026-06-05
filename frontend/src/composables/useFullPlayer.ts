import { ref } from 'vue'
import { onFullPlaybackChange, pauseFull, playFullTracks, resumeFull } from '@/services/musickit'

// Full-track playback for Apple Music subscribers (F6). Mirrors usePreviewPlayer's
// shape so the UI can offer 完整播放 alongside the 30s preview controls. Playback
// is always user-initiated; a non-subscriber's play() rejects and we reset state.
const playingFull = ref(false)
let queueIds: string[] = []
let started = false
let unsubscribe: (() => void) | null = null

async function ensureListener() {
  if (unsubscribe) return
  unsubscribe = await onFullPlaybackChange((p) => (playingFull.value = p))
}

/** Update the full-playback queue to the current displayed order. */
function setFullQueue(ids: string[]) {
  queueIds = ids
}

/** Toggle full playback: pause if playing, resume if started, else start the queue. */
async function playPauseFull() {
  try {
    await ensureListener()
    if (playingFull.value) {
      await pauseFull()
    } else if (started) {
      await resumeFull()
    } else if (queueIds.length) {
      await playFullTracks(queueIds)
      started = true
    }
  } catch {
    playingFull.value = false // non-subscriber / error → caller still has previews
  }
}

export function useFullPlayer() {
  return { playingFull, setFullQueue, playPauseFull }
}
