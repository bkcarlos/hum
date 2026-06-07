import axios, { AxiosError } from 'axios'
import type {
  AdminUsage,
  ApiError,
  Candidate,
  Intent,
  LlmBody,
  ModelInfo,
  QuotaConfig,
  RankResult,
  Song,
  SuggestResult,
} from '@/types'

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

/** Auth for the metered endpoints (suggest/rank): either the user's BYOK key
 *  (unlimited) or a free-tier Sign in with Apple session (quota'd). */
export type LlmAuth = { apiKey: string } | { session: string }

function authConfig(auth: LlmAuth) {
  return 'session' in auth
    ? { headers: { Authorization: `Bearer ${auth.session}` } }
    : { headers: { 'X-LLM-Api-Key': auth.apiKey } }
}

// ── BYOK / LLM ────────────────────────────────────────────────────────
export function testLlm(llm: LlmBody, apiKey: string): Promise<{ ok: boolean }> {
  return http.post('/llm/test', { llm }, keyHeader(apiKey))
}

/** Best-effort: fetch the provider's available models for the current key +
 *  Base URL. Not every OpenAI-compatible gateway supports it — callers fall
 *  back to manual entry on error. The `model` field in `llm` is ignored here. */
export function listModels(llm: LlmBody, apiKey: string): Promise<{ models: ModelInfo[] }> {
  return http.post('/llm/models', { llm }, keyHeader(apiKey))
}

export function parseIntent(llm: LlmBody, apiKey: string, text: string, seedArtists: string[]): Promise<Intent> {
  return http.post('/intent', { llm, text, seedArtists }, keyHeader(apiKey))
}

/** Option A: the LLM proposes real songs for the request; the backend resolves
 *  each against the user's storefront so only tracks that exist reach us. */
export function suggest(
  llm: LlmBody,
  auth: LlmAuth,
  storefront: string,
  text: string,
  seedArtists: string[],
): Promise<SuggestResult> {
  return http.post('/suggest', { llm, storefront, text, seedArtists }, authConfig(auth))
}

/** Generate personalized empty-state example prompts ("千人千面") from local
 *  context + recent tastes. Inspiration text only — no songs named. */
export function genExamples(
  llm: LlmBody,
  apiKey: string,
  context: string,
  tastes: string[],
  count: number,
): Promise<{ examples: string[] }> {
  return http.post('/examples', { llm, context, tastes, count }, keyHeader(apiKey))
}

export function rankSongs(
  llm: LlmBody,
  auth: LlmAuth,
  intent: Intent,
  candidates: Candidate[],
  instruction = '',
): Promise<RankResult> {
  return http.post('/rank', { llm, intent, candidates, instruction }, authConfig(auth))
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

// ── Sign in with Apple (web) ──────────────────────────────────────────
/** Non-secret config for the browser Apple-JS flow. enabled:false ⇒ no Services
 *  ID configured server-side, so the UI falls back to pasting a session token. */
export function getAppleWebConfig(): Promise<{
  enabled: boolean
  clientId?: string
  redirectUri?: string
  scope?: string
}> {
  return http.get('/auth/apple/web')
}

/** Exchange an Apple identity token (from the web Apple-JS popup) for our own
 *  session token. Same endpoint the iOS app uses. */
export function exchangeAppleToken(identityToken: string): Promise<{ session: string; expiresInSeconds: number }> {
  return http.post('/auth/apple', { identityToken })
}

/** The signed-in user's Apple sub (+ admin flag) for any valid session — used to
 *  show the account in the free-tier UI. */
export function getMe(session: string): Promise<{ sub: string; email: string; isAdmin: boolean }> {
  return http.get('/auth/me', { headers: { Authorization: `Bearer ${session}` } })
}

// ── Free-tier admin (Sign in with Apple session; C2/C3/C4) ────────────
/** The admin session Bearer (issued by /auth/apple). Like the BYOK key it is a
 *  credential — it lives only in the admin store's localStorage, never elsewhere. */
function bearer(session: string) {
  return { headers: { Authorization: `Bearer ${session}` } }
}

/** Who am I: the caller's Apple sub + whether they're an admin. Any valid session. */
export function adminMe(session: string): Promise<{ sub: string; email: string; isAdmin: boolean }> {
  return http.get('/admin/me', bearer(session))
}

export function getAdminConfig(session: string): Promise<QuotaConfig> {
  return http.get('/admin/config', bearer(session))
}

/** Partial update: only the provided fields change (omitted ones — notably
 *  `admins` — are preserved server-side). */
export function updateAdminConfig(session: string, patch: Partial<QuotaConfig>): Promise<QuotaConfig> {
  return http.post('/admin/config', patch, bearer(session))
}

export function getAdminUsage(session: string, day?: string): Promise<AdminUsage> {
  return http.get('/admin/usage', { ...bearer(session), params: day ? { day } : undefined })
}

export function setUserBan(session: string, sub: string, banned: boolean): Promise<{ sub: string; banned: boolean }> {
  return http.post(`/admin/users/${encodeURIComponent(sub)}/${banned ? 'ban' : 'unban'}`, null, bearer(session))
}
