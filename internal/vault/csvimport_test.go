package vault

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const bitwardenCSV = `folder,favorite,type,name,notes,fields,reprompt,login_uri,login_username,login_password,login_totp
Work,1,login,GitHub,primary dev account,,,https://github.com,ada,pw-github,JBSWY3DPEHPK3PXP
Personal,0,login,Fastmail,mail,,,https://fastmail.com,ada@example.com,pw-fastmail,
Personal,0,note,Passport renewal,"expires 2031, renew early",,,,,
Personal,0,card,Visa travel,,,,,,
`

const chromeCSV = "name,url,username,password,note\nExample,https://example.com,ada,pw-example,hello\nSecond,https://second.example,bob,pw-second,\n"

const onePasswordCSV = "Title,Url,Username,Password,OTPAuth,Favorite,Archived,Tags,Notes\nBank,https://bank.example,ada,pw-bank,otpauth://totp/Bank?secret=JBSWY3DPEHPK3PXP&issuer=Bank,true,,money;bank,savings\n"

const keepassCSV = "Group,Title,Username,Password,URL,Notes,TOTP\nInternet,Router admin,admin,pw-router,http://192.168.1.1,firmware 1.4,\n"

func importVault(t *testing.T) *Vault {
	t.Helper()
	v := newTestVault(t)
	v.now = func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) }
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	return v
}

func titles(t *testing.T, v *Vault) []string {
	t.Helper()
	items, err := v.List(ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Title)
	}
	return out
}

func TestImportBitwardenCSV(t *testing.T) {
	v := importVault(t)
	result, err := v.ImportCSV(bitwardenCSV, ImportOptions{})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Format != "Bitwarden" {
		t.Fatalf("want the Bitwarden format, got %q", result.Format)
	}
	if result.Imported != 3 {
		t.Fatalf("want 2 logins and 1 note, got %d (rows: %+v)", result.Imported, result.Rows)
	}
	if result.New != 3 || result.Invalid != 0 {
		t.Fatalf("unexpected classification: %+v", result)
	}
	if result.Unsupported != 1 {
		t.Fatalf("Bitwarden card rows cannot be represented, got %d unsupported", result.Unsupported)
	}

	items, err := v.List(ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("want 3 items, got %d: %v", len(items), titles(t, v))
	}

	github, err := v.Get(items[0].ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if github.Title != "GitHub" {
		github, err = v.Get(items[1].ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
	}
	if github.Username != "ada" || github.Password != "pw-github" || github.URL != "https://github.com" {
		t.Fatalf("the login fields did not survive the import: %+v", github)
	}
	if !github.Favorite {
		t.Fatal("the favorite flag must carry over")
	}
	if len(github.Tags) != 1 || github.Tags[0] != "Work" {
		t.Fatalf("the folder must become a tag, got %v", github.Tags)
	}
	if github.TOTP != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("the totp secret must carry over, got %q", github.TOTP)
	}
	// The imported secret must actually produce codes.
	code, err := CodeForSecret(github.TOTP, time.Now())
	if err != nil || len(code.Code) != 6 {
		t.Fatalf("imported totp is unusable: %+v %v", code, err)
	}

	note, err := v.Get(items[2].ID)
	if err != nil {
		t.Fatalf("get note: %v", err)
	}
	if note.Type != TypeNote {
		t.Fatalf("want a secure note, got %s", note.Type)
	}
	if !strings.Contains(note.Notes, "renew early") {
		t.Fatalf("quoted notes with a comma must survive, got %q", note.Notes)
	}
}

func TestImportChromeAndKeePassAndOnePassword(t *testing.T) {
	cases := []struct {
		name     string
		csv      string
		format   string
		title    string
		user     string
		tags     []string
		imported int
	}{
		{"chrome", chromeCSV, "Chrome / Edge", "Example", "ada", nil, 2},
		{"keepass", keepassCSV, "KeePass", "Router admin", "admin", []string{"Internet"}, 1},
		{"1password", onePasswordCSV, "1Password", "Bank", "ada", []string{"money", "bank"}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := importVault(t)
			result, err := v.ImportCSV(tc.csv, ImportOptions{})
			if err != nil {
				t.Fatalf("import: %v", err)
			}
			if result.Format != tc.format {
				t.Fatalf("want format %q, got %q", tc.format, result.Format)
			}
			if result.Imported != tc.imported {
				t.Fatalf("want %d imports, got %d (rows %+v)", tc.imported, result.Imported, result.Rows)
			}
			all, err := v.List(ListOptions{})
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			var id string
			for _, summary := range all {
				if summary.Title == tc.title {
					id = summary.ID
				}
			}
			if id == "" {
				t.Fatalf("no item titled %q in %v", tc.title, titles(t, v))
			}
			item, err := v.Get(id)
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			if item.Title != tc.title || item.Username != tc.user {
				t.Fatalf("unexpected item: %+v", item)
			}
			if len(tc.tags) > 0 {
				for _, tag := range tc.tags {
					found := false
					for _, got := range item.Tags {
						if strings.EqualFold(got, tag) {
							found = true
						}
					}
					if !found {
						t.Fatalf("tag %q missing from %v", tag, item.Tags)
					}
				}
			}
			if tc.name == "1password" && item.TOTP != "JBSWY3DPEHPK3PXP" {
				t.Fatalf("the otpauth URI must be reduced to its secret, got %q", item.TOTP)
			}
		})
	}
}

func TestImportSkipsDuplicatesByDefault(t *testing.T) {
	v := importVault(t)
	if _, err := v.Save(Item{Type: TypeLogin, Title: "GitHub", Username: "ada", Password: "existing"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	result, err := v.ImportCSV(bitwardenCSV, ImportOptions{})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Duplicates != 1 || result.Imported != 2 {
		t.Fatalf("want 1 duplicate and 2 imports, got %+v", result)
	}
	row := result.Rows[0]
	if row.Status != "duplicate" || row.Title != "GitHub" {
		t.Fatalf("unexpected first row: %+v", row)
	}
	if row.Line != 2 {
		t.Fatalf("the row should report its CSV line number, got %d", row.Line)
	}
	// The existing item was not touched.
	items, _ := v.List(ListOptions{Query: "GitHub"})
	item, _ := v.Get(items[0].ID)
	if item.Password != "existing" {
		t.Fatalf("the existing password must not be overwritten, got %q", item.Password)
	}
}

func TestImportAddModeKeepsDuplicates(t *testing.T) {
	v := importVault(t)
	if _, err := v.Save(Item{Type: TypeLogin, Title: "GitHub", Username: "ada", Password: "existing"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	result, err := v.ImportCSV(bitwardenCSV, ImportOptions{Mode: ImportAdd})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Imported != 3 || result.Duplicates != 0 {
		t.Fatalf("add mode must keep everything, got %+v", result)
	}
	items, _ := v.List(ListOptions{Query: "GitHub"})
	if len(items) != 2 {
		t.Fatalf("want both GitHub entries, got %d", len(items))
	}
}

func TestImportDryRunChangesNothing(t *testing.T) {
	v := importVault(t)
	result, err := v.ImportCSV(bitwardenCSV, ImportOptions{DryRun: true})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if !result.DryRun || result.New != 3 || result.Imported != 0 {
		t.Fatalf("unexpected dry run result: %+v", result)
	}
	if len(titles(t, v)) != 0 {
		t.Fatalf("a dry run must not write items: %v", titles(t, v))
	}
	if result.Rows[0].Status != "new" {
		t.Fatalf("a dry run should still classify rows, got %+v", result.Rows[0])
	}
}

func TestImportPersistsToDisk(t *testing.T) {
	v := importVault(t)
	if _, err := v.ImportCSV(chromeCSV, ImportOptions{}); err != nil {
		t.Fatalf("import: %v", err)
	}
	v.Lock()
	if err := v.Unlock(testPassword); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	got := titles(t, v)
	if len(got) != 2 {
		t.Fatalf("want 2 items after a round trip, got %v", got)
	}
}

func TestImportRejectsUnusableInput(t *testing.T) {
	v := importVault(t)
	if _, err := v.ImportCSV("", ImportOptions{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("an empty file must be rejected, got %v", err)
	}
	if _, err := v.ImportCSV("\ufeff\n   \n", ImportOptions{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("a whitespace-only file must be rejected, got %v", err)
	}
	if _, err := v.ImportCSV("title,url\nA,https://a.example\n", ImportOptions{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("a header with no password or notes column must be rejected, got %v", err)
	}
	if _, err := v.ImportCSV("id,label,value\n1,x,y\n", ImportOptions{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("an unrecognised header must be rejected, got %v", err)
	}
}

func TestImportSkipsBlankRowsAndSurvivesRaggedOnes(t *testing.T) {
	v := importVault(t)
	csvText := "\ufeffname,url,username,password\r\nA,https://a.example,ada,pw-a\r\n,,,,\r\nB,https://b.example,bob,pw-b,extra-column\r\n"
	result, err := v.ImportCSV(csvText, ImportOptions{})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Imported != 2 {
		t.Fatalf("want 2 imports, got %d (%+v)", result.Imported, result.Rows)
	}
	if len(titles(t, v)) != 2 {
		t.Fatalf("unexpected titles: %v", titles(t, v))
	}
}

func TestImportFallsBackToUsernameOrHostForATitle(t *testing.T) {
	v := importVault(t)
	csvText := "name,url,username,password\n,https://fallback.example,ada,pw-a\n,https://second.example,,pw-b\n"
	result, err := v.ImportCSV(csvText, ImportOptions{})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Imported != 2 {
		t.Fatalf("want 2 imports, got %+v", result)
	}
	got := titles(t, v)
	if got[0] != "ada" {
		t.Fatalf("want the username as the title, got %v", got)
	}
	if got[1] != "second.example" {
		t.Fatalf("want the host as the title, got %v", got)
	}
}

func TestImportReportsAWarningForABadOneTimeSecret(t *testing.T) {
	v := importVault(t)
	csvText := "name,url,username,password,totp\nOdd,https://odd.example,ada,pw-a,not-base32!!\nMissing,https://m.example,bob,pw-b,otpauth://totp/x?issuer=nope\n"
	result, err := v.ImportCSV(csvText, ImportOptions{})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Imported != 2 {
		t.Fatalf("a bad secret must not block the row, got %+v", result)
	}
	if len(result.Warnings) != 2 {
		t.Fatalf("want a warning per bad secret, got %v", result.Warnings)
	}
	items, _ := v.List(ListOptions{Query: "Odd"})
	item, _ := v.Get(items[0].ID)
	if item.TOTP != "" {
		t.Fatalf("the unusable secret must not be stored, got %q", item.TOTP)
	}
	if item.Password != "pw-a" {
		t.Fatalf("the rest of the row must still be imported: %+v", item)
	}
}

func TestImportRejectsRowsTheVaultWouldNotStore(t *testing.T) {
	v := importVault(t)
	long := strings.Repeat("x", maxFieldLen+10)
	csvText := "name,url,username,password\n" + long + ",https://long.example,ada,pw-a\n"
	result, err := v.ImportCSV(csvText, ImportOptions{})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Invalid != 1 || result.Imported != 0 {
		t.Fatalf("an oversized field must be rejected, got %+v", result)
	}
	if result.Rows[0].Reason == "" {
		t.Fatal("a rejected row must explain itself")
	}
}

func TestImportCapsTheNumberOfRows(t *testing.T) {
	v := importVault(t)
	var builder strings.Builder
	builder.WriteString("name,url,username,password\n")
	for index := 0; index < maxImportRows+50; index++ {
		builder.WriteString("Item ")
		builder.WriteString(strings.Repeat("a", index%5))
		builder.WriteString(string(rune('A' + index%26)))
		builder.WriteString(" ")
		builder.WriteString(itoa(index))
		builder.WriteString(",https://x.example,u,pw\n")
	}
	result, err := v.ImportCSV(builder.String(), ImportOptions{})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Imported > maxImportRows {
		t.Fatalf("an import must never exceed %d rows, got %d", maxImportRows, result.Imported)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("a truncated import must say so")
	}
}

func TestImportRejectsHugeFiles(t *testing.T) {
	v := importVault(t)
	huge := "name,url,username,password\n" + strings.Repeat("a", maxImportTextLen+1)
	if _, err := v.ImportCSV(huge, ImportOptions{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("want ErrInvalid for an oversized file, got %v", err)
	}
}

func TestImportIsLockedSafe(t *testing.T) {
	v := newTestVault(t)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	v.Lock()
	if _, err := v.ImportCSV(chromeCSV, ImportOptions{}); !errors.Is(err, ErrLocked) {
		t.Fatalf("want ErrLocked, got %v", err)
	}
}

func TestImportKeepsTrashedItemsOutOfTheDuplicateSet(t *testing.T) {
	v := importVault(t)
	if _, err := v.Save(Item{Type: TypeLogin, Title: "GitHub", Username: "ada", Password: "existing"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	items, _ := v.List(ListOptions{Query: "GitHub"})
	if err := v.Delete(items[0].ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	result, err := v.ImportCSV(bitwardenCSV, ImportOptions{})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Duplicates != 0 || result.Imported != 3 {
		t.Fatalf("a trashed item must not block an import, got %+v", result)
	}
}

// itoa avoids pulling strconv into the test for one call.
func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := ""
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	return digits
}
