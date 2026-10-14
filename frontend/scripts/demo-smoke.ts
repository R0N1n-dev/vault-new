/**
 * Smoke test for the browser preview backend.
 *
 * The preview exists so the UI can be reviewed without the Go engine, which
 * means its crypto path needs to be real. This drives the same calls the UI
 * makes and asserts the vault round-trips, rejects the wrong password and never
 * stores plaintext. Run with `npm run smoke`.
 */

import { DemoBackend } from '../src/lib/demoBackend'
import type { Item } from '../src/types'

// Minimal browser surface for Node.
const store = new Map<string, string>()
;(globalThis as unknown as { window: unknown }).window = globalThis
;(globalThis as unknown as { localStorage: unknown }).localStorage = {
  getItem: (key: string) => store.get(key) ?? null,
  setItem: (key: string, value: string) => void store.set(key, value),
  removeItem: (key: string) => void store.delete(key),
}

let failures = 0
let checks = 0

function check(condition: boolean, label: string): void {
  checks += 1
  if (!condition) {
    failures += 1
    console.error(`  ✗ ${label}`)
  } else {
    console.log(`  ✓ ${label}`)
  }
}

async function expectError(label: string, run: () => Promise<unknown>, expected: string): Promise<void> {
  try {
    await run()
    check(false, `${label} (no error thrown)`)
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error)
    check(message.toLowerCase().includes(expected.toLowerCase()), `${label} → "${message}"`)
  }
}

async function main(): Promise<void> {
  const backend = new DemoBackend()

  console.log('create')
  await expectError('short master password rejected', () => backend.createVault('short', ''), 'at least 10')
  const status = await backend.createVault('correct horse battery staple', 'first pet')
  check(status.exists && status.unlocked, 'vault created and unlocked')
  check((status.info?.hint ?? '') === 'first pet', 'hint stored')
  check(status.itemCount > 0, 'sample items seeded for the preview')

  console.log('items')
  const all = await backend.listItems({ query: '', type: '', tag: '', favoritesOnly: false, sort: 'title' })
  check(all.length === status.itemCount, 'list returns every item')
  const search = await backend.listItems({ query: 'router', type: '', tag: '', favoritesOnly: false, sort: 'title' })
  check(search.length === 1 && search[0]?.title === 'Router admin', 'search filters')
  const loginOnly = await backend.listItems({ query: '', type: 'login', tag: '', favoritesOnly: false, sort: 'title' })
  check(loginOnly.every((item) => item.type === 'login'), 'type filter')
  const favourites = await backend.listItems({ query: '', type: '', tag: '', favoritesOnly: true, sort: 'title' })
  check(favourites.every((item) => item.favorite), 'favorite filter')
  check(all.every((summary) => !('password' in summary)), 'summaries never carry passwords')

  const created = await backend.saveItem({ type: 'login', title: 'Example', username: 'ada', password: 'first-password' } as Item)
  check(created.id !== '' && created.history?.length === 0, 'new item gets an id and no history')
  const fetched = await backend.getItem(created.id)
  check(fetched.password === 'first-password', 'secrets round-trip through getItem')

  await expectError('blank title rejected', () => backend.saveItem({ type: 'login', title: '   ' } as Item), 'title is required')
  await expectError('too short totp secret rejected', () => backend.saveItem({ type: 'login', title: 'x', totp: 'NOPE' } as Item), 'too short')
  await expectError('secret with too few bytes rejected', () => backend.saveItem({ type: 'login', title: 'x', totp: 'NOTASECRET' } as Item), 'too short')

  console.log('history')
  let rotated = { ...fetched, password: 'second-password' }
  rotated = await backend.saveItem(rotated)
  check(rotated.history?.[0]?.password === 'first-password', 'rotated password archived')
  rotated = await backend.saveItem({ ...rotated, password: 'third-password' })
  check(rotated.history?.[0]?.password === 'second-password', 'history is newest first')

  console.log('tags and stats')
  const tagged = await backend.saveItem({ ...rotated, tags: ['work', 'work', ' Personal '] })
  check(JSON.stringify(tagged.tags) === JSON.stringify(['Personal', 'work']), 'tags trimmed, de-duplicated and sorted')
  const tags = await backend.tags()
  check(tags.find((tag) => tag.name === 'work')?.count === 2, 'tag counts include every item that uses the tag')
  check(tags.every((tag) => tag.name === tag.name.trim()), 'stored tags carry no stray whitespace')
  const stats = await backend.stats()
  check(stats.total === status.itemCount + 1, 'stats count every item')
  check(stats.withTOTP >= 2, 'totp items counted')

  console.log('generator and strength')
  const generated = await backend.generatePassword({
    mode: 'password', length: 24, uppercase: true, lowercase: true, digits: true, symbols: true,
    excludeAmbiguous: true, requireEverySelected: true, words: 6, separator: '-', capitalize: true, appendNumber: true,
  })
  check(generated.value.length === 24, 'generated password has the requested length')
  check(/[A-Z]/.test(generated.value) && /[a-z]/.test(generated.value) && /[0-9]/.test(generated.value) && /[^A-Za-z0-9]/.test(generated.value), 'generated password uses every selected class')
  check(!/[Il1O0o|]/.test(generated.value), 'ambiguous characters excluded')
  check(generated.strength.score >= 3, 'generated password scores well')
  const passphrase = await backend.generatePassword({
    mode: 'passphrase', length: 20, uppercase: true, lowercase: true, digits: true, symbols: true,
    excludeAmbiguous: true, requireEverySelected: true, words: 6, separator: '-', capitalize: true, appendNumber: true,
  })
  check(passphrase.value.split('-').length === 7, 'passphrase has six words plus a digit')
  check((await backend.scorePassword('password')).score === 0, 'common password scores 0')
  check((await backend.scorePassword('vE7#kQ2$mZ9!pL4&xR6@wT1')).score >= 3, 'random password scores high')

  console.log('totp')
  const totpItem = all.find((summary) => summary.hasTOTP)
  check(Boolean(totpItem), 'a sample item has a totp secret')
  if (totpItem) {
    const code = await backend.totpCode(totpItem.id)
    check(/^[0-9]{6}$/.test(code.code), `totp produces six digits (${code.code})`)
    check(code.remaining >= 1 && code.remaining <= 30, 'totp countdown is in range')
  }

  console.log('locking')
  await expectError('wrong password rejected', async () => {
    await backend.lock()
    return backend.unlock('not the password')
  }, 'invalid master password')
  await backend.lock()
  await expectError('locked vault refuses reads', () => backend.getItem(created.id), 'vault is locked')
  await backend.unlock('correct horse battery staple')
  const afterUnlock = await backend.getItem(created.id)
  check(afterUnlock.password === 'third-password', 'data survives a lock/unlock cycle')
  check(afterUnlock.history?.length === 2, 'history survives a lock/unlock cycle')

  console.log('master password rotation')
  await expectError('rotation needs the current password', () => backend.changeMasterPassword('wrong password here', 'a brand new passphrase'), 'invalid master password')
  await backend.changeMasterPassword('correct horse battery staple', 'a brand new passphrase')
  await backend.lock()
  await expectError('old password no longer works', () => backend.unlock('correct horse battery staple'), 'invalid master password')
  await backend.unlock('a brand new passphrase')
  check((await backend.getItem(created.id)).password === 'third-password', 'data intact after rotation')

  console.log('csv import')
  const bitwardenCsv = [
    'folder,favorite,type,name,notes,fields,reprompt,login_uri,login_username,login_password,login_totp',
    'Work,1,login,Imported GitHub,primary dev,,,https://github.com,ada,pw-imported,JBSWY3DPEHPK3PXP',
    'Personal,0,login,Imported Fastmail,mail,,,https://fastmail.com,ada@example.com,pw-imported-two,',
    'Personal,0,note,Imported note,renew in 2031,,,,,,',
    'Personal,0,card,Imported card,,,,,,,',
  ].join('\n')

  const beforeImport = (await backend.stats()).total
  const preview = await backend.importCsv(bitwardenCsv, { dryRun: true, mode: 'skip' })
  check(preview.format === 'Bitwarden', `format detected (${preview.format})`)
  check(preview.new === 3 && preview.unsupported === 1, 'a dry run classifies every row')
  check(preview.imported === 0, 'a dry run writes nothing')
  check((await backend.stats()).total === beforeImport, 'the vault is untouched after a preview')

  const applied = await backend.importCsv(bitwardenCsv, { dryRun: false, mode: 'skip' })
  check(applied.imported === 3, 'the importable rows land in the vault')
  check((await backend.stats()).total === beforeImport + 3, 'the vault grew by the imported rows')
  const importedList = await backend.listItems({ query: 'Imported GitHub', type: '', tag: '', favoritesOnly: false, sort: 'title' })
  const importedItem = await backend.getItem(importedList[0]?.id ?? '')
  check(importedItem.password === 'pw-imported' && importedItem.url === 'https://github.com', 'imported fields round-trip')
  check(importedItem.favorite === true, 'the favorite flag carries over')
  check(JSON.stringify(importedItem.tags) === JSON.stringify(['Work']), 'the folder becomes a tag')
  check(importedItem.totp === 'JBSWY3DPEHPK3PXP', 'the totp secret carries over')
  check(/^[0-9]{6}$/.test((await backend.totpCode(importedItem.id)).code), 'an imported totp secret produces codes')

  const reimport = await backend.importCsv(bitwardenCsv, { dryRun: true, mode: 'skip' })
  check(reimport.duplicates === 3, 'importing the same file again is detected as duplicates')
  const overwrite = await backend.importCsv(bitwardenCsv, { dryRun: false, mode: 'add' })
  check(overwrite.imported === 3, 'add mode keeps duplicates')
  await expectError('an unusable header is rejected', () => backend.importCsv('id,label\n1,x\n', { dryRun: true, mode: 'skip' }), 'needs at least a title')
  await expectError('an empty file is rejected', () => backend.importCsv('   ', { dryRun: true, mode: 'skip' }), 'empty')

  console.log('security audit')
  const baseline = await backend.audit()
  check(baseline.score >= 0 && baseline.score <= 100, `score in range (${baseline.score})`)
  check(baseline.items === (await backend.stats()).total, 'every live item is checked')
  const weakA = await backend.saveItem({ type: 'login', title: 'Weak A', username: 'a', url: 'https://a.example', password: 'letmein' } as Item)
  const weakB = await backend.saveItem({ type: 'login', title: 'Weak B', username: 'b', url: 'https://b.example', password: 'letmein' } as Item)
  const report = await backend.audit()
  check(report.weak >= 2, 'weak passwords are flagged')
  check(report.reused >= 2, 'reused passwords are flagged')
  check(report.noTotp >= 2, 'logins with a URL and no second factor are flagged')
  check(report.findings.some((finding) => finding.kind === 'weak' && finding.severity === 'high'), 'a breached password is high severity')
  check(report.findings.some((finding) => finding.kind === 'reused' && finding.itemId === weakA.id), 'findings name the item they belong to')
  check(report.score < baseline.score, 'findings lower the score')
  check(report.affected >= 2, 'affected items are counted once each')
  const order = { high: 0, medium: 1, low: 2 } as const
  const sorted = report.findings.every((finding, index) => {
    const previous = report.findings[index - 1]
    return index === 0 || order[previous!.severity] <= order[finding.severity]
  })
  check(sorted, 'findings are ordered by severity')

  console.log('trash')
  await backend.deleteItem(created.id)
  const afterDelete = await backend.listItems({ query: '', type: '', tag: '', favoritesOnly: false, sort: 'title' })
  check(!afterDelete.some((item) => item.id === created.id), 'a deleted item leaves the live list')
  await expectError('a trashed item cannot be read', () => backend.getItem(created.id), 'item not found')
  const trashed = await backend.trashList()
  check(trashed.length === 1 && trashed[0]?.id === created.id, 'the item is in the trash')
  check(Boolean(trashed[0]?.deletedAt), 'the trashed item records when it was deleted')
  check((await backend.stats()).trashed === 1, 'the trash is counted separately from live items')

  const restored = await backend.restoreItem(created.id)
  check(restored.title === 'Example' && !restored.deletedAt, 'restore brings the item back')
  check((await backend.trashList()).length === 0, 'the trash is empty again')
  check((await backend.getItem(created.id)).password === 'third-password', 'a restored item keeps its secrets')

  const liveBeforeTrashing = (await backend.stats()).total
  await backend.deleteItem(created.id)
  await backend.deleteItem(weakA.id)
  check((await backend.stats()).total === liveBeforeTrashing - 2, 'trashing removes items from the live count')
  check((await backend.emptyTrash()) === 2, 'emptying the trash reports how many items went')
  check((await backend.trashList()).length === 0, 'the trash is empty afterwards')
  check((await backend.stats()).total === liveBeforeTrashing - 2, 'emptied items are gone for good')
  await backend.deleteItem(weakB.id)
  await backend.deleteForever(weakB.id)
  check((await backend.trashList()).length === 0, 'delete forever skips the trash')
  await expectError('deleting a missing item still fails', () => backend.deleteItem(weakB.id), 'item not found')

  console.log('storage is ciphertext')
  const raw = store.get('coffer.preview.vault') ?? ''
  check(raw.length > 0, 'vault persisted to storage')
  for (const secret of [
    'correct horse battery staple',
    'a brand new passphrase',
    'third-password',
    'Example',
    'Router admin',
    'pw-imported',
    'Imported GitHub',
    'letmein',
  ]) {
    check(!raw.includes(secret), `storage does not contain ${JSON.stringify(secret)}`)
  }
  const parsed = JSON.parse(raw) as { meta: { kdf: { salt: string } }; body: string }
  check((parsed.meta.kdf.salt ?? '').length > 0, 'salt stored alongside ciphertext')

  console.log('reset')
  await backend.resetPreview()
  check((await backend.status()).exists === false, 'reset forgets the preview vault')

  // The preview keeps an auto-lock interval alive; stop it so Node can exit.
  backend.dispose()
}

main()
  .then(() => {
    console.log(`\n${checks - failures}/${checks} checks passed`)
    process.exit(failures > 0 ? 1 : 0)
  })
  .catch((error: unknown) => {
    console.error('smoke test crashed:', error)
    process.exit(1)
  })
