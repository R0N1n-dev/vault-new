/**
 * A browser-only implementation of the Backend seam.
 *
 * This exists so the interface can be developed and reviewed in a normal
 * browser (and in a preview deployment) without the Go runtime. It is a real
 * encrypted store, not a stub: WebCrypto derives a key with PBKDF2-SHA256 and
 * encrypts the vault body with AES-256-GCM, using the same header/wrapped-key
 * shape as the packaged app.
 *
 * It is still NOT the product: the packaged app uses Argon2id and
 * XChaCha20-Poly1305 in Go, keeps the key only in native memory and never
 * writes the vault to browser storage. The UI says so prominently.
 */

import type {
  AppInfo,
  AuditFinding,
  AuditKind,
  AuditReport,
  AuditSeverity,
  AutoLockState,
  Generated,
  GeneratorOptions,
  ImportOptions,
  ImportResult,
  ImportRow,
  Item,
  ListOptions,
  Settings,
  Status,
  Stats,
  Strength,
  Summary,
  TOTPResult,
  TagCount,
  VaultInfo,
} from '../types'
import { defaultGeneratorOptions, defaultSettings } from '../types'
import type { Backend } from './backend'
import { duplicateKey, parseImportCsv } from './demoCsv'
import { DEFAULT_WORDLIST, parseTotp, sampleItems, scorePassword, totpFromSignature } from './demoSupport'

const STORAGE_KEY = 'coffer.preview.vault'
const PBKDF2_ITERATIONS = 310_000
const MIN_MASTER_LENGTH = 10

interface StoredBlob {
  v: 1
  meta: {
    vaultId: string
    createdAt: string
    updatedAt: string
    revision: number
    hint: string
    kdf: { alg: 'PBKDF2-SHA256'; iterations: number; salt: string }
    keyWrap: { iv: string; data: string }
    payloadIv: string
  }
  body: string
}

interface Body {
  version: number
  items: Item[]
  settings: Settings
  updatedAt: string
}

const encoder = new TextEncoder()
const decoder = new TextDecoder()

export function toBase64(bytes: Uint8Array): string {
  let binary = ''
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary)
}

export function fromBase64(value: string): Uint8Array {
  const binary = atob(value)
  const out = new Uint8Array(binary.length)
  for (let index = 0; index < binary.length; index += 1) out[index] = binary.charCodeAt(index)
  return out
}

/**
 * TypeScript's DOM lib types BufferSource as an ArrayBuffer-backed view, while
 * Uint8Array became generic over ArrayBufferLike in TS 5.7. WebCrypto accepts a
 * real Uint8Array happily, so this conversion is a typing formality only.
 */
function bs(bytes: Uint8Array): BufferSource {
  return bytes as unknown as BufferSource
}

function randomBytes(length: number): Uint8Array {
  const bytes = new Uint8Array(length)
  crypto.getRandomValues(bytes)
  return bytes
}

/** randInt uses rejection sampling so no modulo bias creeps into the generator. */
export function randInt(bound: number): number {
  if (bound <= 0) throw new Error('empty character set')
  const limit = Math.floor(0xffffffff / bound) * bound
  const buffer = new Uint32Array(1)
  for (;;) {
    crypto.getRandomValues(buffer)
    const value = buffer[0] ?? 0
    if (value < limit) return value % bound
  }
}

function nowIso(): string {
  return new Date(Math.floor(Date.now() / 1000) * 1000).toISOString()
}

/** humanDays renders a day count the way the Go audit does. */
function humanDays(days: number): string {
  if (days >= 730) return `${Math.floor(days / 365)} years`
  if (days >= 365) return 'over a year'
  if (days >= 60) return `${Math.floor(days / 30)} months`
  return `${days} days`
}

export class DemoBackend implements Backend {
  readonly kind = 'demo' as const
  readonly nativeDialogs = false

  private blob: StoredBlob | null = null
  private dek: CryptoKey | null = null
  private body: Body | null = null
  private idleSince = Date.now()
  private lockHandlers: Array<(reason: string) => void> = []
  private timer: number | null = null

  constructor() {
    this.blob = this.readBlob()
    this.timer = window.setInterval(() => this.checkAutoLock(), 1000)
  }

  // ------------------------------------------------------------- internals --

  private readBlob(): StoredBlob | null {
    try {
      const raw = window.localStorage.getItem(STORAGE_KEY)
      if (!raw) return null
      const parsed = JSON.parse(raw) as StoredBlob
      if (parsed.v !== 1 || !parsed.meta || typeof parsed.body !== 'string') return null
      return parsed
    } catch {
      return null
    }
  }

  private writeBlob(blob: StoredBlob | null): void {
    if (!blob) {
      window.localStorage.removeItem(STORAGE_KEY)
      return
    }
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(blob))
  }

  private requireUnlocked(): Body {
    if (!this.body || !this.dek) throw new Error('vault is locked')
    return this.body
  }

  private metaAad(meta: StoredBlob['meta']): Uint8Array {
    const { kdf, vaultId, createdAt } = meta
    return encoder.encode(
      JSON.stringify({ alg: kdf.alg, iterations: kdf.iterations, salt: kdf.salt, vaultId, createdAt }),
    )
  }

  private async deriveKek(password: string, salt: Uint8Array, iterations: number): Promise<CryptoKey> {
    const material = await crypto.subtle.importKey('raw', bs(encoder.encode(password)), 'PBKDF2', false, ['deriveKey'])
    return crypto.subtle.deriveKey(
      { name: 'PBKDF2', salt: bs(salt), iterations, hash: 'SHA-256' },
      material,
      { name: 'AES-GCM', length: 256 },
      false,
      ['encrypt', 'decrypt'],
    )
  }

  private buildInfo(): VaultInfo | undefined {
    if (!this.blob) return undefined
    const { meta } = this.blob
    return {
      path: 'browser preview (localStorage)',
      vaultId: meta.vaultId,
      createdAt: meta.createdAt,
      updatedAt: meta.updatedAt,
      revision: meta.revision,
      hint: meta.hint,
      format: 1,
      kdf: `PBKDF2-SHA256 · ${meta.kdf.iterations.toLocaleString()} iterations`,
      memoryMiB: 0,
      passes: 1,
      parallelism: 1,
      backupAvailable: false,
    }
  }

  private async persist(): Promise<void> {
    const body = this.requireUnlocked()
    if (!this.blob || !this.dek) throw new Error('vault is locked')
    const meta = { ...this.blob.meta, updatedAt: nowIso(), revision: this.blob.meta.revision + 1 }
    const plaintext = encoder.encode(JSON.stringify({ ...body, updatedAt: meta.updatedAt }))
    const iv = randomBytes(12)
    const aad = this.metaAad(meta)
    const cipher = await crypto.subtle.encrypt(
      { name: 'AES-GCM', iv: bs(iv), additionalData: bs(aad) },
      this.dek,
      bs(plaintext),
    )
    for (let index = 0; index < plaintext.length; index += 1) plaintext[index] = 0
    this.blob = { v: 1, meta: { ...meta, payloadIv: toBase64(iv) }, body: toBase64(new Uint8Array(cipher)) }
    this.body = { ...body, updatedAt: meta.updatedAt }
    this.writeBlob(this.blob)
  }

  private checkAutoLock(): void {
    if (!this.body) return
    const minutes = this.body.settings.autoLockMinutes
    if (minutes <= 0) return
    if (Date.now() - this.idleSince >= minutes * 60_000) {
      void this.lock().then(() => this.emitLocked('timeout'))
    }
  }

  private emitLocked(reason: string): void {
    for (const handler of this.lockHandlers) handler(reason)
  }

  private summary(item: Item): Summary {
    return {
      id: item.id,
      type: item.type,
      title: item.title,
      username: item.username ?? '',
      url: item.url ?? '',
      tags: item.tags ?? [],
      favorite: item.favorite ?? false,
      hasPassword: Boolean(item.password),
      hasTOTP: Boolean(item.totp),
      hasNotes: Boolean(item.notes),
      updatedAt: item.updatedAt,
      deletedAt: item.deletedAt,
    }
  }

  /**
   * purgeExpired mirrors the Go sweep: trashed items older than the retention
   * window are dropped the next time the vault is written.
   */
  private purgeExpired(): number {
    const body = this.body
    if (!body) return 0
    const days = body.settings.trashRetentionDays
    if (!days || days <= 0) return 0
    const cutoff = Date.now() - days * 86_400_000
    const before = body.items.length
    body.items = body.items.filter((item) => !item.deletedAt || Date.parse(item.deletedAt) >= cutoff)
    return before - body.items.length
  }

  // ------------------------------------------------------------------- api --

  async appInfo(): Promise<AppInfo> {
    return {
      name: 'Coffer',
      version: '1.0.0-preview',
      platform: 'browser',
      vaultPath: 'browser preview (localStorage)',
      dataDir: 'browser preview',
    }
  }

  async status(): Promise<Status> {
    return {
      exists: this.blob !== null,
      unlocked: this.body !== null,
      info: this.buildInfo(),
      itemCount: this.body?.items.length ?? 0,
      corrupt: false,
    }
  }

  async createVault(masterPassword: string, hint: string): Promise<Status> {
    if (this.blob) throw new Error('a vault file already exists')
    if (masterPassword.length < MIN_MASTER_LENGTH) {
      throw new Error(`the master password must be at least ${MIN_MASTER_LENGTH} characters`)
    }
    const salt = randomBytes(16)
    const kek = await this.deriveKek(masterPassword, salt, PBKDF2_ITERATIONS)
    const dekBytes = randomBytes(32)
    const wrapIv = randomBytes(12)
    const createdAt = nowIso()
    const meta: StoredBlob['meta'] = {
      vaultId: crypto.randomUUID(),
      createdAt,
      updatedAt: createdAt,
      revision: 1,
      hint,
      kdf: { alg: 'PBKDF2-SHA256', iterations: PBKDF2_ITERATIONS, salt: toBase64(salt) },
      keyWrap: { iv: toBase64(wrapIv), data: '' },
      payloadIv: toBase64(randomBytes(12)),
    }
    const wrapped = await crypto.subtle.encrypt(
      { name: 'AES-GCM', iv: bs(wrapIv), additionalData: bs(this.metaAad(meta)) },
      kek,
      bs(dekBytes),
    )
    meta.keyWrap.data = toBase64(new Uint8Array(wrapped))

    this.dek = await crypto.subtle.importKey('raw', bs(dekBytes), { name: 'AES-GCM' }, true, ['encrypt', 'decrypt'])
    this.body = { version: 1, items: sampleItems(), settings: defaultSettings(), updatedAt: createdAt }
    this.blob = { v: 1, meta, body: '' }
    await this.persist()
    this.idleSince = Date.now()
    return this.status()
  }

  async unlock(masterPassword: string): Promise<Status> {
    if (this.body) return this.status()
    if (!this.blob) throw new Error('no vault at this location')
    const { meta } = this.blob
    const kek = await this.deriveKek(masterPassword, fromBase64(meta.kdf.salt), meta.kdf.iterations)
    const aad = this.metaAad(meta)
    let dekBytes: Uint8Array
    try {
      const plain = await crypto.subtle.decrypt(
        { name: 'AES-GCM', iv: bs(fromBase64(meta.keyWrap.iv)), additionalData: bs(aad) },
        kek,
        bs(fromBase64(meta.keyWrap.data)),
      )
      dekBytes = new Uint8Array(plain)
    } catch {
      throw new Error('invalid master password or corrupted vault')
    }
    const dek = await crypto.subtle.importKey('raw', bs(dekBytes), { name: 'AES-GCM' }, true, ['encrypt', 'decrypt'])
    let parsed: Body
    try {
      const plain = await crypto.subtle.decrypt(
        { name: 'AES-GCM', iv: bs(fromBase64(meta.payloadIv)), additionalData: bs(aad) },
        dek,
        bs(fromBase64(this.blob.body)),
      )
      parsed = JSON.parse(decoder.decode(plain)) as Body
    } catch {
      throw new Error('vault file is corrupt or unsupported')
    }
    for (let index = 0; index < dekBytes.length; index += 1) dekBytes[index] = 0
    this.dek = dek
    this.body = {
      version: parsed.version ?? 1,
      items: parsed.items ?? [],
      settings: { ...defaultSettings(), ...parsed.settings },
      updatedAt: parsed.updatedAt ?? nowIso(),
    }
    this.idleSince = Date.now()
    return this.status()
  }

  async lock(): Promise<Status> {
    this.dek = null
    this.body = null
    return this.status()
  }

  async changeMasterPassword(current: string, next: string): Promise<Status> {
    this.requireUnlocked()
    if (!this.blob || !this.dek) throw new Error('vault is locked')
    if (next.length < MIN_MASTER_LENGTH) {
      throw new Error(`the master password must be at least ${MIN_MASTER_LENGTH} characters`)
    }
    const check = await this.deriveKek(current, fromBase64(this.blob.meta.kdf.salt), this.blob.meta.kdf.iterations)
    try {
      await crypto.subtle.decrypt(
        {
          name: 'AES-GCM',
          iv: bs(fromBase64(this.blob.meta.keyWrap.iv)),
          additionalData: bs(this.metaAad(this.blob.meta)),
        },
        check,
        bs(fromBase64(this.blob.meta.keyWrap.data)),
      )
    } catch {
      throw new Error('invalid master password or corrupted vault')
    }

    const salt = randomBytes(16)
    const kek = await this.deriveKek(next, salt, PBKDF2_ITERATIONS)
    const dekBytes = new Uint8Array(await crypto.subtle.exportKey('raw', this.dek))
    const wrapIv = randomBytes(12)
    const meta: StoredBlob['meta'] = {
      ...this.blob.meta,
      kdf: { alg: 'PBKDF2-SHA256', iterations: PBKDF2_ITERATIONS, salt: toBase64(salt) },
      keyWrap: { iv: toBase64(wrapIv), data: '' },
      revision: this.blob.meta.revision + 1,
      updatedAt: nowIso(),
    }
    const wrapped = await crypto.subtle.encrypt(
      { name: 'AES-GCM', iv: bs(wrapIv), additionalData: bs(this.metaAad(meta)) },
      kek,
      bs(dekBytes),
    )
    meta.keyWrap.data = toBase64(new Uint8Array(wrapped))
    for (let index = 0; index < dekBytes.length; index += 1) dekBytes[index] = 0
    this.blob = { v: 1, meta, body: this.blob.body }
    await this.persist()
    return this.status()
  }

  async setHint(hint: string): Promise<Status> {
    this.requireUnlocked()
    if (!this.blob) throw new Error('vault is locked')
    this.blob = { ...this.blob, meta: { ...this.blob.meta, hint } }
    await this.persist()
    return this.status()
  }

  async listItems(options: ListOptions): Promise<Summary[]> {
    const body = this.requireUnlocked()
    const terms = options.query.toLowerCase().split(/\s+/).filter(Boolean)
    const matches = body.items.filter((item) => {
      if (item.deletedAt) return false
      if (options.favoritesOnly && !item.favorite) return false
      if (options.type && item.type !== options.type) return false
      if (options.tag && !(item.tags ?? []).some((tag) => tag.toLowerCase() === options.tag.toLowerCase())) {
        return false
      }
      if (terms.length === 0) return true
      const haystack = [
        item.title,
        item.username ?? '',
        item.url ?? '',
        item.notes ?? '',
        (item.tags ?? []).join(' '),
        (item.custom ?? []).map((field) => field.label).join(' '),
      ]
        .join('\n')
        .toLowerCase()
      return terms.every((term) => haystack.includes(term))
    })
    const sorted = [...matches]
    if (options.sort === 'updated') sorted.sort((a, b) => b.updatedAt.localeCompare(a.updatedAt))
    else if (options.sort === 'created') sorted.sort((a, b) => b.createdAt.localeCompare(a.createdAt))
    else sorted.sort((a, b) => a.title.toLowerCase().localeCompare(b.title.toLowerCase()))
    return sorted.map((item) => this.summary(item))
  }

  async getItem(id: string): Promise<Item> {
    const body = this.requireUnlocked()
    const item = body.items.find((candidate) => candidate.id === id && !candidate.deletedAt)
    if (!item) throw new Error('item not found')
    return JSON.parse(JSON.stringify(item)) as Item
  }

  async saveItem(input: Item): Promise<Item> {
    const body = this.requireUnlocked()
    const item: Item = {
      ...input,
      title: input.title.trim(),
      tags: [...new Set((input.tags ?? []).map((tag) => tag.trim()).filter(Boolean))].sort(),
      custom: (input.custom ?? []).filter((field) => field.label.trim() !== '' || field.value !== ''),
      updatedAt: nowIso(),
    }
    if (!item.type) throw new Error('item type is required')
    if (!item.title) throw new Error('title is required')
    if (item.totp) parseTotp(item.totp)

    delete item.deletedAt
    if (!item.id) {
      item.id = crypto.randomUUID()
      item.createdAt = nowIso()
      item.history = []
      body.items.push(item)
    } else {
      const index = body.items.findIndex((candidate) => candidate.id === item.id && !candidate.deletedAt)
      if (index < 0) throw new Error('item not found')
      const existing = body.items[index] as Item
      let history = existing.history ?? []
      if (existing.password && existing.password !== item.password) {
        history = [{ password: existing.password, changedAt: nowIso() }, ...history]
      }
      const limit = body.settings.passwordHistoryMax
      if (limit > 0) history = history.slice(0, limit)
      item.createdAt = existing.createdAt
      item.history = history
      body.items[index] = item
    }
    await this.persist()
    return JSON.parse(JSON.stringify(item)) as Item
  }

  /** Moves an item to the trash, matching the Go implementation. */
  async deleteItem(id: string): Promise<void> {
    const body = this.requireUnlocked()
    const item = body.items.find((candidate) => candidate.id === id && !candidate.deletedAt)
    if (!item) throw new Error('item not found')
    item.deletedAt = nowIso()
    item.updatedAt = item.deletedAt
    await this.persist()
  }

  async trashList(): Promise<Summary[]> {
    const body = this.requireUnlocked()
    return body.items
      .filter((item) => Boolean(item.deletedAt))
      .sort((a, b) => (b.deletedAt ?? '').localeCompare(a.deletedAt ?? ''))
      .map((item) => this.summary(item))
  }

  async restoreItem(id: string): Promise<Item> {
    const body = this.requireUnlocked()
    const item = body.items.find((candidate) => candidate.id === id && Boolean(candidate.deletedAt))
    if (!item) throw new Error('item not found')
    delete item.deletedAt
    item.updatedAt = nowIso()
    await this.persist()
    return JSON.parse(JSON.stringify(item)) as Item
  }

  async deleteForever(id: string): Promise<void> {
    const body = this.requireUnlocked()
    const index = body.items.findIndex((item) => item.id === id)
    if (index < 0) throw new Error('item not found')
    body.items.splice(index, 1)
    await this.persist()
  }

  async emptyTrash(): Promise<number> {
    const body = this.requireUnlocked()
    const before = body.items.length
    body.items = body.items.filter((item) => !item.deletedAt)
    const removed = before - body.items.length
    if (removed > 0) await this.persist()
    return removed
  }

  async toggleFavorite(id: string): Promise<Item> {
    const item = await this.getItem(id)
    item.favorite = !item.favorite
    return this.saveItem(item)
  }

  async tags(): Promise<TagCount[]> {
    const body = this.requireUnlocked()
    const counts = new Map<string, TagCount>()
    for (const item of body.items) {
      if (item.deletedAt) continue
      for (const tag of item.tags ?? []) {
        const key = tag.toLowerCase()
        const entry = counts.get(key) ?? { name: tag, count: 0 }
        entry.count += 1
        counts.set(key, entry)
      }
    }
    return [...counts.values()].sort((a, b) => b.count - a.count || a.name.localeCompare(b.name))
  }

  async stats(): Promise<Stats> {
    const body = this.requireUnlocked()
    const stats: Stats = { total: 0, logins: 0, notes: 0, cards: 0, favorites: 0, withTOTP: 0, trashed: 0 }
    for (const item of body.items) {
      if (item.deletedAt) {
        stats.trashed += 1
        continue
      }
      stats.total += 1
      if (item.type === 'login') stats.logins += 1
      if (item.type === 'note') stats.notes += 1
      if (item.type === 'card') stats.cards += 1
      if (item.favorite) stats.favorites += 1
      if (item.totp) stats.withTOTP += 1
    }
    return stats
  }

  /**
   * audit mirrors internal/vault/audit.go: weak, reused and stale passwords,
   * logins without a second factor, and card expiry. Nothing leaves the device.
   */
  async audit(): Promise<AuditReport> {
    const body = this.requireUnlocked()
    const STALE_DAYS = 365
    const WEAK_SCORE = 2
    const CARD_WARNING_DAYS = 60
    const penalty: Record<AuditKind, number> = {
      weak: 18,
      reused: 12,
      cardExpired: 8,
      stale: 6,
      cardExpiringSoon: 3,
      noTotp: 3,
    }
    const rank: Record<AuditSeverity, number> = { high: 0, medium: 1, low: 2 }
    const now = new Date()
    const findings: AuditFinding[] = []
    const report: AuditReport = {
      score: 100,
      items: 0,
      affected: 0,
      weak: 0,
      reused: 0,
      stale: 0,
      noTotp: 0,
      cards: 0,
      findings,
      trashed: 0,
      checkedAt: now.toISOString(),
    }
    const add = (item: Item, kind: AuditKind, severity: AuditSeverity, detail: string): void => {
      findings.push({ kind, severity, itemId: item.id, title: item.title, detail, priority: penalty[kind] })
    }

    const live = body.items.filter((item) => !item.deletedAt)
    report.trashed = body.items.length - live.length
    report.items = live.length

    const groups = new Map<string, Item[]>()
    for (const item of live) {
      if (!item.password) continue
      const bucket = groups.get(item.password) ?? []
      bucket.push(item)
      groups.set(item.password, bucket)
    }

    for (const item of live) {
      if (item.password) {
        const strength = scorePassword(item.password)
        if (strength.score <= WEAK_SCORE) {
          report.weak += 1
          add(
            item,
            'weak',
            strength.score <= 1 ? 'high' : 'medium',
            `The password rates "${strength.label}", about ${Math.round(strength.entropyBits)} bits of entropy.`,
          )
        }
      }
      if (item.password && (groups.get(item.password)?.length ?? 0) > 1) {
        report.reused += 1
        const others = (groups.get(item.password)?.length ?? 1) - 1
        add(item, 'reused', 'high', `The same password also protects ${others} other ${others === 1 ? 'item' : 'items'}.`)
      }
      if (item.password) {
        const changed = item.history?.[0]?.changedAt ?? item.createdAt
        const age = Math.floor((now.getTime() - Date.parse(changed)) / 86_400_000)
        if (age >= STALE_DAYS) {
          report.stale += 1
          add(item, 'stale', 'medium', `The password has not changed in ${humanDays(age)} (last change ${changed.slice(0, 10)}).`)
        }
      }
      if (item.type === 'login' && item.password && item.url && !item.totp) {
        report.noTotp += 1
        add(item, 'noTotp', 'low', 'No second factor is stored for this login.')
      }
      if (item.type === 'card' && item.card?.expYear) {
        const expiry = new Date(Date.UTC(item.card.expYear, item.card.expMonth ?? 1, 0))
        const days = Math.floor((expiry.getTime() - now.getTime()) / 86_400_000)
        if (expiry.getTime() < now.getTime()) {
          report.cards += 1
          add(item, 'cardExpired', 'medium', `This card expired on ${expiry.toISOString().slice(0, 7)}.`)
        } else if (days <= CARD_WARNING_DAYS) {
          report.cards += 1
          add(item, 'cardExpiringSoon', 'low', `This card expires on ${expiry.toISOString().slice(0, 7)}.`)
        }
      }
    }

    const worst = new Map<string, number>()
    for (const finding of findings) {
      worst.set(finding.itemId, Math.max(worst.get(finding.itemId) ?? 0, finding.priority))
    }
    for (const cost of worst.values()) report.score -= cost
    report.score = Math.max(report.score, 0)
    report.affected = worst.size

    findings.sort((a, b) => {
      if (rank[a.severity] !== rank[b.severity]) return rank[a.severity] - rank[b.severity]
      if (a.priority !== b.priority) return b.priority - a.priority
      return a.title.toLowerCase().localeCompare(b.title.toLowerCase())
    })
    return report
  }

  /** Merges a CSV export, mirroring internal/vault/csvimport.go. */
  async importCsv(text: string, options: ImportOptions): Promise<ImportResult> {
    const body = this.requireUnlocked()
    const mode = options.mode === 'add' ? 'add' : 'skip'
    const parsed = parseImportCsv(text)

    const seen = new Set<string>()
    for (const item of body.items) {
      if (!item.deletedAt) seen.add(duplicateKey(item.title, item.username ?? ''))
    }

    const rows: ImportRow[] = []
    const pending: Item[] = []
    let fresh = 0
    let duplicates = 0
    let invalid = 0
    let unsupported = 0

    for (const row of parsed.rows) {
      const summary: ImportRow = {
        line: row.line,
        title: row.title,
        username: row.username,
        url: row.url,
        type: row.type,
        status: 'new',
      }
      if (row.status === 'unsupported') {
        unsupported += 1
        rows.push({ ...summary, status: 'unsupported', reason: row.problem })
        continue
      }
      if (row.status === 'invalid') {
        invalid += 1
        rows.push({ ...summary, status: 'invalid', reason: row.problem })
        continue
      }
      const key = duplicateKey(row.title, row.username)
      if (seen.has(key) && mode !== 'add') {
        duplicates += 1
        rows.push({ ...summary, status: 'duplicate', reason: 'an item with this title and username is already in the vault' })
        continue
      }
      const stamp = nowIso()
      pending.push({
        id: crypto.randomUUID(),
        type: row.type,
        title: row.title,
        username: row.username,
        password: row.password,
        url: row.type === 'note' ? '' : row.url,
        notes: row.type === 'note' && row.url ? (row.notes ? `${row.url}\n${row.notes}` : row.url) : row.notes,
        tags: [...new Set(row.tags)],
        favorite: row.favorite,
        totp: row.totp,
        custom: [],
        history: [],
        createdAt: stamp,
        updatedAt: stamp,
      })
      seen.add(key)
      fresh += 1
      rows.push(summary)
    }

    if (!options.dryRun && pending.length > 0) {
      body.items.push(...pending)
      await this.persist()
    }

    const warnings = [...parsed.warnings]
    if (parsed.truncated) warnings.push('only the first 2000 rows were read')
    return {
      format: parsed.format,
      dryRun: options.dryRun,
      mode,
      total: rows.length,
      new: fresh,
      duplicates,
      invalid,
      unsupported,
      imported: options.dryRun ? 0 : pending.length,
      rows,
      warnings,
    }
  }

  async getSettings(): Promise<Settings> {
    const body = this.requireUnlocked()
    return { ...body.settings, generator: { ...body.settings.generator } }
  }

  async saveSettings(settings: Settings): Promise<Settings> {
    const body = this.requireUnlocked()
    const merged: Settings = {
      ...settings,
      generator: { ...defaultGeneratorOptions(), ...settings.generator },
      autoLockMinutes: Math.min(Math.max(settings.autoLockMinutes, 0), 24 * 60),
      clipboardClearSeconds: Math.min(Math.max(settings.clipboardClearSeconds, 0), 600),
      passwordHistoryMax: Math.min(Math.max(settings.passwordHistoryMax, 0), 20),
      trashRetentionDays: Math.min(Math.max(settings.trashRetentionDays ?? 0, 0), 3650),
    }
    body.settings = merged
    this.purgeExpired()
    await this.persist()
    return merged
  }

  async generatePassword(options: GeneratorOptions): Promise<Generated> {
    const opts = { ...defaultGeneratorOptions(), ...options }
    if (opts.mode === 'passphrase') {
      const separator = opts.separator || '-'
      // Draw only from words that cannot contain the separator, so the result
      // always splits back into its parts (same rule as the Go generator).
      const usable = DEFAULT_WORDLIST.filter((candidate) => !candidate.includes(separator))
      const words: string[] = []
      for (let index = 0; index < opts.words; index += 1) {
        const word = usable[randInt(usable.length)] ?? 'harbor'
        words.push(opts.capitalize ? (word[0]?.toUpperCase() ?? '') + word.slice(1) : word)
      }
      if (opts.appendNumber) words.push(String(randInt(10)))
      const entropy = opts.words * Math.log2(Math.max(usable.length, 2)) + (opts.appendNumber ? Math.log2(10) : 0)
      const value = words.join(separator)
      return { value, entropyBits: Math.round(entropy * 10) / 10, strength: scorePassword(value) }
    }
    const ambiguous = 'Il1O0o|'
    const filter = (set: string): string =>
      opts.excludeAmbiguous ? [...set].filter((char) => !ambiguous.includes(char)).join('') : set
    const classes = [
      opts.lowercase ? filter('abcdefghijklmnopqrstuvwxyz') : '',
      opts.uppercase ? filter('ABCDEFGHIJKLMNOPQRSTUVWXYZ') : '',
      opts.digits ? filter('0123456789') : '',
      opts.symbols ? filter('!@#$%^&*()-_=+[]{};:,.?/') : '',
    ].filter((set) => set.length > 0)

    const pool = classes.join('')
    const length = Math.min(Math.max(opts.length, 8), 128)
    if (pool.length < 2) throw new Error('select at least two character sets')
    const chars: string[] = []
    if (opts.requireEverySelected) {
      for (const set of classes) chars.push(set[randInt(set.length)] as string)
    }
    while (chars.length < length) chars.push(pool[randInt(pool.length)] as string)
    for (let index = chars.length - 1; index > 0; index -= 1) {
      const swap = randInt(index + 1)
      const current = chars[index] as string
      chars[index] = chars[swap] as string
      chars[swap] = current
    }
    const value = chars.slice(0, length).join('')
    const entropy = length * Math.log2(pool.length)
    return { value, entropyBits: Math.round(entropy * 10) / 10, strength: scorePassword(value) }
  }

  async scorePassword(password: string): Promise<Strength> {
    return scorePassword(password)
  }

  async totpCode(id: string): Promise<TOTPResult> {
    const item = await this.getItem(id)
    if (!item.totp) throw new Error('this item has no one-time password secret')
    const config = parseTotp(item.totp)
    const counter = Math.floor(Date.now() / 1000 / config.period)
    const key = await crypto.subtle.importKey(
      'raw',
      bs(config.secretBytes),
      { name: 'HMAC', hash: config.algorithm },
      false,
      ['sign'],
    )
    const message = new Uint8Array(8)
    let value = counter
    for (let index = 7; index >= 0; index -= 1) {
      message[index] = value & 0xff
      value = Math.floor(value / 256)
    }
    const signature = new Uint8Array(await crypto.subtle.sign('HMAC', key, bs(message)))
    return totpFromSignature(signature, config)
  }

  async copySecret(value: string, clearAfterSeconds: number): Promise<void> {
    this.requireUnlocked()
    if (value === '') throw new Error('nothing to copy')
    await navigator.clipboard.writeText(value)
    if (clearAfterSeconds > 0) {
      window.setTimeout(() => {
        void (async () => {
          try {
            const current = await navigator.clipboard.readText()
            if (current === value) await navigator.clipboard.writeText('')
          } catch {
            /* the browser may refuse clipboard reads; nothing we can do */
          }
        })()
      }, clearAfterSeconds * 1000)
    }
    this.idleSince = Date.now()
  }

  async clearClipboard(): Promise<void> {
    try {
      await navigator.clipboard.writeText('')
    } catch {
      /* ignore */
    }
  }

  async ping(): Promise<AutoLockState> {
    this.idleSince = Date.now()
    return this.autoLockState()
  }

  async autoLockState(): Promise<AutoLockState> {
    if (!this.body) return { enabled: false, timeoutSeconds: 0, secondsRemaining: 0 }
    const minutes = this.body.settings.autoLockMinutes
    if (minutes <= 0) return { enabled: false, timeoutSeconds: 0, secondsRemaining: 0 }
    const timeout = minutes * 60
    const elapsed = Math.floor((Date.now() - this.idleSince) / 1000)
    return { enabled: true, timeoutSeconds: timeout, secondsRemaining: Math.max(timeout - elapsed, 0) }
  }

  async exportBackup(): Promise<string> {
    if (!this.blob) throw new Error('there is no vault to export yet')
    const name = `coffer-preview-${new Date().toISOString().slice(0, 10)}.coffer.json`
    const url = URL.createObjectURL(new Blob([JSON.stringify(this.blob)], { type: 'application/json' }))
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = name
    anchor.click()
    URL.revokeObjectURL(url)
    return name
  }

  async importVault(): Promise<Status> {
    throw new Error('Importing a vault is only available in the desktop app')
  }

  async restoreBackup(): Promise<Status> {
    throw new Error('Restoring a backup is only available in the desktop app')
  }

  onLocked(handler: (reason: string) => void): () => void {
    this.lockHandlers.push(handler)
    return () => {
      this.lockHandlers = this.lockHandlers.filter((candidate) => candidate !== handler)
    }
  }

  /** Forgets the preview vault so the create-vault flow can be reviewed again. */
  async resetPreview(): Promise<void> {
    this.dek = null
    this.body = null
    this.blob = null
    this.writeBlob(null)
  }

  /** Releases the preview timer; used when the app unmounts. */
  dispose(): void {
    if (this.timer !== null) window.clearInterval(this.timer)
    this.timer = null
  }
}
