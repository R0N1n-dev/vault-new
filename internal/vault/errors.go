package vault

import "errors"

// Sentinel errors returned by the vault. The UI maps these onto messages; they
// are deliberately coarse so that a failed unlock cannot be used as an oracle
// to distinguish "wrong password" from "modified file".
var (
	// ErrLocked means the operation needs the vault to be unlocked first.
	ErrLocked = errors.New("vault is locked")
	// ErrWrongPassword means the master password did not unlock the vault.
	ErrWrongPassword = errors.New("invalid master password or corrupted vault")
	// ErrAuthentication means an AEAD tag check failed.
	ErrAuthentication = errors.New("authentication failed")
	// ErrCorrupt means the file could not be understood as a Coffer vault.
	ErrCorrupt = errors.New("vault file is corrupt or unsupported")
	// ErrNotAVault means the file is not a Coffer vault at all.
	ErrNotAVault = errors.New("not a Coffer vault file")
	// ErrExists means a vault already exists at the target path.
	ErrExists = errors.New("a vault file already exists")
	// ErrNotFound means the requested item does not exist.
	ErrNotFound = errors.New("item not found")
	// ErrInvalid wraps user input validation failures.
	ErrInvalid = errors.New("invalid input")
)
