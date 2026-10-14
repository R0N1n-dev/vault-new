package vault

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
)

// magic identifies a Coffer container. The trailing newline keeps the header
// length readable when someone looks at the file with a text editor.
var magic = []byte("COFFER1\n")

// maxHeaderLen bounds the header so a hostile file cannot make us allocate a
// huge buffer before we have validated anything.
const maxHeaderLen = 64 * 1024

// Header is the cleartext, authenticated part of a vault file. It contains no
// secrets: the KDF cost parameters and a random salt are public by definition,
// and both blobs are ciphertext.
type Header struct {
	Format    int       `json:"format"`
	VaultID   string    `json:"vaultId"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Revision  uint64    `json:"revision"`
	Hint      string    `json:"hint,omitempty"`
	KDF       KDFParams `json:"kdf"`
	AEAD      string    `json:"aead"`
	// KeyWrap holds the vault data key encrypted under the key-encryption key.
	KeyWrap Blob `json:"keyWrap"`
	// PayloadNonce is the nonce for the encrypted vault body. The body itself
	// is stored once, after the header, so there is no stale copy to tamper
	// with: a modification there is caught by the AEAD tag.
	PayloadNonce []byte `json:"payloadNonce"`
}

// wrapMeta is the immutable subset of the header that the wrapped data key is
// bound to. Changing a KDF parameter, the format or the vault identity therefore
// invalidates the wrapped key.
type wrapMeta struct {
	Format    int       `json:"format"`
	VaultID   string    `json:"vaultId"`
	CreatedAt time.Time `json:"createdAt"`
	AEAD      string    `json:"aead"`
	KDF       KDFParams `json:"kdf"`
}

// payloadMeta adds the mutable bookkeeping fields to the associated data so
// that an attacker cannot roll back or forge the revision counter unnoticed.
type payloadMeta struct {
	wrapMeta
	Revision  uint64    `json:"revision"`
	UpdatedAt time.Time `json:"updatedAt"`
	Hint      string    `json:"hint,omitempty"`
}

func (h *Header) wrapAAD() []byte {
	return aad("kek-wrap", wrapMeta{
		Format:    h.Format,
		VaultID:   h.VaultID,
		CreatedAt: h.CreatedAt,
		AEAD:      h.AEAD,
		KDF:       h.KDF,
	})
}

func (h *Header) payloadAAD() []byte {
	return aad("payload", payloadMeta{
		wrapMeta: wrapMeta{
			Format:    h.Format,
			VaultID:   h.VaultID,
			CreatedAt: h.CreatedAt,
			AEAD:      h.AEAD,
			KDF:       h.KDF,
		},
		Revision:  h.Revision,
		UpdatedAt: h.UpdatedAt,
		Hint:      h.Hint,
	})
}

// aad serialises an associated-data scope. encoding/json emits struct fields in
// declaration order and time.Time base64/seconds deterministically, so both
// writer and reader derive identical bytes.
func aad(scope string, meta any) []byte {
	encoded, err := json.Marshal(meta)
	if err != nil {
		// Structs of primitives cannot fail to marshal; treat as programmer error.
		panic(fmt.Sprintf("vault: marshal aad: %v", err))
	}
	out := make([]byte, 0, len(scope)+len(encoded)+16)
	out = append(out, "coffer/v1/"...)
	out = append(out, scope...)
	out = append(out, '\n')
	return append(out, encoded...)
}

func (h *Header) validate() error {
	if h.Format != FormatVersion {
		return fmt.Errorf("%w: unknown container format %d", ErrCorrupt, h.Format)
	}
	if h.AEAD != aeadAlg {
		return fmt.Errorf("%w: unsupported cipher %q", ErrCorrupt, h.AEAD)
	}
	if h.AEAD != aeadAlg {
		return ErrCorrupt
	}
	if h.VaultID == "" {
		return fmt.Errorf("%w: missing vault id", ErrCorrupt)
	}
	if len(h.KeyWrap.Nonce) != chacha20poly1305.NonceSizeX || len(h.KeyWrap.Data) < chacha20poly1305.Overhead {
		return fmt.Errorf("%w: malformed wrapped key", ErrCorrupt)
	}
	if len(h.PayloadNonce) != chacha20poly1305.NonceSizeX {
		return fmt.Errorf("%w: malformed payload nonce", ErrCorrupt)
	}
	if err := h.KDF.validate(); err != nil {
		return err
	}
	return nil
}

// marshalContainer renders magic || headerLen || header || payload.
func marshalContainer(h *Header, payload []byte) ([]byte, error) {
	headerJSON, err := json.Marshal(h)
	if err != nil {
		return nil, fmt.Errorf("marshal header: %w", err)
	}
	if len(headerJSON) > maxHeaderLen {
		return nil, fmt.Errorf("header too large: %d bytes", len(headerJSON))
	}
	buf := new(bytes.Buffer)
	buf.Grow(len(magic) + 4 + len(headerJSON) + len(payload))
	buf.Write(magic)
	if err := binary.Write(buf, binary.BigEndian, uint32(len(headerJSON))); err != nil {
		return nil, err
	}
	buf.Write(headerJSON)
	buf.Write(payload)
	return buf.Bytes(), nil
}

// parseContainer splits a vault file into its header and payload ciphertext.
// The payload is everything after the header, and is exactly what the tag in
// the header's AEAD covers.
func parseContainer(raw []byte) (*Header, []byte, error) {
	if len(raw) < len(magic)+4 {
		return nil, nil, ErrNotAVault
	}
	if !bytes.Equal(raw[:len(magic)], magic) {
		return nil, nil, ErrNotAVault
	}
	rest := raw[len(magic):]
	headerLen := binary.BigEndian.Uint32(rest[:4])
	if headerLen == 0 || headerLen > maxHeaderLen {
		return nil, nil, fmt.Errorf("%w: bad header length %d", ErrCorrupt, headerLen)
	}
	if uint32(len(rest)-4) < headerLen {
		return nil, nil, fmt.Errorf("%w: header truncated", ErrCorrupt)
	}
	headerJSON := rest[4 : 4+headerLen]
	payload := rest[4+headerLen:]
	if len(payload) < chacha20poly1305.Overhead {
		return nil, nil, fmt.Errorf("%w: missing or truncated payload", ErrCorrupt)
	}
	var h Header
	if err := json.Unmarshal(headerJSON, &h); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	if err := h.validate(); err != nil {
		return nil, nil, err
	}
	return &h, payload, nil
}

// readFile reads a vault file with a sanity limit so that pointing the app at a
// multi-gigabyte file fails fast instead of exhausting memory.
func readFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%w: %s is a directory", ErrNotAVault, path)
	}
	const maxVaultBytes = 64 << 20 // 64 MiB is far beyond a realistic item count
	if info.Size() > maxVaultBytes {
		return nil, fmt.Errorf("%w: file is unexpectedly large (%d bytes)", ErrNotAVault, info.Size())
	}
	return os.ReadFile(path)
}

// writeFileAtomic writes data to path via a temporary file in the same
// directory followed by a rename, and keeps the previous revision as a .bak
// sibling. A crash therefore leaves either the old or the new file, never a
// half-written one.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create vault directory: %w", err)
	}
	if err := backupExisting(path); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".coffer-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		// No-op after a successful rename; makes the error paths tidy.
		_ = os.Remove(tmpName)
	}()
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("set vault permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write vault: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("flush vault: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close vault: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace vault: %w", err)
	}
	syncDir(dir)
	// Best effort: tighten permissions on a pre-existing file we just replaced.
	_ = os.Chmod(path, 0o600)
	return nil
}

func backupExisting(path string) error {
	src, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open existing vault: %w", err)
	}
	defer src.Close()
	dst, err := os.OpenFile(path+".bak", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create backup: %w", err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("write backup: %w", err)
	}
	return dst.Sync()
}

// syncDir flushes the directory entry so the rename survives a power loss on
// filesystems that need it. Failures are not fatal.
func syncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	defer d.Close()
	_ = d.Sync()
}
