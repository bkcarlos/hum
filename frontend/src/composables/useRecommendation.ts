import { ref } from 'vue'
import * as api from '@/api/client'
import { useLlmConfigStore } from '@/stores/llmConfig'
import { useAppleStore } from '@/stores/apple'
import { useConversationStore } from '@/stores/conversation'
import { usePlaylistStore } from '@/stores/playlist'
import type { ApiError, Candidate, Song } from '@/types'

// What to re-run if the user hits "重试" after a failure. The core ops
// (runFull/rerank/researchCore) never touch the transcript, so a retry repeats
// the work without duplicating chat turns.
type LastAction =
  | { kind: 'full'; text: string; seeds: string[] }
  | { kind: 'rerank'; instruction: string }
  | { kind: 'research' }

// Orchestrates the F2→F5 pipeline (+ F10 refinement + F3 re-search), keeping the
// left conversation store and right playlist store in sync. ConversationPane is
// the single consumer, so the local reactive state below is effectively a unit.
export function useRecommendation() {
  const llm = useLlmConfigStore()
  const apple = useAppleStore()
  const convo = useConversationStore()
  const playlist = usePlaylistStore()

  const loading = ref(false)
  const stage = ref('')
  const lastError = ref('')
  let lastAction: LastAction | null = null

  function precondition(): string | null {
    if (!llm.configured) return '请先在右上角「LLM 设置」里配置并测试你的 API Key（F0）。'
    if (!apple.authorized || !apple.storefront) return '请先点击「连接 Apple Music」完成授权（F1）。'
    return null
  }

  function fail(e: unknown) {
    const msg = (e as ApiError)?.message ?? '未知错误'
    lastError.value = msg
    convo.addAssistant('出错了：' + msg)
  }

  // Full pipeline: parse intent → search → rank. No transcript write (callers
  // own the chat flow), so it doubles as the retry unit.
  async function runFull(text: string, seeds: string[]) {
    const blocked = precondition()
    if (blocked) {
      convo.addAssistant(blocked)
      return
    }
    loading.value = true
    lastError.value = ''
    try {
      stage.value = '解析意图…'
      const intent = await api.parseIntent(llm.body, llm.apiKey, text, seeds)
      convo.setIntent(intent)

      stage.value = '检索候选…'
      const { candidates } = await api.searchCandidates(apple.storefront, intent)

      stage.value = '智能排序…'
      const rank = await api.rankSongs(llm.body, llm.apiKey, intent, toCandidates(candidates))
      playlist.setRecommendation(candidates, rank)

      convo.addAssistant(
        `为你挑了 ${rank.songs.length} 首：「${rank.playlist_name}」。右侧可试听、勾选；也可以继续告诉我怎么调整（如“去掉有歌词的”“再慢一点”），或编辑左侧条件后重搜。`,
      )
    } catch (e) {
      fail(e)
    } finally {
      loading.value = false
      stage.value = ''
    }
  }

  // F10 refinement: re-rank the existing candidate pool with a natural-language tweak.
  async function rerank(instruction: string) {
    const blocked = precondition()
    if (blocked) {
      convo.addAssistant(blocked)
      return
    }
    loading.value = true
    lastError.value = ''
    try {
      stage.value = '重新挑选…'
      const rank = await api.rankSongs(llm.body, llm.apiKey, convo.intent, playlist.candidatesForRank, instruction)
      playlist.applyRefinement(rank)
      const tail = playlist.notice ? ` ${playlist.notice}` : ''
      convo.addAssistant(`已按「${instruction}」重新挑选：「${rank.playlist_name}」。${tail}`)
    } catch (e) {
      fail(e)
    } finally {
      loading.value = false
      stage.value = ''
    }
  }

  // F3: re-run search + rank from the CURRENT (user-edited) intent chips, then
  // override the list while keeping surviving selections.
  async function researchCore() {
    const blocked = precondition()
    if (blocked) {
      convo.addAssistant(blocked)
      return
    }
    if (!convo.hasIntent) {
      convo.addAssistant('还没有可用的检索条件，请先描述一下。')
      return
    }
    loading.value = true
    lastError.value = ''
    try {
      stage.value = '按条件检索…'
      const { candidates } = await api.searchCandidates(apple.storefront, convo.intent)

      stage.value = '智能排序…'
      const rank = await api.rankSongs(llm.body, llm.apiKey, convo.intent, toCandidates(candidates))
      playlist.applyRefinement(rank, candidates) // replace pool, keep selected

      const tail = playlist.notice ? ` ${playlist.notice}` : ''
      convo.addAssistant(`已按调整后的条件重新检索：「${rank.playlist_name}」，共 ${rank.songs.length} 首。${tail}`)
    } catch (e) {
      fail(e)
    } finally {
      loading.value = false
      stage.value = ''
    }
  }

  async function submit(text: string, seeds: string[] = []) {
    convo.addUser(text)
    lastAction = { kind: 'full', text, seeds }
    await runFull(text, seeds)
  }

  async function refine(instruction: string) {
    convo.addUser(instruction)
    if (!playlist.hasResult) {
      lastAction = { kind: 'full', text: instruction, seeds: [] }
      await runFull(instruction, [])
      return
    }
    lastAction = { kind: 'rerank', instruction }
    await rerank(instruction)
  }

  async function researchFromIntent() {
    lastAction = { kind: 'research' }
    await researchCore()
  }

  async function retry() {
    if (!lastAction || loading.value) return
    if (lastAction.kind === 'full') await runFull(lastAction.text, lastAction.seeds)
    else if (lastAction.kind === 'rerank') await rerank(lastAction.instruction)
    else await researchCore()
  }

  return { loading, stage, lastError, submit, refine, researchFromIntent, retry }
}

function toCandidates(songs: Song[]): Candidate[] {
  return songs.map((s) => ({ id: s.id, title: s.title, artist: s.artist, album: s.album, genres: s.genres }))
}
