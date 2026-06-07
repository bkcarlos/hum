// Shared types mirroring the backend data contracts (docs/requirements.md §5.5).

export type ProviderType = 'openai-compat' | 'anthropic' | 'gemini'

/** Non-secret LLM config sent in request bodies. The API key travels separately
 *  in the X-LLM-Api-Key header and is never put in a body. */
export interface LlmBody {
  provider: ProviderType
  baseUrl: string
  model: string
}

/** One selectable model returned by /llm/models — the provider's live catalog.
 *  The config UI merges these with the static presets (and still allows typing
 *  a model name by hand for gateways that don't support listing). */
export interface ModelInfo {
  id: string
  displayName?: string
}

/** Structured intent parsed from natural language (F3). */
export interface Intent {
  moods: string[]
  genres: string[]
  instruments: string[]
  tempo: string
  keywords: string[]
  seed_artists: string[]
}

export function emptyIntent(): Intent {
  return { moods: [], genres: [], instruments: [], tempo: '', keywords: [], seed_artists: [] }
}

/** A real Apple Music track from catalog search (F4). */
export interface Song {
  id: string
  title: string
  artist: string
  album: string
  genres: string[]
  durationMs: number
  artworkUrl: string
  previewUrl: string
  releaseDate?: string // "1959-08-17" or "1959"
  contentRating?: string // "clean" | "explicit"
  hasLyrics?: boolean // false ⇒ likely instrumental
  isrc?: string
  composer?: string
}

/** Candidate shape sent to /rank (subset of Song + ranking-relevant attributes). */
export interface Candidate {
  id: string
  title: string
  artist: string
  album?: string
  genres?: string[]
  year?: string
  hasLyrics?: boolean
  contentRating?: string
}

/** Map a catalog Song to the Candidate sent to /rank, carrying the ranking-
 *  relevant attributes (year/lyrics/rating) so refinements like "去掉有歌词的"
 *  and "不要露骨的" act on real data. Shared by the first rank and F10 reranks. */
export function toCandidate(s: Song): Candidate {
  return {
    id: s.id,
    title: s.title,
    artist: s.artist,
    album: s.album,
    genres: s.genres,
    year: s.releaseDate ? s.releaseDate.slice(0, 4) : undefined,
    hasLyrics: s.hasLyrics,
    contentRating: s.contentRating,
  }
}

export interface RankedSong {
  id: string
  reason: string
}

/** LLM ranking output (F5). */
export interface RankResult {
  playlist_name: string
  description: string
  songs: RankedSong[]
}

/** One LLM-proposed track to resolve against Apple Music (Option A). */
export interface SongSuggestion {
  title: string
  artist: string
}

/** Result of /suggest (Option A): a grounded candidate pool (every track verified
 *  to exist on Apple Music) plus the intent for display and resolution stats. */
export interface SuggestResult {
  storefront: string
  intent: Intent
  candidates: Song[]
  suggested: number
  resolved: number
  unresolved: string[]
}

/** Normalized API error surfaced to the UI. */
export interface ApiError {
  code: string
  message: string
}

// ── Free-tier admin (C2/C3/C4) ──────────────────────────────────────────

/** Live free-tier policy + admin allowlist (backend quota.Config). The server's
 *  own LLM key is NOT part of this — it's a server secret, never sent to clients. */
export interface QuotaConfig {
  enabled: boolean
  perUserDailyLimit: number
  globalDailyLimit: number
  llmProvider: ProviderType
  llmBaseUrl: string
  llmModel: string
  admins: string[]
}

/** One user's metered usage for a day + ban state (admin usage view). */
export interface UserUsage {
  sub: string
  email?: string // Apple email when known (captured at login)
  used: number
  banned: boolean
}

/** GET /admin/usage response: the day's totals + per-user breakdown. */
export interface AdminUsage {
  day: string
  globalUsed: number
  globalLimit: number
  perUserLimit: number
  users: UserUsage[]
}

export type ChatRole = 'user' | 'assistant'

export interface ChatMessage {
  id: number
  role: ChatRole
  text: string
}
