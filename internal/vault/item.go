package vault

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ItemType discriminates the shape of an item's secret payload.
type ItemType string

const (
	TypeLogin ItemType = "login"
	TypeNote  ItemType = "note"
	TypeCard  ItemType = "card"
)

// Field limits. They are generous for humans and tight enough that a runaway
// frontend cannot balloon the vault file.
const (
	maxTitleLen = 512
	maxFieldLen = 4096
	maxNotesLen = 64 * 1024
	maxTags     = 32
	maxTagLen   = 64
	maxCustom   = 40
	maxHistory  = 20
)

// HistoryEntry records a previous password so a rotated credential can be
// recovered after the fact.
type HistoryEntry struct {
	Password  string    `json:"password"`
	ChangedAt time.Time `json:"changedAt"`
}

// CustomField is a user-defined label/value pair; Secret fields are masked in
// the UI and copied like passwords.
type CustomField struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Secret bool   `json:"secret,omitempty"`
}

// Card holds payment card details for TypeCard items.
type Card struct {
	Cardholder string `json:"cardholder,omitempty"`
	Number     string `json:"number,omitempty"`
	Brand      string `json:"brand,omitempty"`
	ExpMonth   int    `json:"expMonth,omitempty"`
	ExpYear    int    `json:"expYear,omitempty"`
	CVC        string `json:"cvc,omitempty"`
}

// Item is one entry in the vault. Every field here is encrypted at rest.
type Item struct {
	ID        string         `json:"id"`
	Type      ItemType       `json:"type"`
	Title     string         `json:"title"`
	Username  string         `json:"username,omitempty"`
	Password  string         `json:"password,omitempty"`
	URL       string         `json:"url,omitempty"`
	Notes     string         `json:"notes,omitempty"`
	Tags      []string       `json:"tags,omitempty"`
	Favorite  bool           `json:"favorite,omitempty"`
	TOTP      string         `json:"totp,omitempty"`
	Card      *Card          `json:"card,omitempty"`
	Custom    []CustomField  `json:"custom,omitempty"`
	History   []HistoryEntry `json:"history,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	// DeletedAt is set while the item sits in the trash. Trashed items stay
	// encrypted in the same file but are invisible to List, Get and search
	// until they are restored or purged.
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

// Trashed reports whether the item is in the trash.
func (i *Item) Trashed() bool { return i.DeletedAt != nil }

// Summary is the list/search projection of an item. It intentionally omits
// passwords, notes, TOTP seeds and card numbers so that the browsing list the
// UI holds never contains secrets.
type Summary struct {
	ID          string     `json:"id"`
	Type        ItemType   `json:"type"`
	Title       string     `json:"title"`
	Username    string     `json:"username,omitempty"`
	URL         string     `json:"url,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
	Favorite    bool       `json:"favorite"`
	HasPassword bool       `json:"hasPassword"`
	HasTOTP     bool       `json:"hasTOTP"`
	HasNotes    bool       `json:"hasNotes"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	DeletedAt   *time.Time `json:"deletedAt,omitempty"`
}

func (i *Item) Summary() Summary {
	return Summary{
		ID:          i.ID,
		Type:        i.Type,
		Title:       i.Title,
		Username:    i.Username,
		URL:         i.URL,
		Tags:        append([]string(nil), i.Tags...),
		Favorite:    i.Favorite,
		HasPassword: i.Password != "",
		HasTOTP:     i.TOTP != "",
		HasNotes:    i.Notes != "",
		UpdatedAt:   i.UpdatedAt,
		DeletedAt:   i.DeletedAt,
	}
}

// normalize trims and de-duplicates user input. Empty values collapse to zero
// values so that an item saved with a blank field stays blank instead of
// storing whitespace.
func (i *Item) normalize() {
	i.Title = strings.TrimSpace(i.Title)
	i.Username = strings.TrimSpace(i.Username)
	i.URL = strings.TrimSpace(i.URL)
	i.Notes = strings.TrimRight(i.Notes, "\n")
	i.TOTP = strings.TrimSpace(i.TOTP)

	tags := make([]string, 0, len(i.Tags))
	seen := map[string]bool{}
	for _, t := range i.Tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		key := strings.ToLower(t)
		if seen[key] {
			continue
		}
		seen[key] = true
		tags = append(tags, t)
	}
	sort.Strings(tags)
	i.Tags = tags
	if len(tags) == 0 {
		i.Tags = nil
	}

	if i.Card != nil && *i.Card == (Card{}) {
		i.Card = nil
	}
	if len(i.History) == 0 {
		i.History = nil
	}
	// A caller cannot move an item in or out of the trash by saving it; the
	// restore path is the only way back.
	i.DeletedAt = nil
}

func (i *Item) validate() error {
	switch i.Type {
	case TypeLogin, TypeNote, TypeCard:
	case "":
		return fmt.Errorf("%w: item type is required", ErrInvalid)
	default:
		return fmt.Errorf("%w: unknown item type %q", ErrInvalid, i.Type)
	}
	if i.Title == "" {
		return fmt.Errorf("%w: title is required", ErrInvalid)
	}
	if len(i.Title) > maxTitleLen {
		return fmt.Errorf("%w: title is longer than %d characters", ErrInvalid, maxTitleLen)
	}
	if len(i.Notes) > maxNotesLen {
		return fmt.Errorf("%w: notes are longer than %d characters", ErrInvalid, maxNotesLen)
	}
	for _, f := range []struct {
		name  string
		value string
	}{{"username", i.Username}, {"url", i.URL}, {"password", i.Password}, {"one-time password secret", i.TOTP}} {
		if len(f.value) > maxFieldLen {
			return fmt.Errorf("%w: %s is longer than %d characters", ErrInvalid, f.name, maxFieldLen)
		}
	}
	if len(i.Tags) > maxTags {
		return fmt.Errorf("%w: an item can hold at most %d tags", ErrInvalid, maxTags)
	}
	for _, t := range i.Tags {
		if len(t) > maxTagLen {
			return fmt.Errorf("%w: tags are limited to %d characters", ErrInvalid, maxTagLen)
		}
	}
	if len(i.Custom) > maxCustom {
		return fmt.Errorf("%w: an item can hold at most %d custom fields", ErrInvalid, maxCustom)
	}
	for _, f := range i.Custom {
		if strings.TrimSpace(f.Label) == "" {
			return fmt.Errorf("%w: custom fields need a label", ErrInvalid)
		}
		if len(f.Label) > maxTagLen || len(f.Value) > maxFieldLen {
			return fmt.Errorf("%w: custom field %q is too long", ErrInvalid, f.Label)
		}
	}
	if i.TOTP != "" {
		if _, err := ParseTOTPSecret(i.TOTP); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalid, err)
		}
	}
	if i.Card != nil {
		if len(i.Card.Number) > maxFieldLen || len(i.Card.Cardholder) > maxTitleLen || len(i.Card.CVC) > 8 {
			return fmt.Errorf("%w: card details are too long", ErrInvalid)
		}
		if i.Card.ExpMonth < 0 || i.Card.ExpMonth > 12 {
			return fmt.Errorf("%w: card expiry month must be between 1 and 12", ErrInvalid)
		}
		if i.Card.ExpYear < 0 || i.Card.ExpYear > 9999 {
			return fmt.Errorf("%w: card expiry year is out of range", ErrInvalid)
		}
	}
	return nil
}

// searchable is the lowercase haystack used by ListOptions.Query.
func (i *Item) searchable() string {
	var b strings.Builder
	b.WriteString(strings.ToLower(i.Title))
	b.WriteByte('\n')
	b.WriteString(strings.ToLower(i.Username))
	b.WriteByte('\n')
	b.WriteString(strings.ToLower(i.URL))
	b.WriteByte('\n')
	b.WriteString(strings.ToLower(strings.Join(i.Tags, " ")))
	b.WriteByte('\n')
	b.WriteString(strings.ToLower(i.Notes))
	for _, f := range i.Custom {
		b.WriteByte('\n')
		b.WriteString(strings.ToLower(f.Label))
	}
	return b.String()
}

// ListOptions filters and orders List results.
type ListOptions struct {
	Query         string   `json:"query"`
	Type          ItemType `json:"type"`
	Tag           string   `json:"tag"`
	FavoritesOnly bool     `json:"favoritesOnly"`
	Sort          string   `json:"sort"`
}

func (o ListOptions) match(i *Item) bool {
	if o.FavoritesOnly && !i.Favorite {
		return false
	}
	if o.Type != "" && i.Type != o.Type {
		return false
	}
	if o.Tag != "" {
		found := false
		for _, t := range i.Tags {
			if strings.EqualFold(t, o.Tag) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if q := strings.ToLower(strings.TrimSpace(o.Query)); q != "" {
		// Every whitespace-separated term must appear, so "git work" narrows
		// rather than widens the result set.
		for _, term := range strings.Fields(q) {
			if !strings.Contains(i.searchable(), term) {
				return false
			}
		}
	}
	return true
}

func (o ListOptions) sort(items []*Item) {
	switch o.Sort {
	case "updated":
		sort.SliceStable(items, func(a, b int) bool { return items[a].UpdatedAt.After(items[b].UpdatedAt) })
	case "created":
		sort.SliceStable(items, func(a, b int) bool { return items[a].CreatedAt.After(items[b].CreatedAt) })
	default:
		sort.SliceStable(items, func(a, b int) bool {
			at, bt := strings.ToLower(items[a].Title), strings.ToLower(items[b].Title)
			if at == bt {
				return strings.ToLower(items[a].Username) < strings.ToLower(items[b].Username)
			}
			return at < bt
		})
	}
}
