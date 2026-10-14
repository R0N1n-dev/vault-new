# Coffer

A local-first password vault: **Wails v2** (Go) + **Vue 3** + **TypeScript**.

One encrypted file on your machine. No account, no sync, no telemetry, no network
calls. Everything — passwords, notes, card numbers, TOTP seeds — is encrypted
with XChaCha20-Poly1305 under a key derived from your master password with
Argon2id.

```
┌──────────────┐        bindings        ┌──────────────────────────────┐
│  Vue 3 UI    │ ◄────────────────────► │  Go (internal/vault)         │
│  (webview)   │                        │  Argon2id → KEK → DEK → AEAD │
└──────────────┘                        └───────────────┬──────────────┘
                                                        │
                                              vault.coffer (one file)
```

## Features

- **Logins, secure notes and cards** with tags, favourites, custom fields and
  automatic password history (previous values are encrypted, not discarded).
- **TOTP codes** generated locally from a stored secret or an `otpauth://` link,
  with a live countdown ring — the seed never leaves the Go side.
- **Password generator**: random passwords (class-by-class, look-alike filter) or
  six-word passphrases from the EFF short word list (~65 bits).
- **Offline strength meter** that folds leetspeak and sequence patterns before
  scoring, so `p@ssw0rd` scores zero.
- **Auto-lock** enforced in Go (not just by a JS timer), clipboard clearing that
  never clobbers something you copied in the meantime, reveal-on-demand secrets.
- **Search** across titles, usernames, URLs, tags, notes and custom field labels;
  filter by type, tag, favourites; sort by title or recency.
- **Full keyboard driving**: `Ctrl/Cmd+K` search, `Ctrl/Cmd+N` new item,
  `Ctrl/Cmd+L` lock, `Ctrl/Cmd+G` generator, `Ctrl/Cmd+,` settings, `↑`/`↓` to
  move, `E` to edit, `Esc` to clear the selection.
- **Security check**: an offline health report over the unlocked vault — weak,
  reused and stale passwords, logins with no second factor, expiring cards —
  scored out of 100 with findings that jump straight to the item. No breach-corpus
  lookup, no hashing against a remote service, no network request of any kind.
- **Trash with a safety net**: deleting moves an item to the trash instead of
  shredding it, so a misclick is recoverable. Restore it, delete it for good, or
  let the retention window (30 days by default, configurable per vault) sweep it.
- **CSV import** from Bitwarden, 1Password, KeePass/KeePassXC, Chrome/Edge or any
  spreadsheet: columns are matched by name, folders become tags, `otpauth://`
  seeds are reduced to their secret, and a dry-run preview shows exactly which
  rows are new, already present or unusable before anything is written.
- **Encrypted export / import** plus a rolling `.bak` of the previous revision.
- Dark and light themes (or follow the system), persisted for the unlock screen.

## Requirements

- **Go 1.21+** (the go.mod targets 1.25)
- **Node 15+** (built and tested with Node 24)
- **Wails v2 CLI**: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0`
- Platform dependencies for Wails:
  - **Linux**: `libgtk-3-dev` and `libwebkit2gtk-4.1-dev` (Wails checks these with
    `wails doctor`)
  - **Windows**: WebView2 runtime (preinstalled on Windows 11)
  - **macOS**: Xcode command line tools

## Running it

```sh
# live development (starts the Vite dev server and the native window)
wails dev

# production build into build/bin/
wails build
```

The vault is created at the user configuration directory:

| Platform | Location |
| --- | --- |
| Linux | `~/.config/coffer/vault.coffer` |
| macOS | `~/Library/Application Support/coffer/vault.coffer` |
| Windows | `%AppData%\coffer\vault.coffer` |

Set `COFFER_VAULT_PATH` to keep the vault somewhere else (a USB stick, a synced
folder, a portable install).

## Security model

```
master password ──Argon2id(salt, m=64 MiB, t=3, p=4)──► KEK
                                                         │ wraps (XChaCha20-Poly1305)
                                                         ▼
                                     random 32-byte DEK ──encrypts──► vault payload
```

- **Key hierarchy.** The master password derives a key-encryption key (KEK) that
  only ever wraps a random data-encryption key (DEK). The vault body is
  encrypted with the DEK. Changing the master password re-wraps the DEK and
  generates a fresh salt; the data is never re-encrypted with a
  password-derived key.
- **Cost parameters.** 64 MiB of memory, 3 passes and 4 lanes, following RFC 9106
  §4 and well above the OWASP minimums. Parameters live in the header, are
  authenticated, and are range-checked *before* any allocation so a hostile file
  cannot turn into a memory bomb.
- **Authenticated everything.** The header (format, vault id, KDF parameters,
  nonces) is bound to both ciphertexts as associated data, as are the revision
  counter, the timestamp and the hint. Downgrading the KDF cost, rewinding the
  revision or swapping nonces invalidates the tag.
- **Nonces.** XChaCha20-Poly1305's 24-byte random nonce is generated fresh on
  every single write, which is safe with random nonces and removes any counter
  bookkeeping bug class.
- **Atomic writes.** Saves go to a temporary file in the same directory, are
  `fsync`ed, then renamed over the vault, with the previous revision kept as
  `vault.coffer.bak`. A crash leaves either the old file or the new one, never a
  half-written one. New files are created `0600`.
- **Memory hygiene.** Keys are overwritten on lock; the DEK is `nil`ed. Secret
  strings themselves cannot be reliably zeroed in a garbage-collected runtime —
  see the threat model below.
- **Clipboard.** Copied secrets are cleared after the configured delay, and only
  if the clipboard still holds our value (compared by SHA-256, so the secret is
  not duplicated in a second buffer).
- **Locks.** Auto-lock runs on a Go ticker driven by activity pings from the UI,
  so it still fires when the webview throttles timers. Optional lock-on-blur.
- **Single instance.** A second launch hands over to the running window instead
  of racing on the same file.

### What this protects against, and what it does not

Protected: an attacker who obtains the vault file (stolen laptop, backup, cloud
folder) without the master password. Offline brute force pays the full Argon2id
cost per guess.

Not protected: malware or a keylogger on an unlocked machine, a privileged user
reading process memory, or someone using the app while it is unlocked. There is
**no recovery** for a forgotten master password — keep an encrypted export
somewhere safe. Rotation does not re-key old backups: an older `.bak` or export
still opens with the password that was current when it was written.

## Project layout

```
main.go                  Wails window options, single-instance lock
app.go                   Bindings exposed to the frontend (the whole UI API)
internal/vault/          The vault engine, with no UI dependencies
  crypto.go              Argon2id parameters, KEK/DEK, AEAD helpers
  container.go           File format: header, associated data, atomic writes
  vault.go               Lock/unlock/save, items, settings, rotation, export
  item.go                Item model, validation, search, sorting
  trash.go               Soft delete, restore, purge by retention window
  audit.go               Offline health report (weak/reused/stale/2FA/cards)
  csvimport.go           CSV import from other password managers
  generator.go           Password and passphrase generation
  strength.go            Offline strength heuristic
  totp.go                RFC 6238 one-time passwords (RFC vectors in the tests)
  wordlist.go            Generated from the EFF short word list (CC BY 3.0)
frontend/                Vue 3 + TypeScript UI
  src/lib/backend.ts     The seam: Backend interface + error mapping
  src/lib/nativeBackend.ts  Implementation over the Wails bindings
  src/lib/demoBackend.ts    Browser-preview implementation (see below)
  src/stores/session.ts  Reactive session state and actions
  src/lib/demoCsv.ts     CSV import parity for the browser preview
  src/components/        List, detail, editor, audit, trash, import, TOTP…
  src/views/             Setup, unlock and vault screens
docs/FORMAT.md           Byte-level vault file format
```

## Browser preview (no Go required)

`frontend/src/lib/demoBackend.ts` implements the same `Backend` interface in the
browser, so the UI can be developed and reviewed without the Go toolchain:

```sh
cd frontend
npm install
npm run dev        # http://localhost:3004
```

It is a genuine encrypted store (PBKDF2-SHA256 → AES-256-GCM via WebCrypto, same
header/wrapped-key shape) but it is **not** the product: it keeps the vault in
`localStorage`, uses a different KDF and ships sample items. The app labels this
loudly with a banner, offers a "Reset preview" button, and the packaged desktop
build never touches it (it activates only when the Wails bindings are absent).

## Tests

```sh
# Go engine: round trips, tamper detection, KDF parameter bounds, RFC 6238
# vectors, item validation, search, history, concurrency, plus the trash,
# audit and CSV-import suites
go test ./... && go vet ./...          # add -race for the concurrency test
go test -cover ./internal/...

# Frontend: types, then the production bundle
cd frontend && npm run typecheck && npm run build

# The browser-preview backend, end to end (88 checks)
npm run smoke

# The new panes rendered for real against that backend (30 checks): catches a
# template that throws at runtime, which typechecking and bundling cannot
npm run render-check
```

`wails build` additionally needs the platform webview development packages — on
Linux `libgtk-3-dev` and `libwebkit2gtk-4.1-dev` — because the desktop frontend
links against them through cgo. Without those, `wails build` fails in the
`pkg-config` step after compiling the frontend, while `go test`, `go vet` and the
browser preview all still work.

## Keyboard reference

| Keys | Action |
| --- | --- |
| `Ctrl/Cmd + K` | Focus search |
| `Ctrl/Cmd + N` | New item |
| `Ctrl/Cmd + G` | Password generator |
| `Ctrl/Cmd + L` | Lock the vault |
| `Ctrl/Cmd + ,` | Settings |
| `↑` / `↓` | Previous / next item |
| `E` | Edit the selected item |
| `Esc` | Clear the selection, or close a dialog |

## Limitations / next steps

- No sync or multi-device story by design: it is one file. Point
  `COFFER_VAULT_PATH` at a synced folder if you want that, and accept the
  conflict semantics.
- No browser extension or autofill (that needs a native messaging host).
- CSV import is one-way: there is no CSV *export*, because writing every
  password to a plain-text file is a downgrade. Move data out with the encrypted
  export, or per-item copy.
- The security check is heuristic: it flags weak, reused and stale passwords and
  missing second factors, but it does not know whether a password has appeared
  in a breach (that would require a network call). The container format is
  documented so a converter is straightforward.
- Argon2id parameters are fixed per vault, so a slow machine pays ~0.3–1 s per
  unlock. Re-benchmarking on first run is on the roadmap.
- Linux packaging (`-tags webkit2_41`, AppImage/deb) is left to `wails build`
  with the platform's webkit2gtk version installed.

## Credits and licences

- Passphrase word list: [EFF short word list](https://www.eff.org/dice), CC BY 3.0.
- UI font: Nunito, SIL Open Font License (see `frontend/src/assets/fonts/OFL.txt`).
- Icons are hand-written SVG paths in `frontend/src/lib/icons.ts`.
