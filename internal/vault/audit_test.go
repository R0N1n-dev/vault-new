package vault

import (
	"errors"
	"testing"
	"time"
)

const (
	strongOne = "Q7#vLm2!pRw9Xc4&"
	strongTwo = "tR5@kQw8#zNb3!Ye"
)

// auditVault builds a vault whose clock is fixed, with the auditable fields set
// explicitly so each check can be exercised in isolation. Saving an item always
// stamps it with the current time, so a test that wants an old creation date or
// a rotation history writes those fields straight into the store afterwards.
func auditVault(t *testing.T, now time.Time, items ...Item) *Vault {
	t.Helper()
	v := newTestVault(t)
	v.now = func() time.Time { return now }
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	for _, item := range items {
		saved, err := v.Save(item)
		if err != nil {
			t.Fatalf("save %q: %v", item.Title, err)
		}
		stored := &v.payload.Items[v.indexOf(saved.ID)]
		if !item.CreatedAt.IsZero() {
			stored.CreatedAt = item.CreatedAt
		}
		if len(item.History) > 0 {
			stored.History = item.History
		}
	}
	return v
}

func findingFor(report AuditReport, title string, kind AuditKind) (AuditFinding, bool) {
	for _, finding := range report.Findings {
		if finding.Title == title && finding.Kind == kind {
			return finding, true
		}
	}
	return AuditFinding{}, false
}

func TestAuditOfAnEmptyVaultIsPerfect(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	v := auditVault(t, now)
	report, err := v.Audit()
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if report.Score != 100 || len(report.Findings) != 0 || report.Items != 0 {
		t.Fatalf("unexpected empty report: %+v", report)
	}
	if !report.CheckedAt.Equal(now) {
		t.Fatalf("CheckedAt should follow the vault clock, got %s", report.CheckedAt)
	}
}

func TestAuditFlagsWeakPasswords(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	v := auditVault(t, now,
		Item{Type: TypeLogin, Title: "Common", Username: "a", Password: "letmein"},
		Item{Type: TypeLogin, Title: "Shortish", Username: "b", Password: "abc12345"},
		Item{Type: TypeLogin, Title: "Solid", Username: "c", Password: strongOne},
	)
	report, err := v.Audit()
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if finding, ok := findingFor(report, "Common", KindWeak); !ok {
		t.Fatal("a common password must be flagged")
	} else if finding.Severity != SeverityHigh {
		t.Fatalf("a common password should be high severity, got %s", finding.Severity)
	}
	if _, ok := findingFor(report, "Shortish", KindWeak); !ok {
		t.Fatal("an 8 character alphanumeric password must be flagged")
	}
	if _, ok := findingFor(report, "Solid", KindWeak); ok {
		t.Fatal("a strong generated password must not be flagged")
	}
	if report.Weak != 2 {
		t.Fatalf("want 2 weak items, got %d", report.Weak)
	}
	if report.Score >= 100 {
		t.Fatalf("weak passwords must cost score, got %d", report.Score)
	}
	if report.Affected != 2 {
		t.Fatalf("want 2 affected items, got %d", report.Affected)
	}
}

func TestAuditDetectsReuse(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	v := auditVault(t, now,
		Item{Type: TypeLogin, Title: "Bank", Username: "a", Password: strongOne},
		Item{Type: TypeLogin, Title: "Email", Username: "b", Password: strongOne},
		Item{Type: TypeLogin, Title: "Other", Username: "c", Password: strongTwo},
	)
	report, err := v.AuditWith(AuditOptions{IgnoreWeak: true})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if report.Reused != 2 {
		t.Fatalf("want 2 reused items, got %d", report.Reused)
	}
	finding, ok := findingFor(report, "Bank", KindReused)
	if !ok {
		t.Fatal("reuse must be reported for each item sharing the password")
	}
	if finding.Severity != SeverityHigh {
		t.Fatalf("reuse should be high severity, got %s", finding.Severity)
	}
	if _, ok := findingFor(report, "Other", KindReused); ok {
		t.Fatal("a unique password must not be flagged as reused")
	}
	// Reused items on their own (weak check off) cost 12 each.
	if want := 76; report.Score != want {
		t.Fatalf("want score %d, got %d", want, report.Score)
	}
}

func TestAuditMeasuresPasswordAgeFromTheLastRotation(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	old := now.AddDate(-2, 0, 0)
	rotated := now.AddDate(0, 0, -10)

	v := auditVault(t, now,
		Item{Type: TypeLogin, Title: "Ancient", Username: "a", Password: strongOne, CreatedAt: old},
		Item{
			Type: TypeLogin, Title: "Rotated", Username: "b", Password: strongTwo, CreatedAt: old,
			History: []HistoryEntry{{Password: "previous-password", ChangedAt: rotated}},
		},
	)
	report, err := v.AuditWith(AuditOptions{IgnoreWeak: true})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	finding, ok := findingFor(report, "Ancient", KindStale)
	if !ok {
		t.Fatal("a password that is two years old must be flagged as stale")
	}
	if finding.Severity != SeverityMedium {
		t.Fatalf("stale should be medium severity, got %s", finding.Severity)
	}
	if _, ok := findingFor(report, "Rotated", KindStale); ok {
		t.Fatal("a password rotated ten days ago is not stale")
	}
	if report.Stale != 1 {
		t.Fatalf("want 1 stale item, got %d", report.Stale)
	}
}

func TestAuditSecondFactorAdvisory(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	v := auditVault(t, now,
		Item{Type: TypeLogin, Title: "No 2FA", Username: "a", URL: "https://example.com", Password: strongOne},
		Item{Type: TypeLogin, Title: "Has 2FA", Username: "b", URL: "https://example.com", Password: strongTwo, TOTP: "JBSWY3DPEHPK3PXP"},
		Item{Type: TypeNote, Title: "Just a note", Notes: "nothing to protect"},
	)
	report, err := v.AuditWith(AuditOptions{IgnoreWeak: true})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if _, ok := findingFor(report, "No 2FA", KindNoTOTP); !ok {
		t.Fatal("a login with a URL and no second factor should be flagged")
	}
	if _, ok := findingFor(report, "Has 2FA", KindNoTOTP); ok {
		t.Fatal("a login with a second factor must not be flagged")
	}
	if _, ok := findingFor(report, "Just a note", KindNoTOTP); ok {
		t.Fatal("notes have no second factor to store")
	}
	if report.NoTOTP != 1 {
		t.Fatalf("want 1 advisory, got %d", report.NoTOTP)
	}
}

func TestAuditReportsCardExpiry(t *testing.T) {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	v := auditVault(t, now,
		Item{Type: TypeCard, Title: "Expired", Card: &Card{Number: "4111", ExpMonth: 5, ExpYear: 2026}},
		Item{Type: TypeCard, Title: "Soon", Card: &Card{Number: "4111", ExpMonth: 7, ExpYear: 2026}},
		Item{Type: TypeCard, Title: "Later", Card: &Card{Number: "4111", ExpMonth: 1, ExpYear: 2029}},
	)
	report, err := v.Audit()
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if _, ok := findingFor(report, "Expired", KindCardExpired); !ok {
		t.Fatal("an expired card must be flagged")
	}
	if _, ok := findingFor(report, "Soon", KindCardExpiringSoon); !ok {
		t.Fatal("a card expiring within the warning window must be flagged")
	}
	if _, ok := findingFor(report, "Later", KindCardExpired); ok {
		t.Fatal("a card expiring in three years is fine")
	}
	if report.Cards != 2 {
		t.Fatalf("want 2 card findings, got %d", report.Cards)
	}
}

func TestAuditScoresEachItemOnceByItsWorstFinding(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	old := now.AddDate(-3, 0, 0)
	// One terrible item tripping weak, reused, stale and no-2FA at once.
	v := auditVault(t, now,
		Item{Type: TypeLogin, Title: "Terrible", Username: "a", URL: "https://x.example", Password: "letmein", CreatedAt: old},
		Item{Type: TypeLogin, Title: "Terrible too", Username: "b", URL: "https://y.example", Password: "letmein", CreatedAt: old},
	)
	report, err := v.Audit()
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(report.Findings) < 6 {
		t.Fatalf("expected several findings, got %d", len(report.Findings))
	}
	// Each item is charged only its worst finding (weak = 18).
	if want := 64; report.Score != want {
		t.Fatalf("want score %d, got %d", want, report.Score)
	}
}

func TestAuditScoreNeverGoesBelowZeroAndSortsBySeverity(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	items := make([]Item, 0, 10)
	for index := 0; index < 10; index++ {
		items = append(items, Item{
			Type:      TypeLogin,
			Title:     string(rune('A'+index)) + " account",
			Username:  "user",
			URL:       "https://example.com",
			Password:  "letmein",
			CreatedAt: now.AddDate(-4, 0, 0),
		})
	}
	v := auditVault(t, now, items...)
	report, err := v.Audit()
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if report.Score != 0 {
		t.Fatalf("a vault of ten breached passwords should score 0, got %d", report.Score)
	}
	for i := 1; i < len(report.Findings); i++ {
		if severityRank(report.Findings[i-1].Severity) > severityRank(report.Findings[i].Severity) {
			t.Fatalf("findings are not ordered by severity at %d", i)
		}
	}
	if report.Findings[0].Severity != SeverityHigh {
		t.Fatalf("the first finding should be high severity, got %s", report.Findings[0].Severity)
	}
}

func TestAuditIgnoresTheTrash(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	v := auditVault(t, now,
		Item{Type: TypeLogin, Title: "Live", Username: "a", Password: strongOne},
		Item{Type: TypeLogin, Title: "Deleted", Username: "b", Password: "letmein"},
	)
	live, _ := v.List(ListOptions{Query: "Live"})
	if len(live) != 1 {
		t.Fatalf("expected one live item, got %d", len(live))
	}
	trashed, _ := v.List(ListOptions{Query: "Deleted"})
	if err := v.Delete(trashed[0].ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	report, err := v.Audit()
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if report.Items != 1 || report.Trashed != 1 {
		t.Fatalf("want 1 live and 1 trashed item, got %+v", report)
	}
	if _, ok := findingFor(report, "Deleted", KindWeak); ok {
		t.Fatal("a trashed item must not be audited")
	}
	if report.Score != 100 {
		t.Fatalf("the remaining item is clean, so the score should be 100, got %d", report.Score)
	}
}

func TestAuditRespectsThresholds(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	v := auditVault(t, now,
		Item{Type: TypeLogin, Title: "Aged", Username: "a", Password: strongOne, CreatedAt: now.AddDate(0, 0, -40)},
	)
	strict, err := v.AuditWith(AuditOptions{StaleDays: 30, WeakScore: 1, IgnoreWeak: true})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if _, ok := findingFor(strict, "Aged", KindStale); !ok {
		t.Fatal("a 40 day old password is stale at a 30 day threshold")
	}
	relaxed, err := v.AuditWith(AuditOptions{StaleDays: 365, IgnoreWeak: true})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if _, ok := findingFor(relaxed, "Aged", KindStale); ok {
		t.Fatal("a 40 day old password is not stale at a yearly threshold")
	}
}

func TestAuditIsLockedSafe(t *testing.T) {
	v := newTestVault(t)
	if err := v.Create(testPassword, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	v.Lock()
	if _, err := v.Audit(); !errors.Is(err, ErrLocked) {
		t.Fatalf("want ErrLocked, got %v", err)
	}
}
