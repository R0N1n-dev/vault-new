// Package vault implements Coffer's encrypted, offline-first password store.
//
// Security model (see docs/FORMAT.md):
//
//	master password --Argon2id--> KEK --wraps--> DEK --XChaCha20-Poly1305--> vault payload
//
// The key-encryption key (KEK) never leaves memory and is never stored. The
// data-encryption key (DEK) is random per vault and stored only wrapped by the
// KEK, which lets the master password be rotated without re-encrypting the data
// with a user-derived key.
package vault

import (
	"crypto/rand"
	"crypto/subtle"
	"fmt"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

const (
	// FormatVersion is the container format version written by this build.
	FormatVersion = 1

	// KeyLen is the length of every symmetric key used by a vault.
	KeyLen = 32

	saltLen = 16
	kdfAlg  = "argon2id"
	aeadAlg = "xchacha20-poly1305"
)

// Argon2id parameter bounds. They are enforced on read as well as on write so a
// tampered or hostile vault file cannot make us allocate gigabytes or spin for
// minutes before authentication fails.
const (
	minMemoryKiB = 8 * 1024    // 8 MiB
	maxMemoryKiB = 1024 * 1024 // 1 GiB
	maxTimeCost  = 16
	maxThreads   = 64
)

// KDFParams describes how the key-encryption key is derived from the master
// password. It is stored in the container header in the clear (that is safe:
// a salt and cost parameters are public) and is authenticated as associated
// data, so it cannot be downgraded without detection.
type KDFParams struct {
	Alg     string `json:"alg"`
	Version int    `json:"ver"`
	Time    uint32 `json:"time"`
	Memory  uint32 `json:"memory"` // KiB, total
	Threads uint8  `json:"threads"`
	KeyLen  uint32 `json:"keyLen"`
	Salt    []byte `json:"salt"`
}

// DefaultKDFParams returns the parameters used for new vaults: 64 MiB, 3
// passes, 4 lanes, which follows RFC 9106 section 4 (second recommended option)
// and comfortably exceeds the OWASP minimum configurations.
func DefaultKDFParams() KDFParams {
	return KDFParams{
		Alg:     kdfAlg,
		Version: argon2.Version,
		Time:    3,
		Memory:  64 * 1024,
		Threads: 4,
		KeyLen:  KeyLen,
	}
}

// WithFreshSalt returns a copy with a new cryptographically random salt.
func (k KDFParams) WithFreshSalt() (KDFParams, error) {
	salt, err := RandomBytes(saltLen)
	if err != nil {
		return KDFParams{}, err
	}
	k.Salt = salt
	return k, nil
}

func (k KDFParams) validate() error {
	if k.Alg != kdfAlg {
		return fmt.Errorf("%w: unsupported key derivation function %q", ErrCorrupt, k.Alg)
	}
	if k.Version != argon2.Version {
		return fmt.Errorf("%w: unsupported argon2 version %d", ErrCorrupt, k.Version)
	}
	if len(k.Salt) < 8 || len(k.Salt) > 64 {
		return fmt.Errorf("%w: implausible salt length %d", ErrCorrupt, len(k.Salt))
	}
	if k.KeyLen != KeyLen {
		return fmt.Errorf("%w: unsupported key length %d", ErrCorrupt, k.KeyLen)
	}
	if k.Time < 1 || k.Time > maxTimeCost {
		return fmt.Errorf("%w: argon2 time cost %d out of range", ErrCorrupt, k.Time)
	}
	if k.Memory < minMemoryKiB || k.Memory > maxMemoryKiB {
		return fmt.Errorf("%w: argon2 memory %d KiB out of range", ErrCorrupt, k.Memory)
	}
	if k.Threads < 1 || k.Threads > maxThreads {
		return fmt.Errorf("%w: argon2 parallelism %d out of range", ErrCorrupt, k.Threads)
	}
	return nil
}

// derive runs Argon2id and returns the key-encryption key.
func (k KDFParams) derive(password string) []byte {
	return argon2.IDKey([]byte(password), k.Salt, k.Time, k.Memory, k.Threads, k.KeyLen)
}

// Blob is a nonce/ciphertext pair as stored in the container header.
type Blob struct {
	Nonce []byte `json:"nonce"`
	Data  []byte `json:"data"`
}

// sealAnd returns a Blob holding plaintext encrypted under key.
func seal(key, plaintext, ad []byte) (Blob, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return Blob{}, fmt.Errorf("create aead: %w", err)
	}
	nonce, err := RandomBytes(aead.NonceSize())
	if err != nil {
		return Blob{}, err
	}
	return Blob{Nonce: nonce, Data: aead.Seal(nil, nonce, plaintext, ad)}, nil
}

// open decrypts a Blob. A failure here is indistinguishable from a wrong key by
// design; callers translate it into a user-facing error.
func open(key []byte, b Blob, ad []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("create aead: %w", err)
	}
	if len(b.Nonce) != aead.NonceSize() {
		return nil, fmt.Errorf("%w: bad nonce length %d", ErrCorrupt, len(b.Nonce))
	}
	plaintext, err := aead.Open(nil, b.Nonce, b.Data, ad)
	if err != nil {
		return nil, ErrAuthentication
	}
	return plaintext, nil
}

// RandomBytes returns n cryptographically random bytes.
func RandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("read random bytes: %w", err)
	}
	return b, nil
}

// ConstantTimeEqual compares two secrets without leaking their contents through
// timing. Used when checking a re-entered master password.
func ConstantTimeEqual(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

// zero overwrites a key buffer. It is best effort: Go gives no guarantee that
// the runtime did not copy the bytes elsewhere (for example while growing the
// stack), but it removes the obvious in-memory copy on lock.
func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
