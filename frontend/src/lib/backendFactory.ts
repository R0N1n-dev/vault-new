import type { Backend } from './backend'
import { DemoBackend } from './demoBackend'
import { NativeBackend, hasNativeBackend } from './nativeBackend'

let instance: Backend | null = null

/**
 * resolveBackend returns the Go-backed implementation when running inside the
 * Wails webview and the browser preview implementation otherwise. The choice is
 * made once and reused, because both implementations hold session state.
 */
export function resolveBackend(): Backend {
  if (instance) return instance
  instance = hasNativeBackend() ? new NativeBackend() : new DemoBackend()
  return instance
}

export function isPreview(): boolean {
  return resolveBackend().kind === 'demo'
}
