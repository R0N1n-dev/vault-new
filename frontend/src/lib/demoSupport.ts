/**
 * Helpers the browser preview backend needs: TOTP parsing/verification, the
 * strength heuristic and sample content. Kept apart from demoBackend.ts so no
 * single file has to carry the sample fixtures.
 */

import type { Card, Item, ItemType, Strength } from '../types'

export interface TotpConfig {
  secret: string
  secretBytes: Uint8Array
  digits: number
  period: number
  algorithm: 'SHA-1' | 'SHA-256' | 'SHA-512'
  issuer?: string
  account?: string
}

const BASE32_ALPHABET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'

/** base32Decode accepts the grouping, case and padding users paste. */
export function base32Decode(value: string): Uint8Array {
  const cleaned = value.toUpperCase().replace(/[^A-Z2-7]/g, '')
  const bytes: number[] = []
  let buffer = 0
  let bits = 0
  for (const char of cleaned) {
    const index = BASE32_ALPHABET.indexOf(char)
    if (index < 0) continue
    buffer = (buffer << 5) | index
    bits += 5
    if (bits >= 8) {
      bytes.push((buffer >>> (bits - 8)) & 0xff)
      bits -= 8
    }
  }
  // Ten base32 characters are only fifty bits, so the floor applies to the
  // decoded bytes, exactly as in the Go implementation.
  if (bytes.length < 10) throw new Error('one-time password secret is too short')
  return new Uint8Array(bytes)
}

export function parseTotp(raw: string): TotpConfig {
  const trimmed = raw.trim()
  if (!trimmed) throw new Error('missing one-time password secret')
  let secret = trimmed
  let digits = 6
  let period = 30
  let algorithm: TotpConfig['algorithm'] = 'SHA-1'
  let issuer = ''
  let account = ''

  if (trimmed.toLowerCase().startsWith('otpauth://')) {
    let parsed: URL
    try {
      parsed = new URL(trimmed)
    } catch {
      throw new Error('invalid otpauth URI')
    }
    if (parsed.host.toLowerCase() !== 'totp') throw new Error('only totp:// secrets are supported')
    const label = decodeURIComponent(parsed.pathname.replace(/^\//, ''))
    const [labelIssuer, labelAccount] = label.includes(':') ? label.split(':') : ['', label]
    issuer = parsed.searchParams.get('issuer') ?? (labelIssuer ?? '').trim()
    account = (labelAccount ?? '').trim()
    secret = parsed.searchParams.get('secret') ?? ''
    const parsedDigits = Number(parsed.searchParams.get('digits') ?? 6)
    if (Number.isFinite(parsedDigits) && parsedDigits >= 6 && parsedDigits <= 10) digits = parsedDigits
    const parsedPeriod = Number(parsed.searchParams.get('period') ?? 30)
    if (Number.isFinite(parsedPeriod) && parsedPeriod >= 10 && parsedPeriod <= 300) period = parsedPeriod
    const parsedAlgorithm = (parsed.searchParams.get('algorithm') ?? 'SHA1').toUpperCase()
    if (parsedAlgorithm === 'SHA256') algorithm = 'SHA-256'
    else if (parsedAlgorithm === 'SHA512') algorithm = 'SHA-512'
    else if (parsedAlgorithm !== 'SHA1') throw new Error(`unsupported algorithm ${parsedAlgorithm}`)
  }

  return { secret, secretBytes: base32Decode(secret), digits, period, algorithm, issuer, account }
}

/** totpFromSignature applies the RFC 4226 dynamic truncation. */
export function totpFromSignature(signature: Uint8Array, config: TotpConfig): { code: string; digits: number; period: number; remaining: number } {
  const offset = (signature[signature.length - 1] ?? 0) & 0x0f
  const value =
    (((signature[offset] ?? 0) & 0x7f) << 24) |
    ((signature[offset + 1] ?? 0) << 16) |
    ((signature[offset + 2] ?? 0) << 8) |
    (signature[offset + 3] ?? 0)
  const code = String(value % 10 ** config.digits).padStart(config.digits, '0')
  const elapsed = Math.floor(Date.now() / 1000) % config.period
  return { code, digits: config.digits, period: config.period, remaining: config.period - elapsed }
}

const SYMBOLS = '!@#$%^&*()-_=+[]{};:,.?/'
const COMMON = new Set(
  '123456 password 123456789 12345678 12345 1234567 qwerty abc123 111111 123123 letmein dragon admin welcome monkey sunshine princess football baseball shadow master michael superman batman trustno1 hello freedom whatever qazwsx zaq12wsx passw0rd p@ssw0rd password1 password123 iloveyou 000000 654321 666666 121212 guest changeme'.split(
    /\s+/,
  ),
)

/** scorePassword mirrors the Go heuristic in internal/vault/strength.go. */
export function scorePassword(password: string): Strength {
  if (!password) return { score: 0, label: 'Empty', entropyBits: 0, warning: 'No password set.' }
  const chars = [...password]
  const hasLower = /[a-z]/.test(password)
  const hasUpper = /[A-Z]/.test(password)
  const hasDigit = /[0-9]/.test(password)
  const hasSymbol = chars.some((char) => SYMBOLS.includes(char))
  const hasOther = chars.some((char) => !/[\x20-\x7e]/.test(char))
  let pool = 0
  if (hasLower || hasUpper) pool += 26
  if (hasLower && hasUpper) pool += 26
  if (!hasLower && !hasUpper) pool += 10
  if (hasDigit) pool += 10
  if (hasSymbol) pool += 33
  if (hasOther) pool += 100

  let entropy = chars.length * Math.log2(pool)
  const lower = password.toLowerCase()
  const folded = lower
    .replace(/[@4]/g, 'a')
    .replace(/3/g, 'e')
    .replace(/[1!]/g, 'i')
    .replace(/0/g, 'o')
    .replace(/[5$]/g, 's')
  if (COMMON.has(lower) || COMMON.has(folded)) {
    return {
      score: 0,
      label: 'Very weak',
      entropyBits: 0,
      warning: 'This is one of the most commonly used passwords.',
      suggestions: ['Pick something unique, or generate a random password.'],
    }
  }

  const suggestions: string[] = []
  let run = 1
  let longestRun = 1
  for (let index = 1; index < chars.length; index += 1) {
    run = chars[index] === chars[index - 1] ? run + 1 : 1
    longestRun = Math.max(longestRun, run)
  }
  if (longestRun >= 3) {
    entropy -= (longestRun - 2) * 2
    suggestions.push('Avoid repeating the same character.')
  }
  let sequential = 0
  for (let index = 1; index < chars.length; index += 1) {
    const delta = (chars[index] as string).charCodeAt(0) - (chars[index - 1] as string).charCodeAt(0)
    if (delta === 1 || delta === -1) sequential += 1
  }
  if (sequential > 2) {
    entropy -= sequential * 1.5
    suggestions.push('Avoid character sequences like abcd, 1234 or qwerty.')
  }
  if (/(19|20)\d{2}/.test(lower)) {
    entropy -= 6
    suggestions.push('Avoid years, dates and other predictable numbers.')
  }
  entropy = Math.max(entropy, 0)
  if (chars.length < 12) suggestions.push('Use at least 12 characters.')
  if (!hasSymbol && !hasOther) suggestions.push('Adding symbols increases the search space.')

  let score = 0
  let label = 'Very weak'
  if (entropy >= 100) {
    score = 4
    label = 'Excellent'
  } else if (entropy >= 70) {
    score = 3
    label = 'Strong'
  } else if (entropy >= 50) {
    score = 2
    label = 'Fair'
  } else if (entropy >= 30) {
    score = 1
    label = 'Weak'
  }
  return { score, label, entropyBits: Math.round(entropy * 10) / 10, suggestions: score >= 3 ? [] : suggestions }
}

/**
 * The first 256 entries of the EFF short word list. The packaged app ships all
 * 1296 words in Go; the preview keeps a smaller copy so the passphrase UI can
 * be reviewed without duplicating the whole list into the bundle.
 */
export const DEFAULT_WORDLIST: string[] = `
acid
acorn
acre
acts
afar
affix
aged
agent
agile
aging
agony
ahead
aide
aids
aim
ajar
alarm
alias
alibi
alien
alike
alive
aloe
aloft
aloha
alone
amend
amino
ample
amuse
angel
anger
angle
ankle
apple
april
apron
aqua
area
arena
argue
arise
armed
armor
army
aroma
array
arson
art
ashen
ashes
atlas
atom
attic
audio
avert
avoid
awake
award
awoke
axis
bacon
badge
bagel
baggy
baked
baker
balmy
banjo
barge
barn
bash
basil
bask
batch
bath
baton
bats
blade
blank
blast
blaze
bleak
blend
bless
blimp
blink
bloat
blob
blog
blot
blunt
blurt
blush
boast
boat
body
boil
bok
bolt
boned
boney
bonus
bony
book
booth
boots
boss
botch
both
boxer
breed
bribe
brick
bride
brim
bring
brink
brisk
broad
broil
broke
brook
broom
brush
buck
bud
buggy
bulge
bulk
bully
bunch
bunny
bunt
bush
bust
busy
buzz
cable
cache
cadet
cage
cake
calm
cameo
canal
candy
cane
canon
cape
card
cargo
carol
carry
carve
case
cash
cause
cedar
chain
chair
chant
chaos
charm
chase
cheek
cheer
chef
chess
chest
chew
chief
chili
chill
chip
chomp
chop
chow
chuck
chump
chunk
churn
chute
cider
cinch
city
civic
civil
clad
claim
clamp
clap
clash
clasp
class
claw
clay
clean
clear
cleat
cleft
clerk
click
cling
clink
clip
cloak
clock
clone
cloth
cloud
clump
coach
coast
coat
cod
coil
coke
cola
cold
colt
coma
come
comic
comma
cone
cope
copy
coral
cork
cost
cot
couch
cough
cover
cozy
craft
cramp
crane
crank
crate
crave
crawl
crazy
creme
crepe
crept
crib
cried
crisp
crook
crop
cross
crowd
crown
crumb
`
  .split(/\s+/)
  .map((word) => word.trim())
  .filter(Boolean)

/** sampleItems gives the preview something realistic to render. */
export function sampleItems(): Item[] {
  const stamp = (days: number): string => new Date(Date.now() - days * 86_400_000).toISOString()
  return [
    {
      id: crypto.randomUUID(),
      type: 'login' as ItemType,
      title: 'GitHub',
      username: 'ada@example.com',
      password: 'vE7#kQ2$mZ9!pL4&',
      url: 'https://github.com',
      notes: 'Primary work account. Recovery codes are in the safe.',
      tags: ['work', 'code'],
      favorite: true,
      totp: 'JBSWY3DPEHPK3PXP',
      custom: [],
      history: [{ password: 'an-old-replaced-password', changedAt: stamp(220) }],
      createdAt: stamp(420),
      updatedAt: stamp(3),
    },
    {
      id: crypto.randomUUID(),
      type: 'login' as ItemType,
      title: 'Fastmail',
      username: 'ada@example.com',
      password: 'qZ4#vN8*mX2!kL6&',
      url: 'https://fastmail.com',
      tags: ['personal'],
      totp: 'JBSWY3DPEHPK3PXP',
      custom: [],
      createdAt: stamp(360),
      updatedAt: stamp(12),
    },
    {
      id: crypto.randomUUID(),
      type: 'login' as ItemType,
      title: 'Router admin',
      username: 'admin',
      password: 'W8!harbor-drift-9',
      url: 'http://192.168.1.1',
      notes: 'Reboot weekly until the firmware update lands.',
      tags: ['home'],
      custom: [{ label: 'SSID', value: 'lovelace-net', secret: false }],
      createdAt: stamp(120),
      updatedAt: stamp(30),
    },
    {
      id: crypto.randomUUID(),
      type: 'card' as ItemType,
      title: 'Travel card',
      card: {
        cardholder: 'Ada Lovelace',
        number: '4111 1111 1111 1111',
        brand: 'Visa',
        expMonth: 7,
        expYear: 2029,
        cvc: '123',
      } as Card,
      tags: ['finance'],
      custom: [],
      createdAt: stamp(200),
      updatedAt: stamp(60),
    },
    {
      id: crypto.randomUUID(),
      type: 'note' as ItemType,
      title: 'Passport renewal',
      notes: 'Reference 8842-119.\nPhotos taken at the post office.',
      tags: ['personal', 'documents'],
      custom: [],
      createdAt: stamp(90),
      updatedAt: stamp(15),
    },
  ]
}
