package vault

import (
	"fmt"
	"time"
)

// Settings are stored inside the encrypted vault: nothing about the user's
// configuration leaks into a plaintext sidecar file.
type Settings struct {
	// AutoLockMinutes locks the vault after this much inactivity (0 = never).
	AutoLockMinutes int `json:"autoLockMinutes"`
	// ClipboardClearSeconds clears a copied secret after this long (0 = never).
	ClipboardClearSeconds int `json:"clipboardClearSeconds"`
	// LockOnBlur locks the vault when its window loses focus.
	LockOnBlur bool `json:"lockOnBlur"`
	// Theme is the UI theme: dark, light or system.
	Theme              string           `json:"theme"`
	Generator          GeneratorOptions `json:"generator"`
	PasswordHistoryMax int              `json:"passwordHistoryMax"`
	// TrashRetentionDays purges deleted items after this many days (0 = keep
	// them until they are removed by hand).
	TrashRetentionDays int `json:"trashRetentionDays"`
}

// DefaultSettings returns the shipped defaults: a five minute auto-lock and a
// clipboard that forgets secrets after thirty seconds.
func DefaultSettings() Settings {
	return Settings{
		AutoLockMinutes:       5,
		ClipboardClearSeconds: 30,
		LockOnBlur:            false,
		Theme:                 "dark",
		Generator:             DefaultGeneratorOptions(),
		PasswordHistoryMax:    5,
		TrashRetentionDays:    30,
	}
}

func (s *Settings) normalize() error {
	if s.AutoLockMinutes < 0 {
		s.AutoLockMinutes = 0
	}
	if s.AutoLockMinutes > 24*60 {
		s.AutoLockMinutes = 24 * 60
	}
	if s.ClipboardClearSeconds < 0 {
		s.ClipboardClearSeconds = 0
	}
	if s.ClipboardClearSeconds > 600 {
		s.ClipboardClearSeconds = 600
	}
	switch s.Theme {
	case "dark", "light", "system":
	default:
		s.Theme = "dark"
	}
	if s.PasswordHistoryMax < 0 {
		s.PasswordHistoryMax = 0
	}
	if s.PasswordHistoryMax > maxHistory {
		s.PasswordHistoryMax = maxHistory
	}
	if s.TrashRetentionDays < 0 {
		s.TrashRetentionDays = 0
	}
	if s.TrashRetentionDays > maxTrashRetentionDays {
		s.TrashRetentionDays = maxTrashRetentionDays
	}
	gen := s.Generator
	if gen.Mode == "" {
		s.Generator = DefaultGeneratorOptions()
	} else if err := s.Generator.normalize(); err != nil {
		// A settings blob from a future version should not brick the vault.
		s.Generator = DefaultGeneratorOptions()
	}
	return nil
}

// maxTrashRetentionDays bounds the automatic purge window at ten years: long
// enough to be "forever" in practice, short enough to stay a sane integer.
const maxTrashRetentionDays = 3650

// Payload is the decrypted body of a vault file.
type Payload struct {
	Version   int       `json:"version"`
	Items     []Item    `json:"items"`
	Settings  Settings  `json:"settings"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func newPayload() Payload {
	return Payload{
		Version:  FormatVersion,
		Items:    []Item{},
		Settings: DefaultSettings(),
	}
}

// Info is the non-secret metadata the UI shows on the unlock screen and in
// settings. It is readable without the master password.
type Info struct {
	Path      string    `json:"path"`
	VaultID   string    `json:"vaultId"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Revision  uint64    `json:"revision"`
	Hint      string    `json:"hint,omitempty"`
	Format    int       `json:"format"`
	KDF       string    `json:"kdf"`
	MemoryMiB int       `json:"memoryMiB"`
	Passes    uint32    `json:"passes"`
	Parallel  uint8     `json:"parallelism"`
	Backup    bool      `json:"backupAvailable"`
}

func (h *Header) info(path string, backup bool) Info {
	return Info{
		Path:      path,
		VaultID:   h.VaultID,
		CreatedAt: h.CreatedAt,
		UpdatedAt: h.UpdatedAt,
		Revision:  h.Revision,
		Hint:      h.Hint,
		Format:    h.Format,
		KDF:       fmt.Sprintf("%s (argon2 v%d)", h.KDF.Alg, h.KDF.Version),
		MemoryMiB: int(h.KDF.Memory / 1024),
		Passes:    h.KDF.Time,
		Parallel:  h.KDF.Threads,
		Backup:    backup,
	}
}
