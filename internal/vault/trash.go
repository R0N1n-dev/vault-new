package vault

import (
	"sort"
	"time"
)

// The trash keeps a deleted item's ciphertext in the same vault file, so
// "delete" is reversible until the retention window closes. Nothing about the
// trash is stored in the clear: a trashed item is just a normal item with a
// DeletedAt stamp, and Trashed items are excluded from List, Get, Tags, Stats
// and search.

// Trash lists the deleted items, most recently deleted first.
func (v *Vault) Trash() ([]Summary, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if !v.unlocked {
		return nil, ErrLocked
	}
	out := make([]Summary, 0, len(v.payload.Items))
	for i := range v.payload.Items {
		item := &v.payload.Items[i]
		if item.Trashed() {
			out = append(out, item.Summary())
		}
	}
	sort.SliceStable(out, func(a, b int) bool {
		left, right := out[a].DeletedAt, out[b].DeletedAt
		switch {
		case left == nil:
			return false
		case right == nil:
			return true
		case left.Equal(*right):
			return out[a].Title < out[b].Title
		default:
			return left.After(*right)
		}
	})
	return out, nil
}

// Restore returns a trashed item to the vault.
func (v *Vault) Restore(id string) (Item, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.unlocked {
		return Item{}, ErrLocked
	}
	idx := v.indexOf(id)
	if idx < 0 || !v.payload.Items[idx].Trashed() {
		// Not in the trash: either it never existed or it is already active.
		return Item{}, ErrNotFound
	}
	item := &v.payload.Items[idx]
	item.DeletedAt = nil
	item.UpdatedAt = v.now().UTC().Truncate(time.Second)
	if err := v.persist(); err != nil {
		return Item{}, err
	}
	return *cloneItem(item), nil
}

// DeleteForever removes an item from the vault without touching the trash.
func (v *Vault) DeleteForever(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.unlocked {
		return ErrLocked
	}
	idx := v.indexOf(id)
	if idx < 0 {
		return ErrNotFound
	}
	v.payload.Items = append(v.payload.Items[:idx], v.payload.Items[idx+1:]...)
	return v.persist()
}

// EmptyTrash removes every trashed item and reports how many were dropped.
func (v *Vault) EmptyTrash() (int, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.unlocked {
		return 0, ErrLocked
	}
	kept := v.payload.Items[:0]
	removed := 0
	for _, item := range v.payload.Items {
		if item.Trashed() {
			removed++
			continue
		}
		kept = append(kept, item)
	}
	if removed == 0 {
		return 0, nil
	}
	v.payload.Items = kept
	if err := v.persist(); err != nil {
		return 0, err
	}
	return removed, nil
}

// purgeExpired drops trashed items whose retention window has closed. The
// caller must hold the write lock; it is invoked from persist so that every
// write also sweeps the trash.
func (v *Vault) purgeExpired() int {
	days := v.payload.Settings.TrashRetentionDays
	if days <= 0 {
		return 0
	}
	cutoff := v.now().UTC().AddDate(0, 0, -days)
	kept := v.payload.Items[:0]
	removed := 0
	for _, item := range v.payload.Items {
		if deleted := item.DeletedAt; deleted != nil && deleted.Before(cutoff) {
			removed++
			continue
		}
		kept = append(kept, item)
	}
	if removed > 0 {
		v.payload.Items = kept
	}
	return removed
}
