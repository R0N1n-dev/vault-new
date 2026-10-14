/**
 * Wire types. These mirror the Go structs in internal/vault and app.go exactly,
 * including the JSON field names that Wails uses for both directions.
 *
 * Fields tagged `omitempty` on the Go side can be absent in a response, so
 * every optional field is marked optional here and normalised on the way in.
 */

export type ItemType = 'login' | 'note' | 'card'
export type GeneratorMode = 'password' | 'passphrase'
export type Theme = 'dark' | 'light' | 'system'

export interface Card {
  cardholder?: string
  number?: string
  brand?: string
  expMonth?: number
  expYear?: number
  cvc?: string
}

export interface CustomField {
  label: string
  value: string
  secret?: boolean
}

export interface HistoryEntry {
  password: string
  changedAt: string
}

export interface Item {
  id: string
  type: ItemType
  title: string
  username?: string
  password?: string
  url?: string
  notes?: string
  tags?: string[]
  favorite?: boolean
  totp?: string
  card?: Card
  custom?: CustomField[]
  history?: HistoryEntry[]
  createdAt: string
  updatedAt: string
  /** Set while the item sits in the trash. */
  deletedAt?: string
}

export interface Summary {
  id: string
  type: ItemType
  title: string
  username?: string
  url?: string
  tags?: string[]
  favorite: boolean
  hasPassword: boolean
  hasTOTP: boolean
  hasNotes: boolean
  updatedAt: string
  deletedAt?: string
}

export interface VaultInfo {
  path: string
  vaultId: string
  createdAt: string
  updatedAt: string
  revision: number
  hint?: string
  format: number
  kdf: string
  memoryMiB: number
  passes: number
  parallelism: number
  backupAvailable: boolean
}

export interface Status {
  exists: boolean
  unlocked: boolean
  info?: VaultInfo
  itemCount: number
  corrupt: boolean
  message?: string
}

export interface AppInfo {
  name: string
  version: string
  platform: string
  vaultPath: string
  dataDir: string
}

export interface Settings {
  autoLockMinutes: number
  clipboardClearSeconds: number
  lockOnBlur: boolean
  theme: Theme
  generator: GeneratorOptions
  passwordHistoryMax: number
  /** Days a deleted item stays recoverable; 0 keeps it until removed by hand. */
  trashRetentionDays: number
}

export interface GeneratorOptions {
  mode: GeneratorMode
  length: number
  uppercase: boolean
  lowercase: boolean
  digits: boolean
  symbols: boolean
  excludeAmbiguous: boolean
  requireEverySelected: boolean
  words: number
  separator: string
  capitalize: boolean
  appendNumber: boolean
}

export interface Strength {
  score: number
  label: string
  entropyBits: number
  warning?: string
  suggestions?: string[]
}

export interface Generated {
  value: string
  strength: Strength
  entropyBits: number
}

export interface TOTPResult {
  code: string
  digits: number
  period: number
  remaining: number
}

export interface TagCount {
  name: string
  count: number
}

export interface Stats {
  total: number
  logins: number
  notes: number
  cards: number
  favorites: number
  withTOTP: number
  trashed: number
}

export type AuditSeverity = 'high' | 'medium' | 'low'
export type AuditKind = 'weak' | 'reused' | 'stale' | 'noTotp' | 'cardExpired' | 'cardExpiringSoon'

export interface AuditFinding {
  kind: AuditKind
  severity: AuditSeverity
  itemId: string
  title: string
  detail: string
  priority: number
}

/** The offline health report produced by the vault itself. */
export interface AuditReport {
  score: number
  items: number
  affected: number
  weak: number
  reused: number
  stale: number
  noTotp: number
  cards: number
  findings: AuditFinding[]
  trashed: number
  checkedAt: string
}

export type ImportMode = 'skip' | 'add'
export type ImportStatus = 'new' | 'duplicate' | 'invalid' | 'unsupported'

export interface ImportOptions {
  dryRun: boolean
  mode: ImportMode
}

export interface ImportRow {
  line: number
  title: string
  username?: string
  url?: string
  type: ItemType
  status: ImportStatus
  reason?: string
}

export interface ImportResult {
  format: string
  dryRun: boolean
  mode: ImportMode
  total: number
  new: number
  duplicates: number
  invalid: number
  unsupported: number
  imported: number
  rows: ImportRow[]
  warnings?: string[]
}

export interface ListOptions {
  query: string
  type: ItemType | ''
  tag: string
  favoritesOnly: boolean
  sort: 'title' | 'updated' | 'created'
}

export interface AutoLockState {
  enabled: boolean
  timeoutSeconds: number
  secondsRemaining: number
}

export function defaultGeneratorOptions(): GeneratorOptions {
  return {
    mode: 'password',
    length: 20,
    uppercase: true,
    lowercase: true,
    digits: true,
    symbols: true,
    excludeAmbiguous: true,
    requireEverySelected: true,
    words: 6,
    separator: '-',
    capitalize: true,
    appendNumber: true,
  }
}

export function defaultSettings(): Settings {
  return {
    autoLockMinutes: 5,
    clipboardClearSeconds: 30,
    lockOnBlur: false,
    theme: 'dark',
    generator: defaultGeneratorOptions(),
    passwordHistoryMax: 5,
    trashRetentionDays: 30,
  }
}

export function defaultListOptions(): ListOptions {
  return { query: '', type: '', tag: '', favoritesOnly: false, sort: 'title' }
}

/** Fills in the fields Go may omit so the UI never sees `undefined`. */
export function normalizeItem(item: Item): Required<Pick<Item, 'username' | 'password' | 'url' | 'notes' | 'totp'>> & Item {
  return {
    ...item,
    username: item.username ?? '',
    password: item.password ?? '',
    url: item.url ?? '',
    notes: item.notes ?? '',
    totp: item.totp ?? '',
    tags: item.tags ?? [],
    custom: item.custom ?? [],
    history: item.history ?? [],
    favorite: item.favorite ?? false,
  }
}

/** The shape the UI works with: every optional string field present. */
export type DraftItem = ReturnType<typeof normalizeItem>

export function emptyItem(type: ItemType = 'login'): Item {
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
    custom: [],
    history: [],
    createdAt: now,
    updatedAt: now,
  }
}
