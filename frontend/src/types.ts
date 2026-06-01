// Shared types mirroring the backend data contracts (docs/requirements.md §5.5).

export type ProviderType = 'openai-compat' | 'anthropic' | 'gemini'

/** Non-secret LLM config sent in request bodies. The API key travels separately
 *  in the X-LLM-Api-Key header and is never put in a body. */
export interface LlmBody {
  provider: ProviderType
  baseUrl: string
  model: string
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
}

/** Candidate shape sent to /rank (subset of Song). */
export interface Candidate {
  id: string
  title: string
  artist: string
  album?: string
  genres?: string[]
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

/** Normalized API error surfaced to the UI. */
export interface ApiError {
  code: string
  message: string
}

export type ChatRole = 'user' | 'assistant'

export interface ChatMessage {
  id: number
  role: ChatRole
  text: string
}
