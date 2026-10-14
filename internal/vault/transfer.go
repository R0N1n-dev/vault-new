package vault

import (
	"fmt"
	"path/filepath"
)

// Inspect reads the container metadata of an arbitrary file without needing the
// master password. It is used to validate a candidate file before importing it
// and to power a "what is in this file?" preview.
func Inspect(path string) (Info, error) {
	raw, err := readFile(path)
	if err != nil {
		return Info{}, err
	}
	header, _, err := parseContainer(raw)
	if err != nil {
		return Info{}, err
	}
	return header.info(path, false), nil
}

// ReplaceVault validates src as a Coffer vault and atomically installs it at
// dst, keeping the previous dst as a .bak sibling. The caller must ensure the
// destination vault is locked; nothing here touches in-memory state.
func ReplaceVault(src, dst string) error {
	if src == "" || dst == "" {
		return fmt.Errorf("%w: a source and a destination are required", ErrInvalid)
	}
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	dstAbs, err := filepath.Abs(dst)
	if err != nil {
		return err
	}
	if srcAbs == dstAbs {
		return fmt.Errorf("%w: the source and destination are the same file", ErrInvalid)
	}
	// Fail before touching the destination if the source is not a vault.
	if _, err := Inspect(src); err != nil {
		return err
	}
	raw, err := readFile(src)
	if err != nil {
		return err
	}
	return writeFileAtomic(dstAbs, raw)
}
