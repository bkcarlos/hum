import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { Candidate, RankResult, Song } from '@/types'
import { toCandidate } from '@/types'

export type ItemStatus = 'new' | 'kept'

export interface PlaylistItem {
  song: Song
  reason: string
  status: ItemStatus // 'kept' = was shown last round and survived; 'new' = newly added
}

// Right column state (F6/F7) + the F10 state-sync rule:
//   a new round OVERRIDES the list, but KEEPS selections that still appear and
//   NOTICES the user about any selected song that dropped out.
export const usePlaylistStore = defineStore('playlist', () => {
  const pool = ref<Map<string, Song>>(new Map()) // id -> Song for the current candidate pool
  const items = ref<PlaylistItem[]>([]) // ordered display list
  const selected = ref<Set<string>>(new Set())
  const playlistName = ref('')
  const description = ref('')
  const notice = ref('') // F10 dropped-selection notice

  const hasResult = computed(() => items.value.length > 0)

  // Pool fed back into /rank during refinement — carries year/lyrics/rating so
  // F10 tweaks like "去掉有歌词的" act on real attributes (shared mapping).
  const candidatesForRank = computed<Candidate[]>(() => [...pool.value.values()].map(toCandidate))

  const selectedSongs = computed<Song[]>(() =>
    items.value.filter((it) => selected.value.has(it.song.id)).map((it) => it.song),
  )
  const selectedCount = computed(() => selected.value.size)

  /** First recommendation for a fresh pool — overwrites everything. */
  function setRecommendation(songs: Song[], rank: RankResult) {
    pool.value = new Map(songs.map((s) => [s.id, s]))
    applyRank(rank, false)
  }

  /** F10 refinement. Pass new songs to replace the pool (re-search), or omit to
   *  re-rank the existing pool. Either way the list is overridden but surviving
   *  selections are kept. */
  function applyRefinement(rank: RankResult, songs?: Song[]) {
    if (songs && songs.length) pool.value = new Map(songs.map((s) => [s.id, s]))
    applyRank(rank, true)
  }

  function applyRank(rank: RankResult, isRefinement: boolean) {
    const prevIds = new Set(items.value.map((it) => it.song.id))
    const next: PlaylistItem[] = []
    for (const r of rank.songs) {
      const song = pool.value.get(r.id)
      if (!song) continue // belt-and-suspenders: never show an id not in the pool
      next.push({
        song,
        reason: r.reason,
        status: isRefinement && prevIds.has(r.id) ? 'kept' : 'new',
      })
    }
    items.value = next
    playlistName.value = rank.playlist_name
    description.value = rank.description

    // Keep selections that survive; notice the user about dropped ones.
    const presentIds = new Set(next.map((it) => it.song.id))
    const dropped = [...selected.value].filter((id) => !presentIds.has(id))
    selected.value = new Set([...selected.value].filter((id) => presentIds.has(id)))
    notice.value = dropped.length
      ? `有 ${dropped.length} 首已勾选的歌曲不在本轮推荐中，已暂时移出（可重新描述找回）。`
      : ''
  }

  function toggle(id: string) {
    const s = new Set(selected.value)
    if (s.has(id)) s.delete(id)
    else s.add(id)
    selected.value = s
  }
  function selectAll() {
    selected.value = new Set(items.value.map((it) => it.song.id))
  }
  function clearSelection() {
    selected.value = new Set()
  }
  function dismissNotice() {
    notice.value = ''
  }
  function reset() {
    pool.value = new Map()
    items.value = []
    selected.value = new Set()
    playlistName.value = ''
    description.value = ''
    notice.value = ''
  }

  return {
    items,
    selected,
    playlistName,
    description,
    notice,
    hasResult,
    candidatesForRank,
    selectedSongs,
    selectedCount,
    setRecommendation,
    applyRefinement,
    toggle,
    selectAll,
    clearSelection,
    dismissNotice,
    reset,
  }
})
