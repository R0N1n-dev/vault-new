/**
 * Render check for the new panes.
 *
 * `vue-tsc` proves the components typecheck and `vite build` proves they
 * compile, but neither catches a template that throws at runtime (a null
 * report, an unknown icon name, a helper that was never imported). This mounts
 * the real components against the browser-preview store under Node and asserts
 * the rendered HTML. Run with `npm run render-check`.
 */

import { createSSRApp } from 'vue'
import { renderToString } from 'vue/server-renderer'

// ---------------------------------------------------------------- browser --
// The same shims the smoke test uses, plus the DOM surface the theme helper and
// the toast host touch while components render.
const store = new Map<string, string>()

function installBrowser(): void {
  const element = {
    setAttribute: () => undefined,
    removeAttribute: () => undefined,
    style: {},
    dataset: {},
  }
  const documentStub = {
    documentElement: element,
    body: element,
    createElement: () => element,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
  }
  const globals = globalThis as unknown as Record<string, unknown>
  globals.window = globalThis
  globals.document = documentStub
  globals.localStorage = {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => void store.set(key, value),
    removeItem: (key: string) => void store.delete(key),
  }
  globals.matchMedia = () => ({
    matches: false,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
  })
}

installBrowser()

let failures = 0
let checks = 0

function check(condition: boolean, label: string): void {
  checks += 1
  if (condition) {
    console.log(`  ✓ ${label}`)
  } else {
    failures += 1
    console.error(`  ✗ ${label}`)
  }
}

/**
 * render returns the component's markup plus anything it teleported. The modal
 * shell renders into <body>, so without reading the teleport buffer a modal's
 * contents would look empty here.
 */
async function render(component: unknown): Promise<string> {
  const app = createSSRApp(component as never)
  const context: Record<string, unknown> = {}
  const html = await renderToString(app, context)
  const teleports = (context.teleports ?? {}) as Record<string, string>
  return [html, ...Object.values(teleports)].join('\n')
}

async function main(): Promise<void> {
  const session = await import('../src/stores/session')
  const { emptyItem } = await import('../src/types')
  const { default: AuditPanel } = await import('../src/components/AuditPanel.vue')
  const { default: TrashPanel } = await import('../src/components/TrashPanel.vue')
  const { default: ImportPanel } = await import('../src/components/ImportPanel.vue')
  const { default: Sidebar } = await import('../src/components/Sidebar.vue')

  console.log('prepare a preview vault through the store')
  await session.createVault('correct horse battery staple', 'first pet')
  check(session.unlocked.value, 'the preview vault is unlocked')

  // Two items sharing a breached password, so the audit has real findings.
  for (const [index, title] of ['Weak A', 'Weak B'].entries()) {
    const item = emptyItem('login')
    item.title = title
    item.username = `user${index}`
    item.url = 'https://weak.example'
    item.password = 'letmein'
    await session.saveItem(item)
  }
  const victim = session.items.value.find((entry) => entry.title === 'Router admin')
  check(Boolean(victim), 'the seeded sample vault has the item used for the trash check')
  if (victim) {
    session.requestDelete({ id: victim.id, title: victim.title })
    await session.confirmDelete()
  }

  console.log('audit panel')
  await session.refreshAudit()
  const report = session.audit.value
  check(report !== null, 'the store holds a report')
  check((report?.findings.length ?? 0) > 0, 'the report has findings')
  check((report?.reused ?? 0) >= 2, 'reuse was detected')
  const auditHtml = await render(AuditPanel)
  check(auditHtml.includes('Password health'), 'renders its title')
  check(report !== null && auditHtml.includes(String(report.score)), 'renders the score')
  check(auditHtml.includes('Weak passwords'), 'renders the check cards')
  check(auditHtml.includes('Weak A'), 'lists a finding by item title')
  check(auditHtml.includes('High'), 'labels severities')
  check(!auditHtml.includes('undefined'), 'no undefined leaked into the markup')

  console.log('trash panel')
  await session.refreshTrash()
  check(session.trash.value.length === 1, 'the store holds the trashed item')
  const trashHtml = await render(TrashPanel)
  check(trashHtml.includes('Deleted items'), 'renders its title')
  check(trashHtml.includes('Router admin'), 'lists the trashed item')
  check(trashHtml.includes('Restore'), 'offers a restore action')
  check(trashHtml.includes('days left'), 'shows the retention countdown')
  check(!trashHtml.includes('undefined'), 'no undefined leaked into the markup')

  console.log('trash panel when empty')
  if (victim) await session.restoreItem(victim.id)
  const emptyTrashHtml = await render(TrashPanel)
  check(emptyTrashHtml.includes('The trash is empty'), 'renders the empty state')

  console.log('import panel')
  session.openImport()
  await session.previewImport(
    [
      'folder,favorite,type,name,notes,fields,reprompt,login_uri,login_username,login_password,login_totp',
      'Work,1,login,Imported GitHub,primary,,,https://github.com,ada,pw-imported,JBSWY3DPEHPK3PXP',
      'Personal,0,note,Imported note,renew in 2031,,,,,,',
    ].join('\n'),
  )
  check(session.importPreview.value?.format === 'Bitwarden', 'the preview classifies the file')
  const importHtml = await render(ImportPanel)
  check(importHtml.includes('Import from CSV'), 'renders its title')
  check(importHtml.includes('Bitwarden'), 'names the detected format')
  check(importHtml.includes('Imported GitHub'), 'lists the parsed rows')
  check(importHtml.includes('Ready to import'), 'shows the counts')
  check(!importHtml.includes('undefined'), 'no undefined leaked into the markup')
  session.closeImport()
  check(session.importSource.value === '', 'closing the import drops the file text')

  console.log('committing the import')
  await session.previewImport('name,url,username,password\nRendered import,https://r.example,ada,pw-rendered\n')
  await session.commitImport('skip')
  check(
    session.items.value.some((item) => item.title === 'Rendered import'),
    'the imported item reaches the list the UI renders',
  )

  console.log('sidebar')
  // The score shown is the current one, which the import above may have moved.
  const currentScore = session.audit.value?.score
  check(currentScore !== undefined, 'the store has a current score')
  const sidebarHtml = await render(Sidebar)
  check(sidebarHtml.includes('Security check'), 'renders the security entry')
  check(sidebarHtml.includes('Trash'), 'renders the trash entry')
  check(currentScore !== undefined && sidebarHtml.includes(String(currentScore)), 'shows the health score')
}

main()
  .then(() => {
    console.log(`\n${checks - failures}/${checks} render checks passed`)
    process.exit(failures > 0 ? 1 : 0)
  })
  .catch((error: unknown) => {
    console.error('render check crashed:', error)
    process.exit(1)
  })
