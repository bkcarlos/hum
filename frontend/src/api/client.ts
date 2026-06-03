import axios, { AxiosError } from 'axios'
import type { ApiError, Candidate, Intent, LlmBody, RankResult, Song, SuggestResult } from '@/types'

const http = axios.create({
  baseURL: import.meta.env.VITE_API_BASE ?? '/api',
  timeout: 60_000,
})

// Unwrap the {data} envelope on success; normalize {error:{code,message}} on failure.
http.interceptors.response.use(
  (res) => res.data?.data,
  (err: AxiosError<{ error?: ApiError }>) => {
    const apiErr: ApiError = err.response?.data?.error ?? {
      code: 'network',
      message: err.message || '网络错误，请稍后重试。',
    }
    return Promise.reject(apiErr)
  },
)

/** Header carrying the BYOK secret. Set per request, never stored client-side
 *  beyond the user's own localStorage (managed by the llmConfig store). */
function keyHeader(apiKey: string) {
  return { headers: { 'X-LLM-Api-Key': apiKey } }
}

// ── BYOK / LLM ────────────────────────────────────────────────────────
export function testLlm(llm: LlmBody, apiKey: string): Promise<{ ok: boolean }> {
  return http.post('/llm/test', { llm }, keyHeader(apiKey))
}

export function parseIntent(llm: LlmBody, apiKey: string, text: string, seedArtists: string[]): Promise<Intent> {
  return http.post('/intent', { llm, text, seedArtists }, keyHeader(apiKey))
}

/** Option A: the LLM proposes real songs for the request; the backend resolves
 *  each against the user's storefront so only tracks that exist reach us. */
export function suggest(
  llm: LlmBody,
  apiKey: string,
  storefront: string,
  text: string,
  seedArtists: string[],
): Promise<SuggestResult> {
  return http.post('/suggest', { llm, storefront, text, seedArtists }, keyHeader(apiKey))
}

export function rankSongs(
  llm: LlmBody,
  apiKey: string,
  intent: Intent,
  candidates: Candidate[],
  instruction = '',
): Promise<RankResult> {
  return http.post('/rank', { llm, intent, candidates, instruction }, keyHeader(apiKey))
}

// ── Apple Music ───────────────────────────────────────────────────────
export function getDeveloperToken(): Promise<{ token: string; expiresAt: string }> {
  return http.get('/apple/developer-token')
}

export function searchCandidates(storefront: string, intent: Intent): Promise<{ storefront: string; candidates: Song[] }> {
  return http.post('/apple/search', { storefront, intent })
}

export function createPlaylist(
  userToken: string,
  name: string,
  description: string,
  songIds: string[],
): Promise<{ id: string; name: string; url: string }> {
  return http.post('/apple/playlists', { name, description, songIds }, { headers: { 'Music-User-Token': userToken } })
}
