package vault

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// MinMasterPasswordLength is enforced by the vault itself, not only by the UI,
// so a scripted client cannot create a trivially brute-forceable vault.
const MinMasterPasswordLength = 10

// Vault is a single-file, encrypted password store. It is safe for concurrent
// use; every public method takes the lock.
type Vault struct {
	mu       sync.RWMutex
	path     string
	header   *Header
	dek      []byte // data-encryption key, nil while locked
	payload  Payload
	unlocked bool
	now      func() time.Time
}

// New returns a vault handle for path. The file is not touched until Create or
// Unlock is called.
func New(path string) *Vault {
	return &Vault{path: path, now: time.Now}
}

// Path is the on-disk location of the vault file.
func (v *Vault) Path() string { return v.path }

// BackupPath is the sibling file holding the previous revision.
func (v *Vault) BackupPath() string { return v.path + ".bak" }

// Exists reports whether a vault file is present.
func (v *Vault) Exists() bool {
	info, err := os.Stat(v.path)
	return err == nil && !info.IsDir()
}

// Unlocked reports whether secrets are currently held in memory.
func (v *Vault) Unlocked() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.unlocked
}

// Info returns the metadata stored in the container header. It works while
// locked, which is what lets the unlock screen show the hint and KDF cost.
func (v *Vault) Info() (Info, error) {
	v.mu.RLock()
	if v.unlocked && v.header != nil {
		header := *v.header
		v.mu.RUnlock()
		return header.info(v.path, v.backupExists()), nil
	}
	v.mu.RUnlock()

	raw, err := readFile(v.path)
	if err != nil {
		if os.IsNotExist(err) {
			return Info{}, fmt.Errorf("%w: no vault at %s", ErrNotFound, v.path)
		}
		return Info{}, err
	}
	header, _, err := parseContainer(raw)
	if err != nil {
		return Info{}, err
	}
	return header.info(v.path, v.backupExists()), nil
}

func (v *Vault) backupExists() bool {
	info, err := os.Stat(v.BackupPath())
	return err == nil && !info.IsDir()
}

// Create initialises a new vault file. It fails if a file already exists so
// that a mistake cannot silently destroy an existing vault.
func (v *Vault) Create(masterPassword, hint string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.unlocked {
		return fmt.Errorf("%w: a vault is already open", ErrInvalid)
	}
	if v.Exists() {
		return ErrExists
	}
	if err := checkMasterPassword(masterPassword); err != nil {
		return err
	}

	kdf, err := DefaultKDFParams().WithFreshSalt()
	if err != nil {
		return err
	}
	dek, err := RandomBytes(KeyLen)
	if err != nil {
		return err
	}
	vaultID, err := newID()
	if err != nil {
		return err
	}
	now := v.now().UTC().Truncate(time.Second)
	header := &Header{
		Format:    FormatVersion,
		VaultID:   vaultID,
		CreatedAt: now,
		UpdatedAt: now,
		Revision:  1,
		Hint:      hint,
		KDF:       kdf,
		AEAD:      aeadAlg,
	}

	kek := kdf.derive(masterPassword)
	wrap, err := seal(kek, dek, header.wrapAAD())
	zero(kek)
	if err != nil {
		zero(dek)
		return err
	}
	header.KeyWrap = wrap

	payload := newPayload()
	plaintext, err := json.Marshal(payload)
	if err != nil {
		zero(dek)
		return fmt.Errorf("encode vault: %w", err)
	}
	body, err := seal(dek, plaintext, header.payloadAAD())
	zero(plaintext)
	if err != nil {
		zero(dek)
		return err
	}
	header.PayloadNonce = body.Nonce

	v.header = header
	v.dek = dek
	v.payload = payload
	v.unlocked = true

	return v.write(body.Data)
}

// Unlock decrypts the vault with the master password.
func (v *Vault) Unlock(masterPassword string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.unlocked {
		return nil
	}
	if err := checkMasterPassword(masterPassword); err != nil {
		return err
	}
	raw, err := readFile(v.path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: no vault at %s", ErrNotFound, v.path)
		}
		return err
	}
	header, ciphertext, err := parseContainer(raw)
	if err != nil {
		return err
	}

	kek := header.KDF.derive(masterPassword)
	dek, err := open(kek, header.KeyWrap, header.wrapAAD())
	zero(kek)
	if err != nil {
		return ErrWrongPassword
	}
	if len(dek) != KeyLen {
		zero(dek)
		return fmt.Errorf("%w: unexpected data key length", ErrCorrupt)
	}
	plaintext, err := open(dek, Blob{Nonce: header.PayloadNonce, Data: ciphertext}, header.payloadAAD())
	if err != nil {
		zero(dek)
		return fmt.Errorf("%w: the vault body failed authentication, the file may have been modified or truncated", ErrCorrupt)
	}
	var payload Payload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		zero(dek)
		return fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	zero(plaintext)
	if payload.Version > FormatVersion {
		zero(dek)
		return fmt.Errorf("%w: vault was written by a newer version of Coffer (payload v%d)", ErrCorrupt, payload.Version)
	}
	if payload.Items == nil {
		payload.Items = []Item{}
	}
	if err := payload.Settings.normalize(); err != nil {
		zero(dek)
		return err
	}

	v.header = header
	v.dek = dek
	v.payload = payload
	v.unlocked = true
	return nil
}

// Lock forgets every key and decrypted field held in memory.
func (v *Vault) Lock() {
	v.mu.Lock()
	defer v.mu.Unlock()
	zero(v.dek)
	v.dek = nil
	v.payload = Payload{}
	v.unlocked = false
}

// VerifyPassword re-derives the key-encryption key and unwraps the data key,
// proving the caller knows the master password without disturbing the session.
func (v *Vault) VerifyPassword(masterPassword string) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.header == nil {
		return false
	}
	kek := v.header.KDF.derive(masterPassword)
	defer zero(kek)
	_, err := open(kek, v.header.KeyWrap, v.header.wrapAAD())
	return err == nil
}

// ChangeMasterPassword rotates the master password while keeping the data key,
// so the vault body does not have to be re-encrypted with a password-derived
// key. A fresh salt is generated, which also invalidates any rainbow-table
// progress against the previous salt.
func (v *Vault) ChangeMasterPassword(current, next string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.unlocked {
		return ErrLocked
	}
	if err := checkMasterPassword(next); err != nil {
		return err
	}
	kekOld := v.header.KDF.derive(current)
	_, err := open(kekOld, v.header.KeyWrap, v.header.wrapAAD())
	zero(kekOld)
	if err != nil {
		return ErrWrongPassword
	}

	kdf, err := DefaultKDFParams().WithFreshSalt()
	if err != nil {
		return err
	}
	kekNew := kdf.derive(next)
	defer zero(kekNew)

	// Order matters: the wrap associated data covers the KDF parameters, so the
	// header must be updated before the data key is re-wrapped.
	v.header.KDF = kdf
	v.header.UpdatedAt = v.now().UTC().Truncate(time.Second)
	v.header.Revision++
	wrap, err := seal(kekNew, v.dek, v.header.wrapAAD())
	if err != nil {
		return err
	}
	v.header.KeyWrap = wrap
	return v.persist()
}

// SetHint updates the password reminder shown on the unlock screen.
func (v *Vault) SetHint(hint string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.unlocked {
		return ErrLocked
	}
	if len(hint) > 256 {
		return fmt.Errorf("%w: the hint is limited to 256 characters", ErrInvalid)
	}
	v.header.Hint = hint
	v.header.UpdatedAt = v.now().UTC().Truncate(time.Second)
	v.header.Revision++
	return v.persist()
}

// Count returns the number of live items in the vault. Trashed items are not
// counted; Trash and Stats report them separately.
func (v *Vault) Count() (int, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if !v.unlocked {
		return 0, ErrLocked
	}
	total := 0
	for i := range v.payload.Items {
		if !v.payload.Items[i].Trashed() {
			total++
		}
	}
	return total, nil
}

// List returns summaries matching opts, ordered by opts.Sort.
func (v *Vault) List(opts ListOptions) ([]Summary, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if !v.unlocked {
		return nil, ErrLocked
	}
	matches := make([]*Item, 0, len(v.payload.Items))
	for i := range v.payload.Items {
		item := &v.payload.Items[i]
		if item.Trashed() {
			continue
		}
		if opts.match(item) {
			matches = append(matches, item)
		}
	}
	opts.sort(matches)
	out := make([]Summary, 0, len(matches))
	for _, item := range matches {
		out = append(out, item.Summary())
	}
	return out, nil
}

// Get returns the full item including its secrets.
func (v *Vault) Get(id string) (Item, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if !v.unlocked {
		return Item{}, ErrLocked
	}
	idx := v.indexOf(id)
	if idx < 0 || v.payload.Items[idx].Trashed() {
		return Item{}, ErrNotFound
	}
	return *cloneItem(&v.payload.Items[idx]), nil
}

// Save creates (empty ID) or updates an item and writes the vault to disk. When
// the password changes, the previous one is pushed onto the item's history.
func (v *Vault) Save(item Item) (Item, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.unlocked {
		return Item{}, ErrLocked
	}
	now := v.now().UTC().Truncate(time.Second)
	item.normalize()
	if err := item.validate(); err != nil {
		return Item{}, err
	}

	if item.ID == "" {
		id, err := newID()
		if err != nil {
			return Item{}, err
		}
		item.ID = id
		item.CreatedAt = now
		item.UpdatedAt = now
		item.History = nil
		v.payload.Items = append(v.payload.Items, item)
	} else {
		idx := v.indexOf(item.ID)
		if idx < 0 || v.payload.Items[idx].Trashed() {
			return Item{}, ErrNotFound
		}
		existing := &v.payload.Items[idx]
		history := existing.History
		if existing.Password != "" && existing.Password != item.Password {
			history = append([]HistoryEntry{{Password: existing.Password, ChangedAt: now}}, history...)
		}
		limit := v.payload.Settings.PasswordHistoryMax
		if limit > 0 && len(history) > limit {
			history = history[:limit]
		}
		item.CreatedAt = existing.CreatedAt
		item.UpdatedAt = now
		item.History = history
		*existing = item
	}

	if err := v.persist(); err != nil {
		return Item{}, err
	}
	saved := item
	return saved, nil
}

// Delete moves an item to the trash. The item stays encrypted inside the same
// file and can be restored until the retention window closes; use DeleteForever
// or EmptyTrash to remove it for good.
func (v *Vault) Delete(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.unlocked {
		return ErrLocked
	}
	idx := v.indexOf(id)
	if idx < 0 || v.payload.Items[idx].Trashed() {
		return ErrNotFound
	}
	now := v.now().UTC().Truncate(time.Second)
	v.payload.Items[idx].DeletedAt = &now
	v.payload.Items[idx].UpdatedAt = now
	return v.persist()
}

// ToggleFavorite flips the favorite flag of an item.
func (v *Vault) ToggleFavorite(id string) (Item, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.unlocked {
		return Item{}, ErrLocked
	}
	idx := v.indexOf(id)
	if idx < 0 || v.payload.Items[idx].Trashed() {
		return Item{}, ErrNotFound
	}
	item := &v.payload.Items[idx]
	item.Favorite = !item.Favorite
	item.UpdatedAt = v.now().UTC().Truncate(time.Second)
	if err := v.persist(); err != nil {
		return Item{}, err
	}
	return *cloneItem(item), nil
}

// TagCount is one entry of the sidebar tag list.
type TagCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Tags lists the distinct tags in use, most used first then alphabetically.
func (v *Vault) Tags() ([]TagCount, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if !v.unlocked {
		return nil, ErrLocked
	}
	counts := map[string]*TagCount{}
	for i := range v.payload.Items {
		if v.payload.Items[i].Trashed() {
			continue
		}
		for _, tag := range v.payload.Items[i].Tags {
			key := strings.ToLower(tag)
			entry, ok := counts[key]
			if !ok {
				entry = &TagCount{Name: tag}
				counts[key] = entry
			}
			entry.Count++
		}
	}
	out := make([]TagCount, 0, len(counts))
	for _, entry := range counts {
		out = append(out, *entry)
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Count != out[b].Count {
			return out[a].Count > out[b].Count
		}
		return out[a].Name < out[b].Name
	})
	return out, nil
}

// Stats summarises the vault for the sidebar footer. Trashed items are counted
// separately and are not part of Total.
type Stats struct {
	Total     int `json:"total"`
	Logins    int `json:"logins"`
	Notes     int `json:"notes"`
	Cards     int `json:"cards"`
	Favorites int `json:"favorites"`
	WithTOTP  int `json:"withTOTP"`
	Trashed   int `json:"trashed"`
}

// Stats returns item counts by type.
func (v *Vault) Stats() (Stats, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if !v.unlocked {
		return Stats{}, ErrLocked
	}
	var out Stats
	for i := range v.payload.Items {
		item := &v.payload.Items[i]
		if item.Trashed() {
			out.Trashed++
			continue
		}
		out.Total++
		switch item.Type {
		case TypeLogin:
			out.Logins++
		case TypeNote:
			out.Notes++
		case TypeCard:
			out.Cards++
		}
		if item.Favorite {
			out.Favorites++
		}
		if item.TOTP != "" {
			out.WithTOTP++
		}
	}
	return out, nil
}

// Settings returns a copy of the vault settings.
func (v *Vault) Settings() (Settings, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if !v.unlocked {
		return Settings{}, ErrLocked
	}
	return v.payload.Settings, nil
}

// SaveSettings validates and stores new settings.
func (v *Vault) SaveSettings(s Settings) (Settings, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.unlocked {
		return Settings{}, ErrLocked
	}
	if s.Theme == "" {
		s.Theme = v.payload.Settings.Theme
	}
	if s.Generator.Mode == "" {
		s.Generator = v.payload.Settings.Generator
	}
	if err := s.Generator.normalize(); err != nil {
		return Settings{}, err
	}
	if err := s.normalize(); err != nil {
		return Settings{}, err
	}
	v.payload.Settings = s
	if err := v.persist(); err != nil {
		return Settings{}, err
	}
	return s, nil
}

// ExportTo copies the encrypted vault file to dest. The copy stays encrypted
// and remains unlockable with the same master password, which makes it a safe
// local backup.
func (v *Vault) ExportTo(dest string) error {
	if dest == "" {
		return fmt.Errorf("%w: choose a destination", ErrInvalid)
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.path == dest {
		return fmt.Errorf("%w: choose a destination other than the vault itself", ErrInvalid)
	}
	if v.unlocked {
		// Flush pending changes so the export matches what the user sees.
		if err := v.persist(); err != nil {
			return err
		}
	}
	raw, err := readFile(v.path)
	if err != nil {
		return err
	}
	return writeFileAtomic(dest, raw)
}

// persist re-encrypts the payload and writes the container. The payload nonce
// is always fresh, so rewriting the vault never reuses a nonce. Any item whose
// retention window has closed is dropped first, so the trash cannot grow
// without bound.
func (v *Vault) persist() error {
	if v.dek == nil {
		return ErrLocked
	}
	v.purgeExpired()
	v.header.Revision++
	v.header.UpdatedAt = v.now().UTC().Truncate(time.Second)
	v.payload.UpdatedAt = v.header.UpdatedAt

	plaintext, err := json.Marshal(v.payload)
	if err != nil {
		return fmt.Errorf("encode vault: %w", err)
	}
	body, err := seal(v.dek, plaintext, v.header.payloadAAD())
	zero(plaintext)
	if err != nil {
		return err
	}
	v.header.PayloadNonce = body.Nonce
	return v.write(body.Data)
}

// write renders and flushes the container.
func (v *Vault) write(ciphertext []byte) error {
	raw, err := marshalContainer(v.header, ciphertext)
	if err != nil {
		return err
	}
	return writeFileAtomic(v.path, raw)
}

func (v *Vault) indexOf(id string) int {
	for i := range v.payload.Items {
		if v.payload.Items[i].ID == id {
			return i
		}
	}
	return -1
}

func checkMasterPassword(password string) error {
	if len([]rune(password)) < MinMasterPasswordLength {
		return fmt.Errorf("%w: the master password must be at least %d characters", ErrInvalid, MinMasterPasswordLength)
	}
	if len(password) > 1024 {
		return fmt.Errorf("%w: the master password is too long", ErrInvalid)
	}
	return nil
}

// cloneItem deep-copies the slices so callers cannot mutate vault state.
func cloneItem(item *Item) *Item {
	out := *item
	out.Tags = append([]string(nil), item.Tags...)
	out.Custom = append([]CustomField(nil), item.Custom...)
	out.History = append([]HistoryEntry(nil), item.History...)
	if item.Card != nil {
		card := *item.Card
		out.Card = &card
	}
	return &out
}

// newID returns a random RFC 4122 version 4 UUID.
func newID() (string, error) {
	b, err := RandomBytes(16)
	if err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
