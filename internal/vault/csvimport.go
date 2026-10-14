package vault

import (
	"encoding/csv"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// CSV import is the migration path into the vault. Every mainstream manager can
// export a CSV, and the column names are close enough that one mapper covers
// Bitwarden, 1Password, KeePass/KeePassXC, Chrome, Edge and the various
// "generic" shapes. The parser is deliberately forgiving about unknown columns
// and strict about the vault's own validation rules.

const (
	// maxImportRows bounds how much a single import can add.
	maxImportRows = 2000
	// maxImportTextLen bounds the pasted/read CSV so the webview cannot hand
	// the app an unbounded string.
	maxImportTextLen = 8 << 20
)

// ImportMode decides what happens to a row that already exists in the vault.
type ImportMode string

const (
	// ImportSkip drops rows whose title and username already exist (default).
	ImportSkip ImportMode = "skip"
	// ImportAdd keeps every row, including near-duplicates.
	ImportAdd ImportMode = "add"
)

// ImportOptions controls a single import run.
type ImportOptions struct {
	// DryRun parses and classifies without changing the vault, which is how
	// the UI shows a preview before the user commits.
	DryRun bool `json:"dryRun"`
	// Mode is ImportSkip or ImportAdd; empty means ImportSkip.
	Mode ImportMode `json:"mode"`
}

// ImportRow is the per-row outcome of an import.
type ImportRow struct {
	Line     int      `json:"line"`
	Title    string   `json:"title"`
	Username string   `json:"username,omitempty"`
	URL      string   `json:"url,omitempty"`
	Type     ItemType `json:"type"`
	// Status is one of new, duplicate, invalid or unsupported.
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// ImportResult summarises a run. Imported is zero on a dry run.
type ImportResult struct {
	Format      string      `json:"format"`
	DryRun      bool        `json:"dryRun"`
	Mode        ImportMode  `json:"mode"`
	Total       int         `json:"total"`
	New         int         `json:"new"`
	Duplicates  int         `json:"duplicates"`
	Invalid     int         `json:"invalid"`
	Unsupported int         `json:"unsupported"`
	Imported    int         `json:"imported"`
	Rows        []ImportRow `json:"rows"`
	// Warnings are rows that were imported with something dropped, such as an
	// unreadable one-time password secret.
	Warnings []string `json:"warnings,omitempty"`
}

// importColumns maps the canonical fields to their column index, or -1.
type importColumns struct {
	title    int
	username int
	password int
	url      int
	notes    int
	totp     int
	kind     int
	folder   int
	favorite int
	tags     int
}

var importAliases = map[string]string{
	// title
	"name": "title", "title": "title", "item": "title", "item name": "title",
	"account": "title", "account name": "title", "entry": "title", "entry name": "title",
	// username
	"username": "username", "login username": "username", "user": "username",
	"user name": "username", "login": "username", "email": "username",
	"login name": "username", "userid": "username",
	// password
	"password": "password", "login password": "password", "pass": "password", "pwd": "password",
	// url
	"url": "url", "login uri": "url", "uri": "url", "website": "url", "web site": "url",
	"login url": "url", "site": "url", "web address": "url", "login uri 1": "url",
	// notes
	"notes": "notes", "note": "notes", "comments": "notes", "comment": "notes",
	"extra": "notes", "memo": "notes", "description": "notes",
	// totp
	"login totp": "totp", "totp": "totp", "otp": "totp", "otpauth": "totp",
	"otp secret": "totp", "one time password": "totp", "otpsecret": "totp",
	// type
	"type": "kind", "kind": "kind", "item type": "kind",
	// folder
	"folder": "folder", "group": "folder", "grouping": "folder", "category": "folder",
	"collection": "folder", "folder name": "folder",
	// favorite
	"favorite": "favorite", "favourite": "favorite",
	// tags
	"tags": "tags", "tag": "tags",
}

// ImportCSV parses text and merges it into the vault. With DryRun set it only
// classifies the rows, which is what the preview step uses.
func (v *Vault) ImportCSV(text string, opts ImportOptions) (ImportResult, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.unlocked {
		return ImportResult{}, ErrLocked
	}
	if opts.Mode != ImportAdd {
		opts.Mode = ImportSkip
	}

	records, err := readCSV(text)
	if err != nil {
		return ImportResult{}, err
	}
	if len(records) == 0 {
		return ImportResult{}, fmt.Errorf("%w: the CSV file is empty", ErrInvalid)
	}

	header := records[0]
	cols, formats := mapImportColumns(header)
	if cols.title < 0 || (cols.password < 0 && cols.notes < 0) {
		return ImportResult{}, fmt.Errorf("%w: this CSV needs at least a title and a password or notes column (found: %s)",
			ErrInvalid, strings.Join(squash(header), ", "))
	}

	result := ImportResult{Format: formatName(formats), DryRun: opts.DryRun, Mode: opts.Mode, Rows: []ImportRow{}}

	// Existing items keyed the way a duplicate is detected: title plus
	// username, case-insensitive. Trashed items do not count, because the user
	// has already decided they are gone.
	seen := map[string]bool{}
	for i := range v.payload.Items {
		if item := &v.payload.Items[i]; !item.Trashed() {
			seen[duplicateKey(item.Title, item.Username)] = true
		}
	}

	now := v.now().UTC().Truncate(time.Second)
	for line := 1; line < len(records) && len(result.Rows) < maxImportRows; line++ {
		record := records[line]
		row := ImportRow{Line: line + 1, Type: TypeLogin, Status: "invalid"}

		title := strings.TrimSpace(cell(record, cols.title))
		username := strings.TrimSpace(cell(record, cols.username))
		password := cell(record, cols.password)
		rawURL := strings.TrimSpace(cell(record, cols.url))
		notes := cell(record, cols.notes)
		if title == "" && username == "" && password == "" && rawURL == "" && notes == "" {
			continue // a blank trailing row
		}
		if title == "" {
			title = fallbackTitle(username, rawURL)
		}
		row.Title, row.Username, row.URL = title, username, rawURL
		if title == "" {
			row.Reason = "the row has no title, username or URL to name the item"
			result.Invalid++
			result.Rows = append(result.Rows, row)
			continue
		}

		kind := classifyImportType(cell(record, cols.kind))
		if kind == "" {
			row.Status = "unsupported"
			row.Reason = fmt.Sprintf("items of type %q are not part of the vault format", strings.TrimSpace(cell(record, cols.kind)))
			result.Unsupported++
			result.Rows = append(result.Rows, row)
			continue
		}
		row.Type = kind

		key := duplicateKey(title, username)
		duplicate := seen[key]
		if duplicate && opts.Mode != ImportAdd {
			row.Status = "duplicate"
			row.Reason = "an item with this title and username is already in the vault"
			result.Duplicates++
			result.Rows = append(result.Rows, row)
			continue
		}

		totp := ""
		if raw := strings.TrimSpace(cell(record, cols.totp)); raw != "" {
			parsed, err := normalizeImportedTOTP(raw)
			if err != nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("line %d (%s): the one-time password secret was ignored: %v", line+1, title, err))
			} else {
				totp = parsed
			}
		}

		item := Item{
			Type:      kind,
			Title:     title,
			Username:  username,
			Password:  password,
			Notes:     notes,
			Favorite:  truthy(cell(record, cols.favorite)),
			Tags:      importTags(cell(record, cols.folder), cell(record, cols.tags)),
			TOTP:      totp,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if kind == TypeNote && rawURL != "" {
			// A secure note has nowhere to put a URL, so keep it in the body
			// rather than silently dropping it.
			if item.Notes == "" {
				item.Notes = rawURL
			} else {
				item.Notes = rawURL + "\n" + item.Notes
			}
		} else {
			item.URL = rawURL
		}
		item.normalize()
		if err := item.validate(); err != nil {
			row.Reason = err.Error()
			result.Invalid++
			result.Rows = append(result.Rows, row)
			continue
		}
		if opts.DryRun {
			row.Status = "new"
			result.New++
			result.Rows = append(result.Rows, row)
			continue
		}

		id, err := newID()
		if err != nil {
			return result, err
		}
		item.ID = id
		v.payload.Items = append(v.payload.Items, item)
		seen[key] = true
		row.Status = "new"
		result.New++
		result.Imported++
		result.Rows = append(result.Rows, row)
	}

	result.Total = len(result.Rows)
	if result.Total >= maxImportRows {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("only the first %d rows were read", maxImportRows))
	}
	if !opts.DryRun && result.Imported > 0 {
		if err := v.persist(); err != nil {
			return result, err
		}
	}
	return result, nil
}

// readCSV decodes the text into records, accepting the field quirks real
// exports contain: a UTF-8 BOM, dangling quotes and ragged rows.
func readCSV(text string) ([][]string, error) {
	if len(text) > maxImportTextLen {
		return nil, fmt.Errorf("%w: that file is larger than %d MiB", ErrInvalid, maxImportTextLen>>20)
	}
	text = strings.TrimPrefix(text, "\ufeff")
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("%w: the CSV file is empty", ErrInvalid)
	}
	reader := csv.NewReader(strings.NewReader(text))
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%w: the file is not readable as CSV: %v", ErrInvalid, err)
	}
	if len(records) > maxImportRows+1 {
		records = records[:maxImportRows+1]
	}
	return records, nil
}

// mapImportColumns resolves the header row into canonical column indices and
// reports which headers were recognised, so the UI can name the source format.
func mapImportColumns(header []string) (importColumns, map[string]bool) {
	cols := importColumns{title: -1, username: -1, password: -1, url: -1, notes: -1, totp: -1, kind: -1, folder: -1, favorite: -1, tags: -1}
	found := map[string]bool{}
	for index, raw := range header {
		name := normalizeHeader(raw)
		found[name] = true
		field, ok := importAliases[name]
		if !ok {
			continue
		}
		switch field {
		case "title":
			if cols.title < 0 {
				cols.title = index
			}
		case "username":
			if cols.username < 0 {
				cols.username = index
			}
		case "password":
			if cols.password < 0 {
				cols.password = index
			}
		case "url":
			if cols.url < 0 {
				cols.url = index
			}
		case "notes":
			if cols.notes < 0 {
				cols.notes = index
			}
		case "totp":
			if cols.totp < 0 {
				cols.totp = index
			}
		case "kind":
			if cols.kind < 0 {
				cols.kind = index
			}
		case "folder":
			if cols.folder < 0 {
				cols.folder = index
			}
		case "favorite":
			if cols.favorite < 0 {
				cols.favorite = index
			}
		case "tags":
			if cols.tags < 0 {
				cols.tags = index
			}
		}
	}
	return cols, found
}

// formatName guesses the exporting product from the headers it emitted. It is
// only used for labelling the preview.
func formatName(headers map[string]bool) string {
	switch {
	case headers["login uri"] || headers["login totp"] || headers["reprompt"]:
		return "Bitwarden"
	case headers["otpauth"] || headers["archived"]:
		return "1Password"
	case headers["group"] && headers["title"]:
		return "KeePass"
	case headers["name"] && !headers["title"]:
		return "Chrome / Edge"
	default:
		return "Generic CSV"
	}
}

func normalizeHeader(raw string) string {
	raw = strings.TrimPrefix(raw, "\ufeff")
	raw = strings.ToLower(strings.TrimSpace(raw))
	replacer := strings.NewReplacer("_", " ", "-", " ", "  ", " ")
	return strings.TrimSpace(replacer.Replace(raw))
}

func cell(record []string, index int) string {
	if index < 0 || index >= len(record) {
		return ""
	}
	return record[index]
}

func squash(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) > 12 {
		out = append(out[:12], "…")
	}
	return out
}

// fallbackTitle names a row after the only other useful field it has.
func fallbackTitle(username, rawURL string) string {
	if username != "" {
		return username
	}
	if rawURL == "" {
		return ""
	}
	candidate := rawURL
	if !strings.Contains(candidate, "://") {
		candidate = "https://" + candidate
	}
	if parsed, err := url.Parse(candidate); err == nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	return strings.TrimSpace(rawURL)
}

func duplicateKey(title, username string) string {
	return strings.ToLower(strings.TrimSpace(title)) + "\x00" + strings.ToLower(strings.TrimSpace(username))
}

// classifyImportType maps the exporter's type column onto the vault's types.
// An empty kind means the row cannot be represented.
func classifyImportType(raw string) ItemType {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "login", "logins", "password", "web", "1":
		return TypeLogin
	case "note", "notes", "secure note", "securenote", "secure_note", "2":
		return TypeNote
	case "card", "credit card", "3", "identity", "4":
		// Bitwarden exports cards and identities with no usable payload, and
		// the vault's card shape is richer than a CSV row can describe.
		return ""
	default:
		return TypeLogin
	}
}

// importTags merges the folder column (one tag) with the tags column (a list).
func importTags(folder, tags string) []string {
	out := []string{}
	if trimmed := strings.TrimSpace(folder); trimmed != "" {
		out = append(out, trimmed)
	}
	for _, tag := range strings.FieldsFunc(tags, func(r rune) bool {
		return r == ',' || r == ';' || r == '/' || r == '|'
	}) {
		if trimmed := strings.TrimSpace(tag); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func truthy(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "y":
		return true
	default:
		return false
	}
}

// normalizeImportedTOTP accepts either a bare base32 secret or a full
// otpauth:// URI and returns a value ParseTOTPSecret understands.
func normalizeImportedTOTP(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	secret := raw
	if strings.HasPrefix(strings.ToLower(raw), "otpauth://") {
		parsed, err := url.Parse(raw)
		if err != nil {
			return "", fmt.Errorf("the otpauth URI could not be read")
		}
		secret = parsed.Query().Get("secret")
		if secret == "" {
			return "", fmt.Errorf("the otpauth URI has no secret parameter")
		}
	}
	if _, err := ParseTOTPSecret(secret); err != nil {
		return "", err
	}
	return secret, nil
}
