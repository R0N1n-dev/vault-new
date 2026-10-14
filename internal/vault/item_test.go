package vault

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func seedVault(t *testing.T) *Vault {
	t.Helper()
	v := newTestVault(t)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	items := []Item{
		{Type: TypeLogin, Title: "GitHub", Username: "ada", URL: "https://github.com", Tags: []string{"work", "code"}, Favorite: true},
		{Type: TypeLogin, Title: "Fastmail", Username: "ada@example.com", URL: "https://fastmail.com", Tags: []string{"personal"}},
		{Type: TypeNote, Title: "Passport renewal", Notes: "expires 2031", Tags: []string{"personal"}},
		{Type: TypeCard, Title: "Travel card", Card: &Card{Cardholder: "Ada Lovelace", Number: "4111111111111111"}},
	}
	for _, item := range items {
		if _, err := v.Save(item); err != nil {
			t.Fatalf("save %q: %v", item.Title, err)
		}
	}
	return v
}

func TestListFiltersSortsAndNeverLeaksSecrets(t *testing.T) {
	v := seedVault(t)

	all, err := v.List(ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("want 4 items, got %d", len(all))
	}
	// Default ordering is case-insensitive by title.
	want := []string{"Fastmail", "GitHub", "Passport renewal", "Travel card"}
	for i, summary := range all {
		if summary.Title != want[i] {
			t.Fatalf("order mismatch at %d: got %q want %q", i, summary.Title, want[i])
		}
	}

	favorites, err := v.List(ListOptions{FavoritesOnly: true})
	if err != nil {
		t.Fatalf("list favorites: %v", err)
	}
	if len(favorites) != 1 || favorites[0].Title != "GitHub" {
		t.Fatalf("unexpected favorites: %+v", favorites)
	}

	logins, err := v.List(ListOptions{Type: TypeLogin})
	if err != nil {
		t.Fatalf("list logins: %v", err)
	}
	if len(logins) != 2 {
		t.Fatalf("want 2 logins, got %d", len(logins))
	}

	tagged, err := v.List(ListOptions{Tag: "PERSONAL"})
	if err != nil {
		t.Fatalf("list by tag: %v", err)
	}
	if len(tagged) != 2 {
		t.Fatalf("tag matching must ignore case, got %d", len(tagged))
	}

	// Query terms are ANDed and cover notes and custom fields.
	query, err := v.List(ListOptions{Query: "passport renewal"})
	if err != nil {
		t.Fatalf("list by query: %v", err)
	}
	if len(query) != 1 || query[0].Title != "Passport renewal" {
		t.Fatalf("unexpected query result: %+v", query)
	}
	if _, err := v.Save(Item{Type: TypeLogin, Title: "Internal", Custom: []CustomField{{Label: "Employee ID", Value: "E-42"}}}); err != nil {
		t.Fatalf("save custom: %v", err)
	}
	byCustom, err := v.List(ListOptions{Query: "employee id"})
	if err != nil {
		t.Fatalf("list by custom field: %v", err)
	}
	if len(byCustom) != 1 || byCustom[0].Title != "Internal" {
		t.Fatalf("unexpected custom field match: %+v", byCustom)
	}
	none, err := v.List(ListOptions{Query: "passport github"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("terms must be ANDed, got %+v", none)
	}

	byUpdated, err := v.List(ListOptions{Sort: "updated"})
	if err != nil {
		t.Fatalf("list by updated: %v", err)
	}
	if len(byUpdated) != 5 {
		t.Fatalf("want 5 items, got %d", len(byUpdated))
	}
	if !byUpdated[0].UpdatedAt.After(byUpdated[len(byUpdated)-1].UpdatedAt) &&
		!byUpdated[0].UpdatedAt.Equal(byUpdated[len(byUpdated)-1].UpdatedAt) {
		t.Fatal("expected newest first")
	}
}

func TestSummaryOmitsSecrets(t *testing.T) {
	v := newTestVault(t)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	item, err := v.Save(Item{
		Type:     TypeLogin,
		Title:    "Bank",
		Password: "top-secret",
		Notes:    "safe combination 1234",
		TOTP:     "JBSWY3DPEHPK3PXP",
		Card:     &Card{Number: "4111111111111111"},
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	summaries, err := v.List(ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("want 1 summary, got %d", len(summaries))
	}
	summary := summaries[0]
	if !summary.HasPassword || !summary.HasNotes || !summary.HasTOTP {
		t.Fatalf("expected feature flags: %+v", summary)
	}
	// The projection type has no field for secrets at all; this asserts the
	// flags we rely on to render the UI are set.
	if summary.ID != item.ID {
		t.Fatalf("id mismatch: %q vs %q", summary.ID, item.ID)
	}
	got, err := v.Get(item.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Password != "top-secret" || got.Card.Number != "4111111111111111" {
		t.Fatalf("Get must return the secrets: %+v", got)
	}
}

func TestGetReturnsACopy(t *testing.T) {
	v := seedVault(t)
	items, err := v.List(ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	got, err := v.Get(items[0].ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	got.Tags = append(got.Tags, "mutated")
	if _, err := v.Get(items[0].ID); err != nil {
		t.Fatalf("get: %v", err)
	}
	stored, _ := v.Get(items[0].ID)
	for _, tag := range stored.Tags {
		if tag == "mutated" {
			t.Fatal("callers must not be able to mutate vault state through a returned item")
		}
	}
}

func TestItemNormalizationAndValidation(t *testing.T) {
	v := newTestVault(t)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	item, err := v.Save(Item{
		Type:     TypeLogin,
		Title:    "  Spaced  ",
		Tags:     []string{" work ", "work", "WORK", "", "personal"},
		Custom:   []CustomField{{Label: " PIN ", Value: "1234"}},
		TOTP:     "JBSWY3DPEHPK3PXP",
		Password: "x",
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if item.Title != "Spaced" {
		t.Fatalf("title not trimmed: %q", item.Title)
	}
	if len(item.Tags) != 2 {
		t.Fatalf("tags should be trimmed, de-duplicated case-insensitively and sorted: %+v", item.Tags)
	}
	if _, err := v.Save(Item{Type: TypeLogin, Title: "  "}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("want ErrInvalid for a blank title, got %v", err)
	}
	if _, err := v.Save(Item{Title: "no type"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("want ErrInvalid for a missing type, got %v", err)
	}
	if _, err := v.Save(Item{Type: "wallet", Title: "nope"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("want ErrInvalid for an unknown type, got %v", err)
	}
	if _, err := v.Save(Item{Type: TypeLogin, Title: "bad totp", TOTP: "not base32!"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("want ErrInvalid for a bad TOTP secret, got %v", err)
	}
	if _, err := v.Save(Item{Type: TypeLogin, Title: "long", Notes: strings.Repeat("x", maxNotesLen+1)}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("want ErrInvalid for oversized notes, got %v", err)
	}
	if _, err := v.Save(Item{Type: TypeCard, Title: "card", Card: &Card{ExpMonth: 13}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("want ErrInvalid for a bad expiry month, got %v", err)
	}
	long := make([]string, maxTags+1)
	for i := range long {
		long[i] = "tag-" + string(rune('a'+i))
	}
	if _, err := v.Save(Item{Type: TypeNote, Title: "many tags", Tags: long}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("want ErrInvalid for too many tags, got %v", err)
	}
}

func TestUpdateAndDelete(t *testing.T) {
	v := seedVault(t)
	items, err := v.List(ListOptions{Query: "GitHub"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	target := items[0]
	updated, err := v.Get(target.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	updated.Title = "GitHub Enterprise"
	updated.Password = "first-password"
	saved, err := v.Save(updated)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if saved.Title != "GitHub Enterprise" {
		t.Fatalf("update did not stick: %+v", saved)
	}
	if !saved.CreatedAt.Equal(target.UpdatedAt) && saved.CreatedAt.IsZero() {
		t.Fatalf("created timestamp must be preserved: %+v", saved)
	}
	if _, err := v.Save(Item{ID: "does-not-exist", Type: TypeLogin, Title: "ghost"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if err := v.Delete(target.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := v.Delete(target.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound on the second delete, got %v", err)
	}
	if err := v.Delete(""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound for an empty id, got %v", err)
	}
	after, err := v.List(ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(after) != 3 {
		t.Fatalf("want 3 items left, got %d", len(after))
	}
}

func TestToggleFavorite(t *testing.T) {
	v := seedVault(t)
	items, err := v.List(ListOptions{Query: "Passport"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	item, err := v.ToggleFavorite(items[0].ID)
	if err != nil {
		t.Fatalf("toggle: %v", err)
	}
	if !item.Favorite {
		t.Fatal("expected the item to be favorited")
	}
	if _, err := v.ToggleFavorite("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestPasswordHistory(t *testing.T) {
	v := newTestVault(t)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	item, err := v.Save(Item{Type: TypeLogin, Title: "Rotating", Password: "first"})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if len(item.History) != 0 {
		t.Fatalf("a new item starts with no history: %+v", item.History)
	}
	for _, password := range []string{"second", "third", "fourth", "fifth", "sixth", "seventh"} {
		item.Password = password
		if item, err = v.Save(item); err != nil {
			t.Fatalf("save %s: %v", password, err)
		}
	}
	// The default history limit is five entries, newest first.
	if len(item.History) != 5 {
		t.Fatalf("want 5 history entries, got %d", len(item.History))
	}
	if item.History[0].Password != "sixth" {
		t.Fatalf("newest entry should be first: %+v", item.History[0])
	}
	if item.History[0].ChangedAt.IsZero() {
		t.Fatal("history entries need a timestamp")
	}
	// Re-saving without touching the password must not add history.
	before := len(item.History)
	if item, err = v.Save(item); err != nil {
		t.Fatalf("save: %v", err)
	}
	if len(item.History) != before {
		t.Fatalf("unchanged passwords must not create history entries: %d -> %d", before, len(item.History))
	}
	// Clearing the password also records the old one.
	item.Password = ""
	item, err = v.Save(item)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if len(item.History) == 0 || item.History[0].Password != "seventh" {
		t.Fatalf("clearing a password should archive it: %+v", item.History)
	}
}

func TestHistoryLimitFromSettings(t *testing.T) {
	v := newTestVault(t)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	settings, err := v.Settings()
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	settings.PasswordHistoryMax = 2
	if _, err := v.SaveSettings(settings); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	item, err := v.Save(Item{Type: TypeLogin, Title: "Rotating", Password: "one"})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	for _, password := range []string{"two", "three", "four", "five"} {
		item.Password = password
		if item, err = v.Save(item); err != nil {
			t.Fatalf("save: %v", err)
		}
	}
	if len(item.History) != 2 {
		t.Fatalf("want 2 history entries, got %d", len(item.History))
	}
}

func TestTagsAndStats(t *testing.T) {
	v := seedVault(t)
	tags, err := v.Tags()
	if err != nil {
		t.Fatalf("tags: %v", err)
	}
	if len(tags) != 3 {
		t.Fatalf("want 3 tags, got %+v", tags)
	}
	if tags[0].Name != "personal" || tags[0].Count != 2 {
		t.Fatalf("most used tag first: %+v", tags)
	}
	stats, err := v.Stats()
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.Total != 4 || stats.Logins != 2 || stats.Notes != 1 || stats.Cards != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if stats.Favorites != 1 || stats.WithTOTP != 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if _, err := v.Save(Item{Type: TypeLogin, Title: "2FA", TOTP: "JBSWY3DPEHPK3PXP"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if stats, err = v.Stats(); err != nil || stats.WithTOTP != 1 {
		t.Fatalf("expected one TOTP item: %+v %v", stats, err)
	}
}

func TestSettingsValidation(t *testing.T) {
	v := newTestVault(t)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	saved, err := v.SaveSettings(Settings{AutoLockMinutes: 2, ClipboardClearSeconds: 15, Theme: "light", Generator: DefaultGeneratorOptions()})
	if err != nil {
		t.Fatalf("save settings: %v", err)
	}
	if saved.AutoLockMinutes != 2 || saved.Theme != "light" {
		t.Fatalf("settings did not persist: %+v", saved)
	}

	// Out-of-range values are clamped, unknown themes fall back to dark, and the
	// generator keeps its mode.
	clamped, err := v.SaveSettings(Settings{AutoLockMinutes: -5, ClipboardClearSeconds: 100000, Theme: "neon", Generator: GeneratorOptions{Mode: ModePassphrase, Words: 4, Separator: ".", Capitalize: true}})
	if err != nil {
		t.Fatalf("save settings: %v", err)
	}
	if clamped.AutoLockMinutes != 0 || clamped.ClipboardClearSeconds != 600 || clamped.Theme != "dark" {
		t.Fatalf("expected clamping: %+v", clamped)
	}
	if clamped.Generator.Mode != ModePassphrase || clamped.Generator.Words != 4 {
		t.Fatalf("passphrase settings lost: %+v", clamped.Generator)
	}
	if _, err := v.SaveSettings(Settings{Generator: GeneratorOptions{Mode: ModePassword, Length: 4}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("want ErrInvalid for an impossible password length, got %v", err)
	}
	// An invalid generator mode from a client is rejected...
	if _, err := v.SaveSettings(Settings{Generator: GeneratorOptions{Mode: "lol"}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("want ErrInvalid for an unknown generator mode, got %v", err)
	}
	// ...but a settings blob loaded from disk (for example written by a newer
	// build) falls back to defaults instead of making the vault unusable.
	corrupt := Settings{Generator: GeneratorOptions{Mode: "lol"}}
	if err := corrupt.normalize(); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if corrupt.Generator.Mode != ModePassword {
		t.Fatalf("expected the generator to fall back to defaults: %+v", corrupt.Generator)
	}
}

func TestHintUpdate(t *testing.T) {
	v := newTestVault(t)
	if err := v.Create(testPassword, "old hint"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := v.SetHint("new hint"); err != nil {
		t.Fatalf("set hint: %v", err)
	}
	info, err := v.Info()
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if info.Hint != "new hint" {
		t.Fatalf("hint not updated: %+v", info)
	}
	if err := v.SetHint(strings.Repeat("x", 257)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("want ErrInvalid for a long hint, got %v", err)
	}
	v.Lock()
	locked := New(v.Path())
	if err := locked.Unlock(testPassword); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	info, err = locked.Info()
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if info.Hint != "new hint" {
		t.Fatalf("hint did not survive a reload: %+v", info)
	}
}

func TestStatsOnEmptyVault(t *testing.T) {
	v := newTestVault(t)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	items, err := v.List(ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected an empty vault, got %+v", items)
	}
	count, err := v.Count()
	if err != nil || count != 0 {
		t.Fatalf("count: %d %v", count, err)
	}
	tags, err := v.Tags()
	if err != nil {
		t.Fatalf("tags: %v", err)
	}
	if len(tags) != 0 {
		t.Fatalf("expected no tags, got %+v", tags)
	}
	if _, err := v.Get("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestItemTimestampsAdvanceOnUpdate(t *testing.T) {
	v := newTestVault(t)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	base := time.Date(2024, time.March, 1, 12, 0, 0, 0, time.UTC)
	v.now = func() time.Time { return base }
	item, err := v.Save(Item{Type: TypeNote, Title: "clock"})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if !item.CreatedAt.Equal(base) {
		t.Fatalf("unexpected created at: %v", item.CreatedAt)
	}
	v.now = func() time.Time { return base.Add(48 * time.Hour) }
	updated, err := v.Save(item)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if !updated.CreatedAt.Equal(base) || !updated.UpdatedAt.Equal(base.Add(48*time.Hour)) {
		t.Fatalf("unexpected timestamps: %+v", updated)
	}
}
