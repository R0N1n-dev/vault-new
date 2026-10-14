package vault

import (
	"errors"
	"testing"
	"time"
)

// trashVault returns a seeded vault with a fixed clock so retention windows can
// be crossed without sleeping.
func trashVault(t *testing.T, at time.Time) *Vault {
	t.Helper()
	v := seedVault(t)
	v.now = func() time.Time { return at }
	return v
}

func TestDeleteMovesAnItemToTheTrash(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	v := trashVault(t, now)

	all, err := v.List(ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	target := all[0]

	if err := v.Delete(target.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Gone from every live view.
	after, err := v.List(ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(after) != 3 {
		t.Fatalf("want 3 live items, got %d", len(after))
	}
	if _, err := v.Get(target.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("trashed item must not be readable, got %v", err)
	}
	if _, err := v.List(ListOptions{Query: target.Title}); err != nil {
		t.Fatalf("search: %v", err)
	}
	matches, _ := v.List(ListOptions{Query: target.Title})
	if len(matches) != 0 {
		t.Fatalf("a trashed item must not be searchable, got %d hits", len(matches))
	}
	count, err := v.Count()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Fatalf("want Count 3, got %d", count)
	}
	stats, err := v.Stats()
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.Total != 3 || stats.Trashed != 1 {
		t.Fatalf("want total 3 / trashed 1, got %+v", stats)
	}
	tags, err := v.Tags()
	if err != nil {
		t.Fatalf("tags: %v", err)
	}
	if len(tags) == 0 {
		t.Fatal("expected tags from the remaining items")
	}

	// Present in the trash, stamped with the deletion time.
	trashed, err := v.Trash()
	if err != nil {
		t.Fatalf("trash: %v", err)
	}
	if len(trashed) != 1 {
		t.Fatalf("want 1 trashed item, got %d", len(trashed))
	}
	if trashed[0].ID != target.ID {
		t.Fatalf("wrong item in the trash: %+v", trashed[0])
	}
	if trashed[0].DeletedAt == nil || !trashed[0].DeletedAt.Equal(now) {
		t.Fatalf("want DeletedAt %s, got %v", now, trashed[0].DeletedAt)
	}
	if trashed[0].HasPassword != target.HasPassword {
		t.Fatalf("the trash summary must describe the same item: %+v", trashed[0])
	}

	// Deleting twice, or editing a trashed item, is not allowed.
	if err := v.Delete(target.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound on a second delete, got %v", err)
	}
	if _, err := v.Save(Item{ID: target.ID, Type: TypeLogin, Title: "renamed"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound saving a trashed item, got %v", err)
	}
	if _, err := v.ToggleFavorite(target.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound favouriting a trashed item, got %v", err)
	}
}

func TestTrashSurvivesLockAndUnlock(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	v := trashVault(t, now)
	all, _ := v.List(ListOptions{})
	if err := v.Delete(all[0].ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	v.Lock()
	if err := v.Unlock(testPassword); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	trashed, err := v.Trash()
	if err != nil {
		t.Fatalf("trash: %v", err)
	}
	if len(trashed) != 1 || trashed[0].ID != all[0].ID {
		t.Fatalf("the trash did not survive a round trip: %+v", trashed)
	}
}

func TestRestoreReturnsAnItem(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	v := trashVault(t, now)
	all, _ := v.List(ListOptions{})
	target := all[0]
	if err := v.Delete(target.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	v.now = func() time.Time { return now.Add(2 * time.Hour) }
	restored, err := v.Restore(target.ID)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if restored.Title != target.Title || restored.DeletedAt != nil {
		t.Fatalf("unexpected restored item: %+v", restored)
	}
	if !restored.UpdatedAt.Equal(now.Add(2 * time.Hour)) {
		t.Fatalf("restore must touch UpdatedAt, got %s", restored.UpdatedAt)
	}
	live, _ := v.List(ListOptions{})
	if len(live) != 4 {
		t.Fatalf("want 4 live items after restore, got %d", len(live))
	}
	trashed, _ := v.Trash()
	if len(trashed) != 0 {
		t.Fatalf("the trash should be empty, got %d", len(trashed))
	}
	if _, err := v.Restore(target.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("restoring a live item must fail with ErrNotFound, got %v", err)
	}
	if _, err := v.Restore("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("restoring an unknown id must fail with ErrNotFound, got %v", err)
	}
}

func TestDeleteForeverAndEmptyTrash(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	v := trashVault(t, now)
	all, _ := v.List(ListOptions{})

	if err := v.Delete(all[0].ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := v.DeleteForever(all[0].ID); err != nil {
		t.Fatalf("delete forever: %v", err)
	}
	if err := v.DeleteForever(all[0].ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	trashed, _ := v.Trash()
	if len(trashed) != 0 {
		t.Fatalf("want an empty trash, got %d", len(trashed))
	}

	for _, item := range all[1:] {
		if err := v.Delete(item.ID); err != nil {
			t.Fatalf("delete %s: %v", item.Title, err)
		}
	}
	removed, err := v.EmptyTrash()
	if err != nil {
		t.Fatalf("empty trash: %v", err)
	}
	if removed != 3 {
		t.Fatalf("want 3 removed, got %d", removed)
	}
	if removed, err := v.EmptyTrash(); err != nil || removed != 0 {
		t.Fatalf("emptying an empty trash must be a no-op, got %d / %v", removed, err)
	}
	live, _ := v.List(ListOptions{})
	if len(live) != 0 {
		t.Fatalf("want an empty vault, got %d items", len(live))
	}
}

func TestTrashIsPurgedAfterRetention(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	v := trashVault(t, now)
	all, _ := v.List(ListOptions{})
	victim := all[0]
	if err := v.Delete(victim.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Still inside the default 30 day window: any write must keep it.
	v.now = func() time.Time { return now.Add(29 * 24 * time.Hour) }
	if _, err := v.Save(Item{Type: TypeNote, Title: "keep me", Notes: "x"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if trashed, _ := v.Trash(); len(trashed) != 1 {
		t.Fatalf("an item inside the window must survive, got %d", len(trashed))
	}

	// Past the window, the next write sweeps it out of the file for good.
	v.now = func() time.Time { return now.Add(31 * 24 * time.Hour) }
	if _, err := v.Save(Item{Type: TypeNote, Title: "keep me too", Notes: "y"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if trashed, _ := v.Trash(); len(trashed) != 0 {
		t.Fatalf("the expired item should be purged, got %d", len(trashed))
	}
	if _, err := v.Get(victim.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("purged item must be gone, got %v", err)
	}
	// And it is really gone from disk, not just from memory.
	v.Lock()
	if err := v.Unlock(testPassword); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if trashed, _ := v.Trash(); len(trashed) != 0 {
		t.Fatalf("purge was not persisted, got %d trashed", len(trashed))
	}
}

func TestTrashRetentionCanBeDisabled(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	v := trashVault(t, now)
	settings, err := v.Settings()
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	settings.TrashRetentionDays = 0
	if _, err := v.SaveSettings(settings); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	all, _ := v.List(ListOptions{})
	if err := v.Delete(all[0].ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	v.now = func() time.Time { return now.AddDate(5, 0, 0) }
	if _, err := v.Save(Item{Type: TypeNote, Title: "later", Notes: "x"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if trashed, _ := v.Trash(); len(trashed) != 1 {
		t.Fatalf("with retention disabled nothing should be purged, got %d", len(trashed))
	}
}

func TestTrashIsSortedNewestFirst(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	v := trashVault(t, base)
	all, _ := v.List(ListOptions{})
	for index, item := range all {
		v.now = func() time.Time { return base.Add(time.Duration(index) * time.Hour) }
		if err := v.Delete(item.ID); err != nil {
			t.Fatalf("delete %s: %v", item.Title, err)
		}
	}
	trashed, err := v.Trash()
	if err != nil {
		t.Fatalf("trash: %v", err)
	}
	if len(trashed) != len(all) {
		t.Fatalf("want %d trashed items, got %d", len(all), len(trashed))
	}
	for i := 1; i < len(trashed); i++ {
		previous, current := trashed[i-1].DeletedAt, trashed[i].DeletedAt
		if previous == nil || current == nil || previous.Before(*current) {
			t.Fatalf("trash is not newest-first at %d: %v then %v", i, previous, current)
		}
	}
}

func TestLockedVaultRefusesTrashOperations(t *testing.T) {
	v := newTestVault(t)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	v.Lock()
	if _, err := v.Trash(); !errors.Is(err, ErrLocked) {
		t.Fatalf("Trash: want ErrLocked, got %v", err)
	}
	if _, err := v.Restore("x"); !errors.Is(err, ErrLocked) {
		t.Fatalf("Restore: want ErrLocked, got %v", err)
	}
	if err := v.DeleteForever("x"); !errors.Is(err, ErrLocked) {
		t.Fatalf("DeleteForever: want ErrLocked, got %v", err)
	}
	if _, err := v.EmptyTrash(); !errors.Is(err, ErrLocked) {
		t.Fatalf("EmptyTrash: want ErrLocked, got %v", err)
	}
}
