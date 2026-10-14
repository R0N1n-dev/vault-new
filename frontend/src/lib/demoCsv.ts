/**
 * CSV import for the browser preview.
 *
 * This mirrors internal/vault/csvimport.go so the preview behaves like the
 * packaged app. The desktop build is the one that matters: it uses Go's
 * encoding/csv, and its behaviour is covered by the Go test suite. This module
 * exists so the import screen can be reviewed without the Go runtime.
 */

import type { ItemType } from '../types'

export const IMPORT_MAX_ROWS = 2000
export const IMPORT_MAX_BYTES = 8 * 1024 * 1024

export type ImportRowStatus = 'new' | 'duplicate' | 'invalid' | 'unsupported'

export interface ParsedRow {
  line: number
  title: string
  username: string
  url: string
  password: string
  notes: string
  totp: string
  favorite: boolean
  tags: string[]
  type: ItemType
  /** Empty when the row is usable; otherwise why it cannot be imported. */
  problem: string
  /** Set when the row is deliberately not representable in the vault. */
  unsupported: boolean
  status: ImportRowStatus
}

export interface ParsedCsv {
  format: string
  rows: ParsedRow[]
  warnings: string[]
  truncated: boolean
}

const ALIASES: Record<string, string> = {
  name: 'title',
  title: 'title',
  item: 'title',
  'item name': 'title',
  account: 'title',
  'account name': 'title',
  entry: 'title',
  'entry name': 'title',
  username: 'username',
  'login username': 'username',
  user: 'username',
  'user name': 'username',
  login: 'username',
  email: 'username',
  'login name': 'username',
  userid: 'username',
  password: 'password',
  'login password': 'password',
  pass: 'password',
  pwd: 'password',
  url: 'url',
  'login uri': 'url',
  uri: 'url',
  website: 'url',
  'web site': 'url',
  'login url': 'url',
  site: 'url',
  'web address': 'url',
  'login uri 1': 'url',
  notes: 'notes',
  note: 'notes',
  comments: 'notes',
  comment: 'notes',
  extra: 'notes',
  memo: 'notes',
  description: 'notes',
  'login totp': 'totp',
  totp: 'totp',
  otp: 'totp',
  otpauth: 'totp',
  'otp secret': 'totp',
  'one time password': 'totp',
  otpsecret: 'totp',
  type: 'kind',
  kind: 'kind',
  'item type': 'kind',
  folder: 'folder',
  group: 'folder',
  grouping: 'folder',
  category: 'folder',
  collection: 'folder',
  'folder name': 'folder',
  favorite: 'favorite',
  favourite: 'favorite',
  tags: 'tags',
  tag: 'tags',
}

function normalizeHeader(raw: string): string {
  return raw
    .replace(/^\ufeff/, '')
    .trim()
    .toLowerCase()
    .replace(/[_-]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
}

/** splitCsv reads a CSV document into records, tolerating ragged rows. */
export function splitCsv(text: string): string[][] {
  const records: string[][] = []
  let record: string[] = []
  let field = ''
  let quoted = false

  for (let index = 0; index < text.length; index += 1) {
    const char = text[index] as string
    if (quoted) {
      if (char === '"') {
        if (text[index + 1] === '"') {
          field += '"'
          index += 1
        } else {
          quoted = false
        }
      } else {
        field += char
      }
      continue
    }
    if (char === '"') {
      quoted = true
      continue
    }
    if (char === ',') {
      record.push(field)
      field = ''
      continue
    }
    if (char === '\r') continue
    if (char === '\n') {
      record.push(field)
      records.push(record)
      record = []
      field = ''
      continue
    }
    field += char
  }
  if (field !== '' || record.length > 0) {
    record.push(field)
    records.push(record)
  }
  return records
}

function formatName(headers: Set<string>): string {
  if (headers.has('login uri') || headers.has('login totp') || headers.has('reprompt')) return 'Bitwarden'
  if (headers.has('otpauth') || headers.has('archived')) return '1Password'
  if (headers.has('group') && headers.has('title')) return 'KeePass'
  if (headers.has('name') && !headers.has('title')) return 'Chrome / Edge'
  return 'Generic CSV'
}

function classifyType(raw: string): ItemType | null {
  switch (raw.trim().toLowerCase()) {
    case '':
    case 'login':
    case 'logins':
    case 'password':
    case 'web':
    case '1':
      return 'login'
    case 'note':
    case 'notes':
    case 'secure note':
    case 'securenote':
    case 'secure_note':
    case '2':
      return 'note'
    default:
      return null
  }
}

function fallbackTitle(username: string, url: string): string {
  if (username) return username
  if (!url) return ''
  try {
    const parsed = new URL(url.includes('://') ? url : `https://${url}`)
    if (parsed.hostname) return parsed.hostname
  } catch {
    /* not a URL */
  }
  return url
}

function truthy(raw: string): boolean {
  return ['1', 'true', 'yes', 'y'].includes(raw.trim().toLowerCase())
}

// The same field limits internal/vault enforces, so the preview rejects the
// rows the packaged app would refuse to store.
const LIMITS = { title: 512, field: 4096, notes: 64 * 1024 }

function splitTags(folder: string, tags: string): string[] {
  const out: string[] = []
  if (folder.trim()) out.push(folder.trim())
  for (const tag of tags.split(/[,;/|]/)) {
    if (tag.trim()) out.push(tag.trim())
  }
  return out
}

/**
 * normalizeTotp accepts a bare base32 secret or an otpauth:// URI. It returns
 * an empty string plus a reason when the value cannot be used.
 */
function normalizeTotp(raw: string): { secret: string; warning?: string } {
  const value = raw.trim()
  if (!value) return { secret: '' }
  if (value.toLowerCase().startsWith('otpauth://')) {
    try {
      const parsed = new URL(value)
      const secret = parsed.searchParams.get('secret')
      if (!secret) return { secret: '', warning: 'the otpauth URI has no secret parameter' }
      return { secret: secret.replace(/\s+/g, '').toUpperCase() }
    } catch {
      return { secret: '', warning: 'the otpauth URI could not be read' }
    }
  }
  return { secret: value.replace(/\s+/g, '').toUpperCase() }
}

/**
 * parseImportCsv classifies every row of an export. It throws on input that
 * cannot be read at all, matching the Go implementation's ErrInvalid.
 */
export function parseImportCsv(text: string): ParsedCsv {
  if (text.length > IMPORT_MAX_BYTES) {
    throw new Error('that file is larger than 8 MiB')
  }
  const clean = text.replace(/^\ufeff/, '')
  if (!clean.trim()) throw new Error('the CSV file is empty')

  const records = splitCsv(clean)
  if (records.length === 0) throw new Error('the CSV file is empty')

  const header = (records[0] as string[]).map(normalizeHeader)
  const headers = new Set(header)
  const columns: Record<string, number> = {}
  header.forEach((name, index) => {
    const field = ALIASES[name]
    if (field && columns[field] === undefined) columns[field] = index
  })

  if (columns.title === undefined || (columns.password === undefined && columns.notes === undefined)) {
    const found = (records[0] as string[]).filter((value) => value.trim() !== '').slice(0, 12).join(', ')
    throw new Error(`this CSV needs at least a title and a password or notes column (found: ${found})`)
  }

  const rows: ParsedRow[] = []
  const warnings: string[] = []
  for (let index = 1; index < records.length && rows.length < IMPORT_MAX_ROWS; index += 1) {
    const record = records[index] as string[]
    const value = (field: string): string => {
      const column = columns[field]
      if (column === undefined || column >= record.length) return ''
      return (record[column] ?? '') as string
    }

    const line = index + 1
    let title = value('title').trim()
    const username = value('username').trim()
    const password = value('password')
    const url = value('url').trim()
    const notes = value('notes')
    if (!title && !username && !password && !url && !notes) continue
    if (!title) title = fallbackTitle(username, url)

    const row: ParsedRow = {
      line,
      title,
      username,
      url,
      password,
      notes,
      totp: '',
      favorite: truthy(value('favorite')),
      tags: splitTags(value('folder'), value('tags')),
      type: 'login',
      problem: '',
      unsupported: false,
      status: 'new',
    }

    if (!title) {
      row.status = 'invalid'
      row.problem = 'the row has no title, username or URL to name the item'
      rows.push(row)
      continue
    }

    const kind = classifyType(value('kind'))
    if (kind === null) {
      row.status = 'unsupported'
      row.unsupported = true
      row.problem = `items of type "${value('kind').trim()}" are not part of the vault format`
      rows.push(row)
      continue
    }
    row.type = kind

    const rawTotp = value('totp').trim()
    if (rawTotp) {
      const normalized = normalizeTotp(rawTotp)
      if (normalized.warning) {
        warnings.push(`line ${line} (${title}): the one-time password secret was ignored: ${normalized.warning}`)
      } else if (!/^[A-Z2-7]+=*$/.test(normalized.secret) || normalized.secret.replace(/=+$/, '').length < 16) {
        warnings.push(`line ${line} (${title}): the one-time password secret was ignored: it is not a usable base32 secret`)
      } else {
        row.totp = normalized.secret
      }
    }

    const tooLong = [username, password, url, row.totp].find((entry) => entry.length > LIMITS.field)
    if (title.length > LIMITS.title) {
      row.status = 'invalid'
      row.problem = `title is longer than ${LIMITS.title} characters`
    } else if (tooLong !== undefined) {
      row.status = 'invalid'
      row.problem = 'a field is too long for the vault'
    } else if (notes.length > LIMITS.notes) {
      row.status = 'invalid'
      row.problem = 'the notes are longer than the vault allows'
    }
    rows.push(row)
  }

  return {
    format: formatName(headers),
    rows,
    warnings,
    truncated: records.length - 1 > IMPORT_MAX_ROWS,
  }
}

export function duplicateKey(title: string, username: string): string {
  return `${title.trim().toLowerCase()}\u0000${username.trim().toLowerCase()}`
}
