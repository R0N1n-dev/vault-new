package vault

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const testPassword = "correct horse battery staple"

func newTestVault(t *testing.T) *Vault {
	t.Helper()
	v := New(filepath.Join(t.TempDir(), "vault.coffer"))
	t.Cleanup(v.Lock)
	return v
}

func TestCreateUnlockRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.coffer")
	v := New(path)
	if err := v.Create(testPassword, "first pet"); err != nil {
		t.Fatalf("create: %v", err)
	}
	created, err := v.Save(Item{Type: TypeLogin, Title: "GitHub", Username: "ada", Password: "hunter2", Tags: []string{"work"}})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected an assigned id")
	}
	v.Lock()

	fresh := New(path)
	if err := fresh.Unlock(testPassword); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	got, err := fresh.Get(created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Password != "hunter2" || got.Username != "ada" || got.Title != "GitHub" {
		t.Fatalf("round trip mismatch: %+v", got)
	}

	info, err := fresh.Info()
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if info.Hint != "first pet" || info.MemoryMiB != 64 || info.Passes != 3 || info.Parallel != 4 {
		t.Fatalf("unexpected info: %+v", info)
	}
	if info.VaultID == "" || info.Revision < 2 {
		t.Fatalf("unexpected info: %+v", info)
	}
}

func TestUnlockWithWrongPassword(t *testing.T) {
	v := newTestVault(t)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	v.Lock()
	if err := v.Unlock("not the password"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("want ErrWrongPassword, got %v", err)
	}
	if v.Unlocked() {
		t.Fatal("vault must stay locked after a failed unlock")
	}
}

func TestMasterPasswordMinimumLength(t *testing.T) {
	v := newTestVault(t)
	if err := v.Create("short", ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("want ErrInvalid for a short master password, got %v", err)
	}
}

func TestCreateRefusesToOverwrite(t *testing.T) {
	v := newTestVault(t)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	second := New(v.Path())
	if err := second.Create("another master password", ""); !errors.Is(err, ErrExists) {
		t.Fatalf("want ErrExists, got %v", err)
	}
}

func TestFilePermissionsAreOwnerOnly(t *testing.T) {
	v := newTestVault(t)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	info, err := os.Stat(v.Path())
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("want 0600 permissions, got %o", perm)
	}
}

func TestContainerNeverStoresPlaintext(t *testing.T) {
	v := newTestVault(t)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := v.Save(Item{Type: TypeLogin, Title: "Bank", Username: "ada@example.com", Password: "s3cr3t-unique-string", Notes: "do not leak"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	raw, err := os.ReadFile(v.Path())
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	for _, secret := range []string{testPassword, "s3cr3t-unique-string", "ada@example.com", "do not leak", "Bank"} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatalf("vault file leaks %q", secret)
		}
	}
	if !bytes.HasPrefix(raw, magic) {
		t.Fatal("expected the container magic prefix")
	}
}

func TestTamperedPayloadIsRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.coffer")
	v := New(path)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	v.Lock()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	raw[len(raw)-5] ^= 0x01 // flip one bit of the ciphertext
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	fresh := New(path)
	err = fresh.Unlock(testPassword)
	if err == nil {
		t.Fatal("expected the tampered vault to be rejected")
	}
	if errors.Is(err, ErrWrongPassword) {
		t.Fatalf("want a corruption error, got %v", err)
	}
}

func TestTamperedKDFParamsAreRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.coffer")
	v := New(path)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	v.Lock()

	header, payload, err := parseContainer(mustRead(t, path))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Downgrade the Argon2 cost, as an attacker would in order to make an
	// offline attack cheaper. The header is authenticated, so this must fail.
	header.KDF.Memory = 8 * 1024
	header.KDF.Time = 1
	raw, err := marshalContainer(header, payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := New(path).Unlock(testPassword); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("want ErrWrongPassword after a KDF downgrade, got %v", err)
	}
}

func TestOutOfRangeKDFParamsAreRejectedBeforeDeriving(t *testing.T) {
	base := KDFParams{Alg: kdfAlg, Version: 19, Time: 3, Memory: 64 * 1024, Threads: 4, KeyLen: KeyLen, Salt: make([]byte, saltLen)}
	cases := []struct {
		name   string
		mutate func(*KDFParams)
	}{
		{"memory too large", func(k *KDFParams) { k.Memory = 4 << 20 }},
		{"memory too small", func(k *KDFParams) { k.Memory = 1 }},
		{"time too large", func(k *KDFParams) { k.Time = 4096 }},
		{"no salt", func(k *KDFParams) { k.Salt = nil }},
		{"wrong key length", func(k *KDFParams) { k.KeyLen = 16 }},
		{"wrong algorithm", func(k *KDFParams) { k.Alg = "scrypt" }},
		{"wrong version", func(k *KDFParams) { k.Version = 16 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := base
			tc.mutate(&params)
			if err := params.validate(); !errors.Is(err, ErrCorrupt) {
				t.Fatalf("want ErrCorrupt, got %v", err)
			}
		})
	}
}

func TestRejectsForeignAndTruncatedFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.coffer")
	if err := os.WriteFile(path, []byte("this is just a text file"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := New(path).Unlock(testPassword); !errors.Is(err, ErrNotAVault) {
		t.Fatalf("want ErrNotAVault, got %v", err)
	}

	truncated := filepath.Join(dir, "truncated.coffer")
	if err := os.WriteFile(truncated, append([]byte{}, magic...), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := New(truncated).Unlock(testPassword); !errors.Is(err, ErrNotAVault) {
		t.Fatalf("want ErrNotAVault for a header-less file, got %v", err)
	}
}

func TestSavesReuseNoNonce(t *testing.T) {
	v := newTestVault(t)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	seen := map[string]bool{}
	for i := range 5 {
		if _, err := v.Save(Item{Type: TypeNote, Title: "note", Notes: string(rune('a' + i))}); err != nil {
			t.Fatalf("save: %v", err)
		}
		header, payload, err := parseContainer(mustRead(t, v.Path()))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if seen[string(header.PayloadNonce)] {
			t.Fatal("payload nonce was reused across saves")
		}
		seen[string(header.PayloadNonce)] = true
		if header.Revision < uint64(i+1) {
			t.Fatalf("revision did not advance: %d", header.Revision)
		}
		if len(payload) == 0 {
			t.Fatal("expected payload ciphertext")
		}
	}
}

func TestAtomicWriteKeepsPreviousRevision(t *testing.T) {
	v := newTestVault(t)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	first := mustRead(t, v.Path())
	if _, err := v.Save(Item{Type: TypeNote, Title: "second revision"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	backup, err := os.ReadFile(v.BackupPath())
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if !bytes.Equal(backup, first) {
		t.Fatal("backup should hold the previous revision")
	}
	// No temporary files may be left behind.
	entries, err := os.ReadDir(filepath.Dir(v.Path()))
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".coffer-") {
			t.Fatalf("temporary file left behind: %s", entry.Name())
		}
	}
}

func TestLockedVaultRefusesEveryOperation(t *testing.T) {
	v := newTestVault(t)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	item, err := v.Save(Item{Type: TypeLogin, Title: "example", Password: "secret"})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	v.Lock()
	if v.Unlocked() {
		t.Fatal("expected a locked vault")
	}
	checks := map[string]error{}
	_, checks["list"] = v.List(ListOptions{})
	_, checks["get"] = v.Get(item.ID)
	_, checks["save"] = v.Save(item)
	_, checks["tags"] = v.Tags()
	_, checks["stats"] = v.Stats()
	_, checks["settings"] = v.Settings()
	_, checks["count"] = v.Count()
	checks["delete"] = v.Delete(item.ID)
	checks["settings"] = func() error { _, err := v.SaveSettings(DefaultSettings()); return err }()
	checks["hint"] = v.SetHint("hello")
	checks["rotate"] = v.ChangeMasterPassword(testPassword, "another password here")
	for name, err := range checks {
		if !errors.Is(err, ErrLocked) {
			t.Fatalf("%s: want ErrLocked, got %v", name, err)
		}
	}
}

func TestChangeMasterPasswordKeepsDataAndSwapsSalt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.coffer")
	v := New(path)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	item, err := v.Save(Item{Type: TypeLogin, Title: "Mail", Password: "keep-me"})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	saltBefore := append([]byte(nil), v.header.KDF.Salt...)
	const next = "an entirely different passphrase"
	if err := v.ChangeMasterPassword(testPassword, next); err != nil {
		t.Fatalf("change: %v", err)
	}
	if bytes.Equal(saltBefore, v.header.KDF.Salt) {
		t.Fatal("expected a fresh salt after rotation")
	}
	// The old password must be useless and the wrapped key must have changed.
	if err := v.ChangeMasterPassword(testPassword, "yet another passphrase"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("old password still works: %v", err)
	}
	v.Lock()
	if err := New(path).Unlock(testPassword); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("old password unlocked the vault: %v", err)
	}
	fresh := New(path)
	if err := fresh.Unlock(next); err != nil {
		t.Fatalf("unlock with the new password: %v", err)
	}
	got, err := fresh.Get(item.ID)
	if err != nil {
		t.Fatalf("get after rotation: %v", err)
	}
	if got.Password != "keep-me" {
		t.Fatalf("data did not survive rotation: %+v", got)
	}
}

func TestExportProducesAnUnlockableCopy(t *testing.T) {
	dir := t.TempDir()
	v := New(filepath.Join(dir, "vault.coffer"))
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := v.Save(Item{Type: TypeNote, Title: "exported"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	dest := filepath.Join(dir, "backup.coffer")
	if err := v.ExportTo(dest); err != nil {
		t.Fatalf("export: %v", err)
	}
	if err := v.ExportTo(v.Path()); !errors.Is(err, ErrInvalid) {
		t.Fatalf("want ErrInvalid when exporting onto itself, got %v", err)
	}
	copyVault := New(dest)
	if err := copyVault.Unlock(testPassword); err != nil {
		t.Fatalf("unlock export: %v", err)
	}
	items, err := copyVault.List(ListOptions{})
	if err != nil {
		t.Fatalf("list export: %v", err)
	}
	if len(items) != 1 || items[0].Title != "exported" {
		t.Fatalf("unexpected export contents: %+v", items)
	}
}

func TestBackupPathIsReported(t *testing.T) {
	v := newTestVault(t)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	if info, err := v.Info(); err != nil || info.Backup {
		t.Fatalf("no backup should exist yet: %+v %v", info, err)
	}
	if _, err := v.Save(Item{Type: TypeNote, Title: "again"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	info, err := v.Info()
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if !info.Backup {
		t.Fatal("expected the backup flag to be set once a .bak file exists")
	}
}

func TestCreateNotAVaultErrorForMissingFile(t *testing.T) {
	v := New(filepath.Join(t.TempDir(), "missing.coffer"))
	if v.Exists() {
		t.Fatal("expected no vault file")
	}
	if err := v.Unlock(testPassword); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if _, err := v.Info(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound from Info, got %v", err)
	}
}

func TestContainerRejectsOversizedHeader(t *testing.T) {
	raw := append([]byte{}, magic...)
	raw = binary.BigEndian.AppendUint32(raw, maxHeaderLen+1)
	raw = append(raw, make([]byte, 32)...)
	if _, _, err := parseContainer(raw); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("want ErrCorrupt, got %v", err)
	}
}

func TestPayloadVersionFromTheFutureIsRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.coffer")
	v := New(path)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	// Re-seal a payload claiming a newer format version using the live data key.
	future, err := json.Marshal(Payload{Version: FormatVersion + 1, Items: []Item{}, Settings: DefaultSettings()})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body, err := seal(v.dek, future, v.header.payloadAAD())
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	v.header.PayloadNonce = body.Nonce
	if err := v.write(body.Data); err != nil {
		t.Fatalf("write: %v", err)
	}
	v.Lock()
	if err := New(path).Unlock(testPassword); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("want ErrCorrupt for a future payload version, got %v", err)
	}
}

func TestConcurrentAccessIsRaceFree(t *testing.T) {
	v := newTestVault(t)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	item, err := v.Save(Item{Type: TypeLogin, Title: "shared", Password: "one"})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			switch n % 4 {
			case 0:
				_, _ = v.List(ListOptions{})
			case 1:
				_, _ = v.Get(item.ID)
			case 2:
				_, _ = v.Tags()
				_, _ = v.Stats()
			case 3:
				_, _ = v.Save(Item{ID: item.ID, Type: TypeLogin, Title: "shared", Password: "two"})
			}
		}(i)
	}
	wg.Wait()
}

func TestTimeStampsAreUTCSeconds(t *testing.T) {
	v := newTestVault(t)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	item, err := v.Save(Item{Type: TypeNote, Title: "stamped"})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if item.CreatedAt.Location() != time.UTC || item.UpdatedAt.Nanosecond() != 0 {
		t.Fatalf("timestamps must be UTC second precision: %+v", item)
	}
	if time.Since(item.CreatedAt) > time.Minute {
		t.Fatalf("unexpected timestamp: %v", item.CreatedAt)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return raw
}
