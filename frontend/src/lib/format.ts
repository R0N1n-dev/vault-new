const MINUTE = 60_000
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR

/** relativeTime renders "3 min ago"-style labels for list rows. */
export function relativeTime(iso?: string): string {
  if (!iso) return ''
  const then = Date.parse(iso)
  if (Number.isNaN(then)) return ''
  const diff = Date.now() - then
  if (diff < 0) return 'just now'
  if (diff < MINUTE) return 'just now'
  if (diff < HOUR) return `${Math.floor(diff / MINUTE)} min ago`
  if (diff < DAY) return `${Math.floor(diff / HOUR)} h ago`
  if (diff < 30 * DAY) return `${Math.floor(diff / DAY)} d ago`
  return new Date(then).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' })
}

export function formatDateTime(iso?: string): string {
  if (!iso) return '—'
  const value = Date.parse(iso)
  if (Number.isNaN(value)) return '—'
  return new Date(value).toLocaleString(undefined, {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function formatDay(iso?: string): string {
  if (!iso) return '—'
  const value = Date.parse(iso)
  if (Number.isNaN(value)) return '—'
  return new Date(value).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' })
}

export function daysSince(iso?: string): number | null {
  if (!iso) return null
  const value = Date.parse(iso)
  if (Number.isNaN(value)) return null
  return Math.floor((Date.now() - value) / DAY)
}

/** conceal renders a fixed-width mask so length is not leaked in the list. */
export function conceal(length: number): string {
  return '•'.repeat(Math.min(Math.max(length, 8), 24))
}

export function hostFromUrl(url?: string): string {
  if (!url) return ''
  try {
    const parsed = new URL(url.includes('://') ? url : `https://${url}`)
    return parsed.host.replace(/^www\./, '')
  } catch {
    return url.replace(/^https?:\/\//, '').replace(/\/.*$/, '')
  }
}

export function initials(title: string): string {
  const cleaned = title.trim()
  if (!cleaned) return '?'
  const parts = cleaned.split(/\s+/).slice(0, 2)
  return parts.map((part) => part[0]?.toUpperCase() ?? '').join('') || '?'
}

/** hueFromText gives each item a stable accent colour. */
export function hueFromText(text: string): number {
  let hash = 0
  for (let index = 0; index < text.length; index += 1) {
    hash = (hash * 31 + text.charCodeAt(index)) % 360
  }
  return hash
}

export function pluralize(count: number, singular: string, plural?: string): string {
  return count === 1 ? singular : (plural ?? `${singular}s`)
}

export function formatCountdown(seconds: number): string {
  const safe = Math.max(0, Math.floor(seconds))
  const minutes = Math.floor(safe / 60)
  const rest = safe % 60
  if (minutes <= 0) return `${rest}s`
  return `${minutes}m ${rest.toString().padStart(2, '0')}s`
}

/** passwordTypeLabel maps the stored type onto a human label. */
export function typeLabel(type: string): string {
  switch (type) {
    case 'login':
      return 'Login'
    case 'note':
      return 'Secure note'
    case 'card':
      return 'Card'
    default:
      return 'Item'
  }
}
