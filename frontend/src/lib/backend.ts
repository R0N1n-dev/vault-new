import type {
  AppInfo,
  AuditReport,
  AutoLockState,
  Generated,
  GeneratorOptions,
  ImportOptions,
  ImportResult,
  Item,
  ListOptions,
  Settings,
  Status,
  Stats,
  Strength,
  Summary,
  TOTPResult,
  TagCount,
} from '../types'

/**
 * Backend is the single seam between the UI and the vault implementation.
 *
 * In a packaged app it is backed by the Go vault over Wails bindings. When the
 * frontend is served in a plain browser (vite preview, design work, this
 * repo's demo build) it is backed by an in-browser implementation that uses
 * WebCrypto and localStorage. The UI code cannot tell the difference apart
 * from `kind`.
 */
export interface Backend {
  readonly kind: 'native' | 'demo'
  /** Whether the host can show native open/save dialogs. */
  readonly nativeDialogs: boolean

  appInfo(): Promise<AppInfo>
  status(): Promise<Status>
  createVault(masterPassword: string, hint: string): Promise<Status>
  unlock(masterPassword: string): Promise<Status>
  lock(): Promise<Status>
  changeMasterPassword(current: string, next: string): Promise<Status>
  setHint(hint: string): Promise<Status>

  listItems(options: ListOptions): Promise<Summary[]>
  getItem(id: string): Promise<Item>
  saveItem(item: Item): Promise<Item>
  /** Moves an item to the trash. */
  deleteItem(id: string): Promise<void>
  toggleFavorite(id: string): Promise<Item>
  tags(): Promise<TagCount[]>
  stats(): Promise<Stats>

  trashList(): Promise<Summary[]>
  restoreItem(id: string): Promise<Item>
  deleteForever(id: string): Promise<void>
  emptyTrash(): Promise<number>

  /** Offline health check: weak, reused and stale passwords, missing 2FA. */
  audit(): Promise<AuditReport>

  /** Merges a CSV export; with dryRun it only classifies the rows. */
  importCsv(text: string, options: ImportOptions): Promise<ImportResult>

  getSettings(): Promise<Settings>
  saveSettings(settings: Settings): Promise<Settings>

  generatePassword(options: GeneratorOptions): Promise<Generated>
  scorePassword(password: string): Promise<Strength>
  totpCode(id: string): Promise<TOTPResult>

  copySecret(value: string, clearAfterSeconds: number): Promise<void>
  clearClipboard(): Promise<void>
  ping(): Promise<AutoLockState>
  autoLockState(): Promise<AutoLockState>

  exportBackup(): Promise<string>
  importVault(): Promise<Status>
  restoreBackup(): Promise<Status>

  /** Subscribes to vault-locked events; returns an unsubscribe function. */
  onLocked(handler: (reason: string) => void): () => void

  /** Only the browser preview implements this: forget the local preview vault. */
  resetPreview?(): Promise<void>
}

/** message extracts a readable string from anything a backend may throw. */
export function errorMessage(error: unknown): string {
  if (typeof error === 'string') return error
  if (error instanceof Error) return error.message
  if (error && typeof error === 'object') {
    const record = error as Record<string, unknown>
    if (typeof record.message === 'string') return record.message
  }
  return 'Something went wrong'
}

/**
 * Human-friendly text for the errors the vault package returns. The Go layer
 * keeps messages deliberately vague about whether a password was wrong or the
 * file was modified, and the UI preserves that property.
 */
export function friendlyError(error: unknown): string {
  const raw = errorMessage(error)
  const lower = raw.toLowerCase()
  if (lower.includes('invalid master password')) {
    return 'That master password does not unlock this vault.'
  }
  if (lower.includes('vault is locked')) {
    return 'The vault locked itself. Unlock it to continue.'
  }
  if (lower.includes('a vault file already exists')) {
    return 'A vault already exists at this location.'
  }
  if (lower.includes('authentication failed') || lower.includes('failed authentication')) {
    return 'The vault file failed its integrity check. It may have been modified.'
  }
  if (lower.includes('item not found')) {
    return 'That item no longer exists.'
  }
  if (lower.includes('not a coffer vault')) {
    return 'That file is not a Coffer vault.'
  }
  return raw
}
