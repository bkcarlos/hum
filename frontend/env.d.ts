/// <reference types="vite/client" />

// Minimal shim so plain tsc/editors understand single-file components.
declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<Record<string, unknown>, Record<string, unknown>, unknown>
  export default component
}

// MusicKit JS attaches a global `MusicKit` once its script loads. We keep it
// loosely typed (the SDK has no first-party npm types for v3). The interfaces
// live inside `declare global` so they are visible everywhere, not module-local.
declare global {
  interface MusicKitGlobal {
    configure(opts: unknown): Promise<MusicKitInstance>
    getInstance(): MusicKitInstance
  }

  interface MusicKitInstance {
    authorize(): Promise<string> // resolves to the Music User Token
    unauthorize(): Promise<void>
    readonly storefrontId: string
    readonly isAuthorized: boolean

    // Full (non-preview) playback for subscribers (F6). Loosely typed — the v3
    // SDK ships no first-party types; we only declare what we call.
    setQueue(opts: { songs?: string[]; song?: string; startPlaying?: boolean; startPosition?: number }): Promise<unknown>
    play(): Promise<unknown>
    pause(): void
    stop(): void
    skipToNextItem(): Promise<unknown>
    skipToPreviousItem(): Promise<unknown>
    changeToMediaAtIndex(index: number): Promise<unknown>
    readonly playbackState: number // MusicKit.PlaybackStates (2 = playing, 3 = paused)
    readonly nowPlayingItem: { id: string } | null
    addEventListener(name: string, handler: (event: unknown) => void): void
    removeEventListener(name: string, handler: (event: unknown) => void): void
  }

  interface Window {
    MusicKit: MusicKitGlobal
  }
  const MusicKit: MusicKitGlobal
}

export {}
