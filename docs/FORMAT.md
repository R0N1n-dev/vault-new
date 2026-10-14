# The Coffer vault file format

A vault is a single file with one cleartext, authenticated header followed by the
encrypted payload. It is written by `internal/vault/container.go` and
`internal/vault/vault.go`.

```
offset  size    content
0       8       magic: "COFFER1\n"
8       4       header length, uint32 big-endian
12      N       header JSON (UTF-8, no trailing newline)
12+N    rest    payload ciphertext: XChaCha20-Poly1305(vault JSON)
```

The payload ciphertext lives **only** after the header. Its 24-byte nonce is in
the header. (An earlier revision of this format embedded the ciphertext in the
header JSON as well, which meant the trailing bytes were a second, unauthorised
copy — the layout above has exactly one copy, and its AEAD tag covers it.)

## Header

```json
{
  "format": 1,
  "vaultId": "b5d0a4b6-…",
  "createdAt": "2026-10-12T06:12:00Z",
  "updatedAt": "2026-10-12T06:31:41Z",
  "revision": 7,
  "hint": "first pet",
  "kdf": {
    "alg": "argon2id",
    "ver": 19,
    "time": 3,
    "memory": 65536,
    "threads": 4,
    "keyLen": 32,
    "salt": "…16 raw bytes, base64…"
  },
  "aead": "xchacha20-poly1305",
  "keyWrap": { "nonce": "…24 bytes…", "data": "…32 bytes + 16 tag…" },
  "payloadNonce": "…24 bytes…"
}
```

`[]byte` fields are base64 (Go's `encoding/json` default). `time` is the Argon2
pass count and `memory` is total KiB, matching `argon2.IDKey(password, salt,
time, memory, threads, keyLen)`.

## Key schedule

1. `KEK = Argon2id(masterPassword, kdf.salt, kdf.time, kdf.memory, kdf.threads, 32)`
2. `DEK = XChaCha20-Poly1305_Open(KEK, keyWrap, wrapAAD)` — a random 32-byte key
   created with the vault. A failed tag here is reported as "invalid master
   password or corrupted vault"; the two cases are deliberately
   indistinguishable.
3. `payload = XChaCha20-Poly1305_Open(DEK, {payloadNonce, trailing bytes}, payloadAAD)`

The payload JSON is `{"version":1,"items":[…],"settings":{…},"updatedAt":"…"}`.

`settings` carries the vault's own configuration (auto-lock, clipboard timeout,
theme, default generator options, password-history depth and the trash retention
window), so nothing about your setup leaks into a plaintext sidecar file.

Each item is the struct in `internal/vault/item.go`. Two fields are worth
knowing about when reading a file:

- `history`: the previous passwords and when they were retired, oldest last.
  An empty password field in a history entry is impossible; the array is capped
  by `settings.passwordHistoryMax`.
- `deletedAt`: present only while the item sits in the trash. A trashed item is
  an ordinary item with this stamp, still encrypted in the same payload — the
trash is a UI and API concept, not a second container. Items whose stamp is
  older than `settings.trashRetentionDays` are dropped on the next write
  (`internal/vault/trash.go`). `deletedAt` is not part of the associated data:
  it is protected by the payload AEAD like every other field.

### Associated data

```
wrapAAD    = "coffer/v1/kek-wrap\n" + JSON{format, vaultId, createdAt, aead, kdf}
payloadAAD = "coffer/v1/payload\n"  + JSON{format, vaultId, createdAt, aead, kdf, revision, updatedAt, hint}
```

Both are produced from Go structs, whose field order is fixed, so writer and
reader always derive identical bytes. Consequences:

- The KDF cost, cipher name, format version and vault identity are bound to the
  wrapped data key: an attacker cannot weaken Argon2id or swap headers between
  files without breaking the unwrap.
- `revision`, `updatedAt` and `hint` are bound to the payload: tampering with the
  revision counter cannot go unnoticed.

The header's mutable bookkeeping fields are therefore inside `payloadAAD` only,
which is why a save never needs the KEK: it always re-seals the payload with a
fresh nonce, and only re-wraps the data key when the KDF parameters change.

## Operations

| Operation | Effect on the file |
| --- | --- |
| Create | new salt, random DEK, empty payload, `revision = 1` |
| Save (any mutation) | re-encrypt payload with a fresh nonce, `revision++`, `updatedAt = now` |
| Rotate master password | unwrap with the old KEK, fresh salt, new KEK, re-wrap DEK, re-seal payload |
| Set hint | modify `hint` (inside `payloadAAD`), re-seal payload |
| Export | byte copy of the file; still opens with the password current at that time |

Every write is `os.CreateTemp` → write → `fsync` → `rename` over the target, with
the previous revision copied to `<path>.bak` first. The temporary file is created
`0600` and `chmod`ed again after the rename. On failure the temp file is removed
and the old vault stays in place.

## Validation on read

Before any allocation or key derivation:

- magic matches, `4 <= headerLen <= 65536`, both header and payload present
- `format == 1`, `aead == "xchacha20-poly1305"`
- `keyWrap.nonce` is 24 bytes, `keyWrap.data` is at least the 16-byte tag,
  `payloadNonce` is 24 bytes, the trailing payload is at least 16 bytes
- KDF: `alg == "argon2id"`, `ver == 19`, `1 <= time <= 16`,
  `8 MiB <= memory <= 1 GiB`, `1 <= threads <= 64`, `keyLen == 32`,
  `8 <= len(salt) <= 64`
- decrypted payload `version <= 1` (a newer file is rejected with a clear
  message rather than being misinterpreted)

Files larger than 64 MiB are refused before reading, and a file that is not a
Coffer vault at all reports `ErrNotAVault`.

## Forward compatibility

Bump `format` for any incompatible change. Adding optional header fields is safe
only while they are excluded from `wrapAAD`/`payloadAAD`; anything that must be
tamper-proof has to go into the associated data, which is a format change.
Readers are expected to reject unknown KDF algorithms or ciphers instead of
guessing.
