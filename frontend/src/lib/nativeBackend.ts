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
import type { Backend } from './backend'

/**
 * The shape Wails injects into the webview for the `main.App` struct. Wails
 * generates a JS module for this too, but going through one explicit,
 * hand-written declaration keeps the frontend independent of generated code
 * being regenerated (and of its naming quirks).
 */
interface NativeAppBindings {
  AppInfo(): Promise<AppInfo>
  Status(): Promise<Status>
  CreateVault(masterPassword: string, hint: string): Promise<Status>
  Unlock(masterPassword: string): Promise<Status>
  Lock(): Promise<Status>
  ChangeMasterPassword(current: string, next: string): Promise<Status>
  SetHint(hint: string): Promise<Status>
  ListItems(options: ListOptions): Promise<Summary[]>
  GetItem(id: string): Promise<Item>
  SaveItem(item: Item): Promise<Item>
  DeleteItem(id: string): Promise<void>
  ToggleFavorite(id: string): Promise<Item>
  Tags(): Promise<TagCount[]>
  Stats(): Promise<Stats>
  TrashList(): Promise<Summary[]>
  RestoreItem(id: string): Promise<Item>
  DeleteForever(id: string): Promise<void>
  EmptyTrash(): Promise<number>
  Audit(): Promise<AuditReport>
  ImportCSV(text: string, options: ImportOptions): Promise<ImportResult>
  GetSettings(): Promise<Settings>
  SaveSettings(settings: Settings): Promise<Settings>
  GeneratePassword(options: GeneratorOptions): Promise<Generated>
  ScorePassword(password: string): Promise<Strength>
  TOTPCode(id: string): Promise<TOTPResult>
  CopySecret(value: string, clearAfterSeconds: number): Promise<void>
  ClearClipboard(): Promise<void>
  Ping(): Promise<AutoLockState>
  AutoLockState(): Promise<AutoLockState>
  ExportBackup(): Promise<string>
  ImportVault(): Promise<Status>
  RestoreBackup(): Promise<Status>
}

interface WailsRuntime {
  EventsOn(eventName: string, callback: (data: unknown) => void): void
  EventsOff(eventName: string, ...additionalEventNames: string[]): void
}

declare global {
  interface Window {
    go?: { main?: { App?: NativeAppBindings } }
    runtime?: WailsRuntime
  }
}

export function nativeApp(): NativeAppBindings {
  const app = window.go?.main?.App
  if (!app) throw new Error('The Coffer backend is not available')
  return app
}

export function hasNativeBackend(): boolean {
  return typeof window !== 'undefined' && Boolean(window.go?.main?.App)
}

export class NativeBackend implements Backend {
  readonly kind = 'native' as const
  readonly nativeDialogs = true

  appInfo(): Promise<AppInfo> {
    return nativeApp().AppInfo()
  }

  status(): Promise<Status> {
    return nativeApp().Status()
  }

  createVault(masterPassword: string, hint: string): Promise<Status> {
    return nativeApp().CreateVault(masterPassword, hint)
  }

  unlock(masterPassword: string): Promise<Status> {
    return nativeApp().Unlock(masterPassword)
  }

  lock(): Promise<Status> {
    return nativeApp().Lock()
  }

  changeMasterPassword(current: string, next: string): Promise<Status> {
    return nativeApp().ChangeMasterPassword(current, next)
  }

  setHint(hint: string): Promise<Status> {
    return nativeApp().SetHint(hint)
  }

  listItems(options: ListOptions): Promise<Summary[]> {
    return nativeApp().ListItems(options)
  }

  getItem(id: string): Promise<Item> {
    return nativeApp().GetItem(id)
  }

  saveItem(item: Item): Promise<Item> {
    return nativeApp().SaveItem(item)
  }

  deleteItem(id: string): Promise<void> {
    return nativeApp().DeleteItem(id)
  }

  toggleFavorite(id: string): Promise<Item> {
    return nativeApp().ToggleFavorite(id)
  }

  tags(): Promise<TagCount[]> {
    return nativeApp().Tags()
  }

  stats(): Promise<Stats> {
    return nativeApp().Stats()
  }

  trashList(): Promise<Summary[]> {
    return nativeApp().TrashList()
  }

  restoreItem(id: string): Promise<Item> {
    return nativeApp().RestoreItem(id)
  }

  deleteForever(id: string): Promise<void> {
    return nativeApp().DeleteForever(id)
  }

  emptyTrash(): Promise<number> {
    return nativeApp().EmptyTrash()
  }

  audit(): Promise<AuditReport> {
    return nativeApp().Audit()
  }

  importCsv(text: string, options: ImportOptions): Promise<ImportResult> {
    return nativeApp().ImportCSV(text, options)
  }

  getSettings(): Promise<Settings> {
    return nativeApp().GetSettings()
  }

  saveSettings(settings: Settings): Promise<Settings> {
    return nativeApp().SaveSettings(settings)
  }

  generatePassword(options: GeneratorOptions): Promise<Generated> {
    return nativeApp().GeneratePassword(options)
  }

  scorePassword(password: string): Promise<Strength> {
    return nativeApp().ScorePassword(password)
  }

  totpCode(id: string): Promise<TOTPResult> {
    return nativeApp().TOTPCode(id)
  }

  copySecret(value: string, clearAfterSeconds: number): Promise<void> {
    return nativeApp().CopySecret(value, clearAfterSeconds)
  }

  clearClipboard(): Promise<void> {
    return nativeApp().ClearClipboard()
  }

  ping(): Promise<AutoLockState> {
    return nativeApp().Ping()
  }

  autoLockState(): Promise<AutoLockState> {
    return nativeApp().AutoLockState()
  }

  exportBackup(): Promise<string> {
    return nativeApp().ExportBackup()
  }

  importVault(): Promise<Status> {
    return nativeApp().ImportVault()
  }

  restoreBackup(): Promise<Status> {
    return nativeApp().RestoreBackup()
  }

  onLocked(handler: (reason: string) => void): () => void {
    const runtime = window.runtime
    if (!runtime) return () => undefined
    runtime.EventsOn('vault:locked', (data) => {
      handler(typeof data === 'string' ? data : 'locked')
    })
    return () => runtime.EventsOff('vault:locked')
  }
}
