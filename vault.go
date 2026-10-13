package main

import (
	_ "embed"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Vault struct {
	mu      sync.Mutex
	dbPath  string
	key     []byte
	unlocked bool
	entries []Entry
}

type Entry struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Value string `json:"value"`
	Kind  string `json:"kind"`
}

func NewVault() *Vault {
	dbDir, _ := os.UserConfigDir()
	dbPath := filepath.Join(dbDir, "vault-new", "vault.db")
	return &Vault{dbPath: dbPath}
}

func (v *Vault) startup(ctx interface{}) {
	v.mu.Lock()
	defer v.mu.Unlock()
}

func (v *Vault) Unlock(pin string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.unlocked {
		return nil
	}
	key := sha256.Sum256([]byte(pin))
	v.key = key[:]
	v.unlocked = true
	return v.load()
}

func (v *Vault) load() error {
	v.entries = nil
	data, err := os.ReadFile(v.dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	block, err := aes.NewCipher(v.key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	if len(data) < gcm.NonceSize() {
		return fmt.Errorf("invalid vault data")
	}
	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("decrypt failed: wrong pin")
	}
	if err := json.Unmarshal(plain, &v.entries); err != nil {
		return err
	}
	return nil
}

func (v *Vault) Lock() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.key = nil
	v.unlocked = false
	v.entries = nil
	return nil
}

func (v *Vault) GetEntries() ([]Entry, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.unlocked {
		return nil, fmt.Errorf("vault locked")
	}
	out := make([]Entry, len(v.entries))
	copy(out, v.entries)
	return out, nil
}

func (v *Vault) AddEntry(name, value, kind string) (Entry, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.unlocked {
		return Entry{}, fmt.Errorf("vault locked")
	}
	id, _ := randId()
	e := Entry{ID: id, Name: name, Value: value, Kind: kind}
	v.entries = append(v.entries, e)
	v.save()
	return e, nil
}

func (v *Vault) UpdateEntry(id, name, value, kind string) (Entry, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.unlocked {
		return Entry{}, fmt.Errorf("vault locked")
	}
	for i := range v.entries {
		if v.entries[i].ID == id {
			v.entries[i].Name = name
			v.entries[i].Value = value
			v.entries[i].Kind = kind
			v.save()
			return v.entries[i], nil
		}
	}
	return Entry{}, fmt.Errorf("not found")
}

func (v *Vault) DeleteEntry(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.unlocked {
		return fmt.Errorf("vault locked")
	}
	for i, e := range v.entries {
		if e.ID == id {
			v.entries = append(v.entries[:i], v.entries[i+1:]...)
			v.save()
			return nil
		}
	}
	return fmt.Errorf("not found")
}

func (v *Vault) save() error {
	data, err := json.Marshal(v.entries)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(v.key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	os.MkdirAll(filepath.Dir(v.dbPath), 0o700)
	return os.WriteFile(v.dbPath, ciphertext, 0o600)
}

func randId() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
