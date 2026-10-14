/**
 * The session store.
 *
 * A module-level reactive singleton: the app has exactly one vault session, so
 * provide/inject plumbing would only add indirection. Components import the
 * pieces they need directly.
 */

import { computed, ref } from 'vue'
import { friendlyError } from '../lib/backend'
import { resolveBackend } from '../lib/backendFactory'
import { copyWithClear } from '../lib/clipboard'
import { applyTheme, readStoredTheme, storeTheme, watchSystemTheme } from '../lib/theme'
import type {
  AppInfo,
  AuditReport,
  AutoLockState,
  ImportMode,
  ImportResult,
  Item,
  ItemType,
  ListOptions,
  Settings,
  Status,
  Stats,
  Summary,
  TagCount,
  Theme,
} from '../types'
import { defaultListOptions, defaultSettings, normalizeItem } from '../types'
import { pushToast, toastError, toastSuccess } from './toasts'

const backend = resolveBackend()

export const backendKind = backend.kind
export const nativeDialogs = backend.nativeDialogs

// --------------------------------------------------------------------- state --

export const loading = ref(true)
export const busy = ref(false)
export const status = ref<Status | null>(null)
export const appInfo = ref<AppInfo | null>(null)
export const settings = ref<Settings | null>(null)
export const items = ref<Summary[]>([])
export const tags = ref<TagCount[]>([])
export const stats = ref<Stats | null>(null)
export const detail = ref<Item | null>(null)
export const detailLoading = ref(false)
export const filters = ref<ListOptions>(defaultListOptions())
export const autoLock = ref<AutoLockState>({ enabled: false, timeoutSeconds: 0, secondsRemaining: 0 })
export const theme = ref<Theme>(readStoredTheme())

/** Item currently open in the editor sheet, if any. */
export const editing = ref<Item | null>(null)
export const settingsOpen = ref(false)
export const generatorOpen = ref(false)
/** Item queued for deletion confirmation. */
export const pendingDelete = ref<DeletableItem | null>(null)
/** Trashed item queued for permanent deletion. */
export const pendingPurge = ref<DeletableItem | null>(null)

export const trash = ref<Summary[]>([])
export const trashLoading = ref(false)
export const audit = ref<AuditReport | null>(null)
export const auditLoading = ref(false)
export const importOpen = ref(false)
export const importBusy = ref(false)
/** Source text of the CSV being imported; held in memory only. */
export const importSource = ref('')
export const importPreview = ref<ImportResult | null>(null)

/**
 * view selects which workspace the sidebar is showing. The item list, the
 * trash and the security report all reuse the same three panes, so this is a
 * single switch rather than a router.
 */
export const view = ref<'items' | 'trash' | 'audit'>('items')

/** The minimum an item needs to be offered for deletion. */
export type DeletableItem = { id: string; title: string }

// ------------------------------------------------------------------ derived --

export const exists = computed(() => status.value?.exists ?? false)
export const unlocked = computed(() => status.value?.unlocked ?? false)
export const screen = computed<'setup' | 'unlock' | 'vault'>(() => {
  if (!exists.value) return 'setup'
  return unlocked.value ? 'vault' : 'unlock'
})
export const clipboardSeconds = computed(() => settings.value?.clipboardClearSeconds ?? 30)
export const effectiveSettings = computed<Settings>(() => settings.value ?? defaultSettings())
export const itemCount = computed(() => stats.value?.total ?? status.value?.itemCount ?? 0)

// ------------------------------------------------------------------ actions --

export async function bootstrap(): Promise<void> {
  loading.value = true
  try {
    appInfo.value = await backend.appInfo()
    await refreshStatus()
    if (unlocked.value) await loadWorkspace()
  } catch (error) {
    toastError(error, 'Could not read the vault')
  } finally {
    loading.value = false
  }
  installLockListener()
  startTicker()
  applyTheme(theme.value)
  watchSystemTheme(() => theme.value)
}

export async function refreshStatus(): Promise<void> {
  status.value = await backend.status()
}

async function loadWorkspace(): Promise<void> {
  await Promise.all([refreshItems(), refreshTags(), refreshStats(), refreshSettings()])
}

export async function createVault(masterPassword: string, hint: string): Promise<void> {
  busy.value = true
  try {
    status.value = await backend.createVault(masterPassword, hint)
    await loadWorkspace()
    toastSuccess('Vault created', 'Your master password is the only way in. Keep it safe.')
  } finally {
    busy.value = false
  }
}

export async function unlock(masterPassword: string): Promise<void> {
  busy.value = true
  try {
    status.value = await backend.unlock(masterPassword)
    await loadWorkspace()
  } finally {
    busy.value = false
  }
}

export async function lock(): Promise<void> {
  status.value = await backend.lock()
  clearWorkspace()
  pushToast({ kind: 'info', title: 'Vault locked' })
}

function clearWorkspace(): void {
  items.value = []
  tags.value = []
  stats.value = null
  detail.value = null
  editing.value = null
  settingsOpen.value = false
  generatorOpen.value = false
  pendingDelete.value = null
  pendingPurge.value = null
  trash.value = []
  audit.value = null
  closeImport()
  view.value = 'items'
}

/** switchView moves between the item list, the trash and the security report. */
export function switchView(next: 'items' | 'trash' | 'audit'): void {
  view.value = next
  detail.value = null
  if (next === 'trash') void refreshTrash()
  if (next === 'audit') void refreshAudit()
  if (next === 'items') void refreshItems()
}

// -------------------------------------------------------------------- trash --

export async function refreshTrash(): Promise<void> {
  trashLoading.value = true
  try {
    trash.value = await backend.trashList()
  } catch (error) {
    toastError(error, 'Could not read the trash')
  } finally {
    trashLoading.value = false
  }
}

export async function restoreItem(id: string): Promise<void> {
  try {
    const item = await backend.restoreItem(id)
    await Promise.all([refreshTrash(), refreshItems(), refreshStats(), refreshTags()])
    toastSuccess('Item restored', item.title)
  } catch (error) {
    toastError(error, 'Could not restore the item')
  }
}

export function requestPurge(item: DeletableItem): void {
  pendingPurge.value = { id: item.id, title: item.title }
}

export async function confirmPurge(): Promise<void> {
  const target = pendingPurge.value
  if (!target) return
  pendingPurge.value = null
  try {
    await backend.deleteForever(target.id)
    await Promise.all([refreshTrash(), refreshStats()])
    toastSuccess('Deleted permanently', target.title)
  } catch (error) {
    toastError(error, 'Could not delete the item')
  }
}

export async function emptyTrash(): Promise<void> {
  try {
    const removed = await backend.emptyTrash()
    await Promise.all([refreshTrash(), refreshStats()])
    toastSuccess(
      removed === 0 ? 'Nothing to empty' : 'Trash emptied',
      removed === 0 ? undefined : `${removed} ${removed === 1 ? 'item' : 'items'} removed for good.`,
    )
  } catch (error) {
    toastError(error, 'Could not empty the trash')
  }
}

// -------------------------------------------------------------------- audit --

export async function refreshAudit(): Promise<void> {
  auditLoading.value = true
  try {
    audit.value = await backend.audit()
  } catch (error) {
    toastError(error, 'Could not run the health check')
  } finally {
    auditLoading.value = false
  }
}

/** openFinding jumps from a security finding to the item it describes. */
export async function openFinding(itemId: string): Promise<void> {
  view.value = 'items'
  await refreshItems()
  await select(itemId)
}

// ------------------------------------------------------------- csv import --

export async function previewImport(text: string): Promise<void> {
  importBusy.value = true
  importSource.value = text
  try {
    importPreview.value = await backend.importCsv(text, { dryRun: true, mode: 'skip' })
  } catch (error) {
    importPreview.value = null
    importSource.value = ''
    toastError(error, 'Could not read that CSV file')
  } finally {
    importBusy.value = false
  }
}

export async function commitImport(mode: ImportMode): Promise<void> {
  if (!importSource.value) return
  importBusy.value = true
  try {
    const result = await backend.importCsv(importSource.value, { dryRun: false, mode })
    await Promise.all([refreshItems(), refreshTags(), refreshStats(), refreshAudit()])
    closeImport()
    toastSuccess(
      `${result.imported} ${result.imported === 1 ? 'item' : 'items'} imported`,
      result.duplicates > 0 ? `${result.duplicates} duplicates skipped.` : result.format,
    )
  } catch (error) {
    toastError(error, 'Import failed')
  } finally {
    importBusy.value = false
  }
}

export function openImport(): void {
  importOpen.value = true
  importPreview.value = null
  importSource.value = ''
}

/** closeImport drops the parsed CSV so the file text does not linger in memory. */
export function closeImport(): void {
  importOpen.value = false
  importPreview.value = null
  importSource.value = ''
  importBusy.value = false
}

export async function refreshItems(): Promise<void> {
  items.value = await backend.listItems(filters.value)
}

export async function refreshTags(): Promise<void> {
  tags.value = await backend.tags()
}

export async function refreshStats(): Promise<void> {
  stats.value = await backend.stats()
}

export async function refreshSettings(): Promise<void> {
  const loaded = await backend.getSettings()
  settings.value = loaded
  theme.value = loaded.theme ?? theme.value
  applyTheme(theme.value)
}

export function applyFilters(patch: Partial<ListOptions>): void {
  filters.value = { ...filters.value, ...patch }
  void refreshItems().catch((error) => toastError(error, 'Could not filter items'))
}

export function clearFilters(): void {
  filters.value = { ...defaultListOptions(), sort: filters.value.sort }
  void refreshItems().catch((error) => toastError(error, 'Could not filter items'))
}

let detailToken = 0

export async function select(id: string | null): Promise<void> {
  if (!id) {
    detail.value = null
    return
  }
  const token = (detailToken += 1)
  detailLoading.value = true
  try {
    const item = await backend.getItem(id)
    if (token === detailToken) detail.value = normalizeItem(item)
  } catch (error) {
    if (token === detailToken) {
      detail.value = null
      toastError(error, 'Could not open that item')
    }
  } finally {
    if (token === detailToken) detailLoading.value = false
  }
}

export async function refreshDetail(): Promise<void> {
  const current = detail.value
  if (current) await select(current.id)
}

export function startCreate(type: ItemType = 'login'): void {
  editing.value = { ...blankItem(type) }
}

export function startEdit(item: Item): void {
  editing.value = normalizeItem(item)
}

export function closeEditor(): void {
  editing.value = null
}

export async function saveItem(item: Item): Promise<boolean> {
  busy.value = true
  try {
    const saved = await backend.saveItem(item)
    await Promise.all([refreshItems(), refreshTags(), refreshStats()])
    await select(saved.id)
    editing.value = null
    toastSuccess('Item saved', saved.title)
    return true
  } catch (error) {
    toastError(error, 'Could not save the item')
    return false
  } finally {
    busy.value = false
  }
}

export function requestDelete(item: DeletableItem): void {
  pendingDelete.value = { id: item.id, title: item.title }
}

export async function confirmDelete(): Promise<void> {
  const target = pendingDelete.value
  if (!target) return
  pendingDelete.value = null
  try {
    await backend.deleteItem(target.id)
    if (detail.value?.id === target.id) detail.value = null
    await Promise.all([refreshItems(), refreshTags(), refreshStats(), refreshTrash()])
    toastSuccess('Moved to trash', `${target.title} can be restored from the trash.`)
  } catch (error) {
    toastError(error, 'Could not delete the item')
  }
}

export async function toggleFavorite(id: string): Promise<void> {
  try {
    await backend.toggleFavorite(id)
    await Promise.all([refreshItems(), refreshStats()])
    if (detail.value?.id === id) detail.value = { ...detail.value, favorite: !detail.value.favorite }
  } catch (error) {
    toastError(error, 'Could not update the favorite')
  }
}

export async function saveSettings(next: Settings): Promise<boolean> {
  try {
    settings.value = await backend.saveSettings(next)
    theme.value = settings.value.theme
    storeTheme(settings.value.theme)
    applyTheme(settings.value.theme)
    return true
  } catch (error) {
    toastError(error, 'Could not save settings')
    return false
  }
}

export function setTheme(next: Theme): void {
  theme.value = next
  storeTheme(next)
  applyTheme(next)
}

/**
 * copy copies a secret and shows the result. The countdown comes from the
 * vault's own clipboard setting so the message cannot drift from the behaviour.
 */
export async function copy(value: string, label: string, seconds = clipboardSeconds.value): Promise<void> {
  try {
    const outcome = await copyWithClear(value, label, seconds)
    pushToast({ kind: 'success', title: `${outcome.label} copied`, detail: outcome.detail })
  } catch (error) {
    toastError(error, 'Copy failed')
  }
}

export async function generatePassword(options: Settings['generator']): Promise<string> {
  const generated = await backend.generatePassword(options)
  return generated.value
}

export async function revealTOTP(id: string): Promise<{ code: string; remaining: number; period: number } | null> {
  try {
    return await backend.totpCode(id)
  } catch (error) {
    toastError(error, 'Could not generate a code')
    return null
  }
}

export async function changeMasterPassword(current: string, next: string): Promise<void> {
  busy.value = true
  try {
    status.value = await backend.changeMasterPassword(current, next)
    toastSuccess('Master password changed', 'A fresh salt was generated and the data key was re-wrapped.')
  } finally {
    busy.value = false
  }
}

export async function updateHint(hint: string): Promise<void> {
  try {
    status.value = await backend.setHint(hint)
    status.value = await backend.status()
    toastSuccess('Hint updated')
  } catch (error) {
    toastError(error, 'Could not update the hint')
  }
}

/** Only meaningful in the browser preview: throw the local preview vault away. */
export async function resetPreview(): Promise<void> {
  if (!backend.resetPreview) return
  await backend.resetPreview()
  clearWorkspace()
  await refreshStatus()
  toastSuccess('Preview vault cleared', 'Create a new one to go through the setup flow again.')
}

export async function exportBackup(): Promise<void> {
  try {
    const path = await backend.exportBackup()
    if (path) toastSuccess('Encrypted backup written', path)
  } catch (error) {
    toastError(error, 'Export failed')
  }
}

export async function importVault(): Promise<void> {
  if (unlocked.value) {
    pushToast({ kind: 'info', title: 'Lock the vault first', detail: 'Importing replaces the current vault file.' })
    return
  }
  try {
    status.value = await backend.importVault()
    if (status.value.exists) toastSuccess('Vault imported', 'Unlock it with its master password.')
  } catch (error) {
    toastError(error, 'Import failed')
  }
}

export async function restoreBackup(): Promise<void> {
  try {
    status.value = await backend.restoreBackup()
    clearWorkspace()
    toastSuccess('Previous revision restored', 'Unlock the vault to continue.')
  } catch (error) {
    toastError(error, 'Restore failed')
  }
}

// -------------------------------------------------------------- idle tracking --

let ticker: number | null = null
let lastPing = 0

function startTicker(): void {
  if (ticker !== null) return
  ticker = window.setInterval(() => {
    void backend
      .autoLockState()
      .then((state) => {
        autoLock.value = state
      })
      .catch(() => undefined)
  }, 1000)
}

/**
 * registerActivity keeps the Go-side idle timer in step with real input. It is
 * throttled because it crosses the webview boundary.
 */
export function registerActivity(): void {
  if (!unlocked.value) return
  const now = Date.now()
  if (now - lastPing < 4000) return
  lastPing = now
  void backend
    .ping()
    .then((state) => {
      autoLock.value = state
    })
    .catch(() => undefined)
}

let uninstallLockListener: (() => void) | null = null

function installLockListener(): void {
  if (uninstallLockListener) return
  uninstallLockListener = backend.onLocked((reason) => {
    void (async () => {
      await refreshStatus()
      clearWorkspace()
      pushToast({
        kind: 'info',
        title: reason === 'timeout' ? 'Vault locked after inactivity' : 'Vault locked',
        detail: reason === 'timeout' ? 'Unlock again to continue.' : undefined,
      })
    })()
  })
}

/** blankItem builds the template for a brand new item. */
function blankItem(type: ItemType): Item {
  const now = new Date().toISOString()
  return {
    id: '',
    type,
    title: '',
    username: '',
    password: '',
    url: '',
    notes: '',
    tags: [],
    favorite: false,
    totp: '',
    card: type === 'card' ? {} : undefined,
    custom: [],
    history: [],
    createdAt: now,
    updatedAt: now,
  }
}

export { friendlyError }
