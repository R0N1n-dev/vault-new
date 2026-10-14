/**
 * Clipboard helpers layered on the backend so the clear-after-delay behaviour
 * is identical in the packaged app (Go clears it) and the browser preview.
 */
import { resolveBackend } from './backendFactory'
import type { AutoLockState } from '../types'

export interface CopyOutcome {
  label: string
  detail: string
  clearAfterSeconds: number
}

export async function copyWithClear(
  value: string,
  label: string,
  clearAfterSeconds: number,
): Promise<CopyOutcome> {
  if (!value) throw new Error('There is nothing to copy')
  await resolveBackend().copySecret(value, clearAfterSeconds)
  const detail =
    clearAfterSeconds > 0 ? `Clipboard clears in ${clearAfterSeconds}s` : 'Clipboard will not be cleared automatically'
  return { label, detail, clearAfterSeconds }
}

export async function lockState(): Promise<AutoLockState> {
  return resolveBackend().autoLockState()
}
