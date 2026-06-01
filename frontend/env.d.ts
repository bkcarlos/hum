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
  }

  interface Window {
    MusicKit: MusicKitGlobal
  }
  const MusicKit: MusicKitGlobal
}

export {}
