package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"coffer/internal/vault"
)

const (
	appName     = "Coffer"
	appVersion  = "1.0.0"
	vaultFolder = "coffer"
	vaultFile   = "vault.coffer"

	// eventLocked is emitted whenever the vault locks itself, so the frontend
	// can fall back to the unlock screen even if it did not ask for the lock.
	eventLocked = "vault:locked"

	autoLockTick = 5 * time.Second
)

// App is the object exposed to the frontend through Wails bindings. Every
// exported method becomes an async function on `window.go.main.App`.
//
// The app holds no secrets of its own: the vault package owns the data key and
// forgets it on Lock.
type App struct {
	ctx   context.Context
	vault *vault.Vault

	mu           sync.Mutex
	lastActivity time.Time
	clipTimer    *time.Timer
	clipSum      [sha256.Size]byte
	clipArmed    bool
	stopAutoLock chan struct{}
	stopOnce     sync.Once
}

// NewApp wires up the vault at its resolved location.
func NewApp() *App {
	return &App{
		vault:        vault.New(vaultPath()),
		lastActivity: time.Now(),
		stopAutoLock: make(chan struct{}),
	}
}

// vaultPath resolves where the vault file lives. COFFER_VAULT_PATH overrides the
// default so the app can be used in a portable folder or a test harness.
func vaultPath() string {
	if custom := strings.TrimSpace(os.Getenv("COFFER_VAULT_PATH")); custom != "" {
		return custom
	}
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		dir = "."
	}
	return filepath.Join(dir, vaultFolder, vaultFile)
}

// startup is called by Wails once the webview exists.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.touch()
	go a.autoLockLoop()
}

// shutdown runs on window close: forget keys and leave no secret on the
// clipboard.
func (a *App) shutdown(context.Context) {
	a.stopOnce.Do(func() { close(a.stopAutoLock) })
	a.lockVault("shutdown")
}

// AppInfo describes the build and where the vault lives.
type AppInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Platform  string `json:"platform"`
	VaultPath string `json:"vaultPath"`
	DataDir   string `json:"dataDir"`
}

// AppInfo returns static information about the running application.
func (a *App) AppInfo() AppInfo {
	return AppInfo{
		Name:      appName,
		Version:   appVersion,
		Platform:  runtime.GOOS,
		VaultPath: a.vault.Path(),
		DataDir:   filepath.Dir(a.vault.Path()),
	}
}

// Status is the application state the frontend renders around the vault.
type Status struct {
	Exists    bool        `json:"exists"`
	Unlocked  bool        `json:"unlocked"`
	Info      *vault.Info `json:"info,omitempty"`
	ItemCount int         `json:"itemCount"`
	// Corrupt means a vault file exists but cannot be read as a vault, which
	// lets the UI offer the backup instead of pretending the vault is new.
	Corrupt bool   `json:"corrupt"`
	Message string `json:"message,omitempty"`
}

// Status reports whether a vault exists and whether it is currently unlocked.
func (a *App) Status() Status {
	status := Status{Exists: a.vault.Exists(), Unlocked: a.vault.Unlocked()}
	if !status.Exists {
		return status
	}
	info, err := a.vault.Info()
	if err != nil {
		status.Corrupt = true
		status.Message = err.Error()
		return status
	}
	status.Info = &info
	if status.Unlocked {
		if count, err := a.vault.Count(); err == nil {
			status.ItemCount = count
		}
	}
	return status
}

// CreateVault initialises a new encrypted vault and leaves it unlocked.
func (a *App) CreateVault(masterPassword, hint string) (Status, error) {
	if err := a.vault.Create(masterPassword, strings.TrimSpace(hint)); err != nil {
		return a.Status(), err
	}
	a.touch()
	return a.Status(), nil
}

// Unlock decrypts the vault with the master password.
func (a *App) Unlock(masterPassword string) (Status, error) {
	if err := a.vault.Unlock(masterPassword); err != nil {
		return a.Status(), err
	}
	a.touch()
	return a.Status(), nil
}

// Lock forgets the master key and every decrypted field.
func (a *App) Lock() Status {
	a.lockVault("manual")
	return a.Status()
}

// ChangeMasterPassword re-encrypts the vault under a new master password.
func (a *App) ChangeMasterPassword(current, next string) (Status, error) {
	if err := a.vault.ChangeMasterPassword(current, next); err != nil {
		return a.Status(), err
	}
	a.touch()
	return a.Status(), nil
}

// SetHint stores the password reminder shown on the unlock screen.
func (a *App) SetHint(hint string) (Status, error) {
	if err := a.vault.SetHint(strings.TrimSpace(hint)); err != nil {
		return a.Status(), err
	}
	return a.Status(), nil
}

// ListItems returns the item summaries matching the given filters. Secrets are
// deliberately not part of the summary; call GetItem for those.
func (a *App) ListItems(opts vault.ListOptions) ([]vault.Summary, error) {
	return a.vault.List(opts)
}

// GetItem returns one item including its password, notes and one-time password
// secret.
func (a *App) GetItem(id string) (vault.Item, error) {
	return a.vault.Get(id)
}

// SaveItem creates or updates an item and returns the stored version.
func (a *App) SaveItem(item vault.Item) (vault.Item, error) {
	saved, err := a.vault.Save(item)
	if err != nil {
		return vault.Item{}, err
	}
	a.touch()
	return saved, nil
}

// DeleteItem moves an item to the trash, where it can be restored until the
// retention window closes.
func (a *App) DeleteItem(id string) error {
	err := a.vault.Delete(id)
	a.touch()
	return err
}

// TrashList returns the items sitting in the trash, most recently deleted first.
func (a *App) TrashList() ([]vault.Summary, error) {
	return a.vault.Trash()
}

// RestoreItem puts a trashed item back into the vault.
func (a *App) RestoreItem(id string) (vault.Item, error) {
	item, err := a.vault.Restore(id)
	if err != nil {
		return vault.Item{}, err
	}
	a.touch()
	return item, nil
}

// DeleteForever removes an item without leaving a copy in the trash.
func (a *App) DeleteForever(id string) error {
	err := a.vault.DeleteForever(id)
	a.touch()
	return err
}

// EmptyTrash permanently removes every trashed item and reports how many.
func (a *App) EmptyTrash() (int, error) {
	removed, err := a.vault.EmptyTrash()
	a.touch()
	return removed, err
}

// Audit runs the offline health check over the vault: weak, reused and stale
// passwords, missing second factors and expiring cards. It makes no network
// calls and returns nothing that could identify a password to a third party.
func (a *App) Audit() (vault.AuditReport, error) {
	return a.vault.Audit()
}

// ImportCSV merges a password-manager CSV export into the vault. With
// options.DryRun set it only classifies the rows; the UI shows that preview
// before the user commits.
func (a *App) ImportCSV(text string, options vault.ImportOptions) (vault.ImportResult, error) {
	result, err := a.vault.ImportCSV(text, options)
	if err != nil {
		return vault.ImportResult{}, err
	}
	if !options.DryRun {
		a.touch()
	}
	return result, nil
}

// ToggleFavorite flips the favorite flag of an item.
func (a *App) ToggleFavorite(id string) (vault.Item, error) {
	item, err := a.vault.ToggleFavorite(id)
	a.touch()
	return item, err
}

// Tags lists the tags in use, most used first.
func (a *App) Tags() ([]vault.TagCount, error) {
	return a.vault.Tags()
}

// Stats returns item counts for the sidebar.
func (a *App) Stats() (vault.Stats, error) {
	return a.vault.Stats()
}

// GetSettings returns the vault settings.
func (a *App) GetSettings() (vault.Settings, error) {
	return a.vault.Settings()
}

// SaveSettings stores new settings and returns the normalised result.
func (a *App) SaveSettings(settings vault.Settings) (vault.Settings, error) {
	saved, err := a.vault.SaveSettings(settings)
	if err != nil {
		return vault.Settings{}, err
	}
	a.touch()
	return saved, nil
}

// GeneratePassword creates a random password or passphrase. It works while the
// vault is locked so the generator can be used from the unlock screen.
func (a *App) GeneratePassword(opts vault.GeneratorOptions) (vault.Generated, error) {
	return vault.GeneratePassword(opts)
}

// ScorePassword rates a candidate password offline.
func (a *App) ScorePassword(password string) vault.Strength {
	return vault.Score(password)
}

// TOTPCode computes the current one-time password for an item. The seed stays
// inside the app: only the six digit code crosses into the webview.
func (a *App) TOTPCode(id string) (vault.TOTPResult, error) {
	item, err := a.vault.Get(id)
	if err != nil {
		return vault.TOTPResult{}, err
	}
	if item.TOTP == "" {
		return vault.TOTPResult{}, fmt.Errorf("this item has no one-time password secret")
	}
	return vault.CodeForSecret(item.TOTP, time.Now())
}

// CopySecret puts a value on the system clipboard and optionally clears it
// again after a delay, but only if it is still the value we copied.
func (a *App) CopySecret(value string, clearAfterSeconds int) error {
	if !a.vault.Unlocked() {
		return vault.ErrLocked
	}
	if value == "" {
		return fmt.Errorf("nothing to copy")
	}
	if clearAfterSeconds < 0 {
		clearAfterSeconds = 0
	}
	if clearAfterSeconds > 600 {
		clearAfterSeconds = 600
	}
	if err := wailsruntime.ClipboardSetText(a.ctx, value); err != nil {
		return fmt.Errorf("clipboard unavailable: %w", err)
	}
	a.mu.Lock()
	if a.clipTimer != nil {
		a.clipTimer.Stop()
	}
	a.clipSum = sha256.Sum256([]byte(value))
	a.clipArmed = true
	if clearAfterSeconds > 0 {
		a.clipTimer = time.AfterFunc(time.Duration(clearAfterSeconds)*time.Second, a.clearClipboardIfUnchanged)
	} else {
		a.clipTimer = nil
	}
	a.mu.Unlock()
	a.touch()
	return nil
}

// ClearClipboard clears the clipboard immediately, but only if the current
// contents are the secret we placed there.
func (a *App) ClearClipboard() error {
	a.clearClipboardIfUnchanged()
	return nil
}

func (a *App) clearClipboardIfUnchanged() {
	a.mu.Lock()
	armed := a.clipArmed
	a.clipArmed = false
	if a.clipTimer != nil {
		a.clipTimer.Stop()
		a.clipTimer = nil
	}
	sum := a.clipSum
	a.mu.Unlock()
	if !armed {
		return
	}
	current, err := wailsruntime.ClipboardGetText(a.ctx)
	if err != nil || current == "" {
		return
	}
	if sha256.Sum256([]byte(current)) != sum {
		// The user copied something else; never clobber their clipboard.
		return
	}
	_ = wailsruntime.ClipboardSetText(a.ctx, "")
}

// AutoLockState is the countdown shown in the title bar.
type AutoLockState struct {
	Enabled          bool `json:"enabled"`
	TimeoutSeconds   int  `json:"timeoutSeconds"`
	SecondsRemaining int  `json:"secondsRemaining"`
}

// Ping records user activity and returns the auto-lock countdown. The frontend
// calls it on input so that the lock timer follows real activity.
func (a *App) Ping() AutoLockState {
	a.touch()
	return a.AutoLockState()
}

// AutoLockState reports how long the vault will stay unlocked without activity.
func (a *App) AutoLockState() AutoLockState {
	settings, err := a.vault.Settings()
	if err != nil || settings.AutoLockMinutes <= 0 {
		return AutoLockState{}
	}
	a.mu.Lock()
	idle := time.Since(a.lastActivity)
	a.mu.Unlock()
	timeout := time.Duration(settings.AutoLockMinutes) * time.Minute
	remaining := int((timeout - idle).Seconds())
	if remaining < 0 {
		remaining = 0
	}
	return AutoLockState{Enabled: true, TimeoutSeconds: int(timeout.Seconds()), SecondsRemaining: remaining}
}

// ExportBackup writes an encrypted copy of the vault to a location the user
// picks. The copy is still protected by the same master password.
func (a *App) ExportBackup() (string, error) {
	if !a.vault.Exists() {
		return "", fmt.Errorf("there is no vault to export yet")
	}
	dest, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Export an encrypted backup",
		DefaultFilename: fmt.Sprintf("coffer-backup-%s.coffer", time.Now().Format("2006-01-02")),
		Filters:         []wailsruntime.FileFilter{{DisplayName: "Coffer vault (*.coffer)", Pattern: "*.coffer"}},
	})
	if err != nil {
		return "", err
	}
	if dest == "" {
		return "", nil // cancelled
	}
	if err := a.vault.ExportTo(dest); err != nil {
		return "", err
	}
	a.touch()
	return dest, nil
}

// ImportVault replaces the current vault with a backup the user picks. It
// requires the vault to be locked so a running session cannot be half-replaced.
func (a *App) ImportVault() (Status, error) {
	if a.vault.Unlocked() {
		return a.Status(), fmt.Errorf("lock the vault before importing a backup")
	}
	src, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:   "Choose a Coffer vault to import",
		Filters: []wailsruntime.FileFilter{{DisplayName: "Coffer vault (*.coffer)", Pattern: "*.coffer"}},
	})
	if err != nil {
		return a.Status(), err
	}
	if src == "" {
		return a.Status(), nil // cancelled
	}
	if src == a.vault.Path() {
		return a.Status(), fmt.Errorf("that is the vault you are already using")
	}
	if err := vault.ReplaceVault(src, a.vault.Path()); err != nil {
		return a.Status(), err
	}
	return a.Status(), nil
}

// InspectVaultFile reports the metadata of an arbitrary vault file so the UI
// can confirm what is being restored.
func (a *App) InspectVaultFile(path string) (vault.Info, error) {
	return vault.Inspect(path)
}

// RestoreBackup rolls the vault back to its previous revision. The .bak file is
// what an interrupted write would have left behind.
func (a *App) RestoreBackup() (Status, error) {
	if a.vault.Unlocked() {
		a.lockVault("restore")
	}
	if _, err := os.Stat(a.vault.BackupPath()); err != nil {
		return a.Status(), fmt.Errorf("no backup revision is available")
	}
	if err := vault.ReplaceVault(a.vault.BackupPath(), a.vault.Path()); err != nil {
		return a.Status(), err
	}
	return a.Status(), nil
}

// touch records activity for the auto-lock timer.
func (a *App) touch() {
	a.mu.Lock()
	a.lastActivity = time.Now()
	a.mu.Unlock()
}

// lockVault forgets the keys, clears a secret we placed on the clipboard and
// tells the frontend to show the unlock screen.
func (a *App) lockVault(reason string) {
	wasUnlocked := a.vault.Unlocked()
	a.vault.Lock()
	a.mu.Lock()
	armed := a.clipArmed
	a.mu.Unlock()
	if armed {
		a.clearClipboardIfUnchanged()
	}
	if wasUnlocked && a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, eventLocked, reason)
	}
}

func (a *App) autoLockLoop() {
	ticker := time.NewTicker(autoLockTick)
	defer ticker.Stop()
	for {
		select {
		case <-a.stopAutoLock:
			return
		case <-ticker.C:
			if a.shouldAutoLock() {
				a.lockVault("timeout")
			}
		}
	}
}

func (a *App) shouldAutoLock() bool {
	if !a.vault.Unlocked() {
		return false
	}
	settings, err := a.vault.Settings()
	if err != nil || settings.AutoLockMinutes <= 0 {
		return false
	}
	a.mu.Lock()
	idle := time.Since(a.lastActivity)
	a.mu.Unlock()
	return idle >= time.Duration(settings.AutoLockMinutes)*time.Minute
}
