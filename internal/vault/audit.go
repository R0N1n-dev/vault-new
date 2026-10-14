package vault

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// The audit is an offline health check over the decrypted vault. It exists
// because the failure mode of a password manager is not a crash: it is a vault
// full of passwords that were already in a breach corpus, copied across five
// sites, or set in 2014 and never rotated.
//
// Nothing here talks to the network. There is no breach corpus lookup, no
// telemetry and no hashing of passwords against a remote service; every verdict
// is derived from the vault contents alone.

// AuditSeverity ranks a finding so the UI can colour and order it.
type AuditSeverity string

const (
	SeverityHigh   AuditSeverity = "high"
	SeverityMedium AuditSeverity = "medium"
	SeverityLow    AuditSeverity = "low"
)

// AuditKind names the check that produced a finding.
type AuditKind string

const (
	// KindWeak means the strength meter scored the password at or below the
	// configured threshold.
	KindWeak AuditKind = "weak"
	// KindReused means the same password protects more than one item.
	KindReused AuditKind = "reused"
	// KindStale means the password has not been changed within the window.
	KindStale AuditKind = "stale"
	// KindNoTOTP is advisory: a login with a URL that has no second factor.
	KindNoTOTP AuditKind = "noTotp"
	// KindCardExpired and KindCardExpiringSoon cover payment card dates.
	KindCardExpired      AuditKind = "cardExpired"
	KindCardExpiringSoon AuditKind = "cardExpiringSoon"
)

// AuditOptions tunes the thresholds. DefaultAuditOptions is what the app uses;
// tests override the fields to keep the checks deterministic.
type AuditOptions struct {
	// StaleDays flags a password older than this. The age is measured from the
	// last recorded password change, not from the last edit of the item.
	StaleDays int
	// WeakScore flags passwords whose strength score is at or below this value
	// (the scale is 0..4).
	WeakScore int
	// CardWarningDays flags a card that expires within this many days.
	CardWarningDays int
	// IgnoreCommonPasswords disables the weak check entirely, which is only
	// useful for tests that want to isolate the other checks.
	IgnoreWeak bool
}

// DefaultAuditOptions returns the shipped thresholds: rotate yearly, flag
// anything "Fair" or worse, and warn two months ahead of a card expiry.
func DefaultAuditOptions() AuditOptions {
	return AuditOptions{StaleDays: 365, WeakScore: 2, CardWarningDays: 60}
}

// AuditFinding is one problem attached to one item.
type AuditFinding struct {
	Kind     AuditKind     `json:"kind"`
	Severity AuditSeverity `json:"severity"`
	ItemID   string        `json:"itemId"`
	Title    string        `json:"title"`
	Detail   string        `json:"detail"`
	// Priority orders findings inside the same severity; it is what the score
	// subtracts for this item.
	Priority int `json:"priority"`
}

// AuditReport is the whole health check.
type AuditReport struct {
	Score    int            `json:"score"`
	Items    int            `json:"items"`
	Affected int            `json:"affected"`
	Weak     int            `json:"weak"`
	Reused   int            `json:"reused"`
	Stale    int            `json:"stale"`
	NoTOTP   int            `json:"noTotp"`
	Cards    int            `json:"cards"`
	Findings []AuditFinding `json:"findings"`
	// Trashed counts items skipped because they are in the trash.
	Trashed   int       `json:"trashed"`
	CheckedAt time.Time `json:"checkedAt"`
}

// auditPenalty is how much each finding costs the score. A single item can trip
// several checks, but only its worst finding is charged, so one careless item
// cannot drag the whole vault to zero on its own.
var auditPenalty = map[AuditKind]int{
	KindWeak:             18,
	KindReused:           12,
	KindCardExpired:      8,
	KindStale:            6,
	KindCardExpiringSoon: 3,
	KindNoTOTP:           3,
}

// Audit runs the health check with the default thresholds.
func (v *Vault) Audit() (AuditReport, error) {
	return v.AuditWith(DefaultAuditOptions())
}

// AuditWith runs the health check with caller-supplied thresholds.
func (v *Vault) AuditWith(opts AuditOptions) (AuditReport, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if !v.unlocked {
		return AuditReport{}, ErrLocked
	}
	opts = opts.withDefaults()
	now := v.now().UTC()

	report := AuditReport{Score: 100, CheckedAt: now}

	// Reuse is a property of the whole vault, so group passwords first. Only
	// live items participate: a trashed duplicate cannot be fixed by the user.
	groups := map[string][]*Item{}
	for i := range v.payload.Items {
		item := &v.payload.Items[i]
		if item.Trashed() {
			report.Trashed++
			continue
		}
		report.Items++
		if item.Password == "" {
			continue
		}
		groups[item.Password] = append(groups[item.Password], item)
	}

	findings := make([]AuditFinding, 0)
	add := func(item *Item, kind AuditKind, severity AuditSeverity, format string, args ...any) {
		findings = append(findings, AuditFinding{
			Kind:     kind,
			Severity: severity,
			ItemID:   item.ID,
			Title:    item.Title,
			Detail:   fmt.Sprintf(format, args...),
			Priority: auditPenalty[kind],
		})
	}

	for i := range v.payload.Items {
		item := &v.payload.Items[i]
		if item.Trashed() {
			continue
		}

		if !opts.IgnoreWeak && item.Password != "" {
			if strength := Score(item.Password); strength.Score <= opts.WeakScore {
				severity := SeverityMedium
				if strength.Score <= 1 {
					severity = SeverityHigh
				}
				add(item, KindWeak, severity, "The password rates %q, about %.0f bits of entropy.", strength.Label, strength.EntropyBits)
			}
		}

		if len(groups[item.Password]) > 1 && item.Password != "" {
			report.Reused++
			others := len(groups[item.Password]) - 1
			add(item, KindReused, SeverityHigh,
				"The same password also protects %d other %s.", others, plural(others, "item"))
		}

		if item.Password != "" && opts.StaleDays > 0 {
			changed := passwordChangedAt(item)
			age := int(now.Sub(changed).Hours() / 24)
			if age >= opts.StaleDays {
				report.Stale++
				add(item, KindStale, SeverityMedium,
					"The password has not changed in %s (last change %s).",
					humanDays(age), changed.Format("2 Jan 2006"))
			}
		}

		if item.Type == TypeLogin && item.Password != "" && item.URL != "" && item.TOTP == "" {
			report.NoTOTP++
			add(item, KindNoTOTP, SeverityLow, "No second factor is stored for this login.")
		}

		if item.Type == TypeCard && item.Card != nil && item.Card.ExpYear > 0 {
			expiry := time.Date(item.Card.ExpYear, time.Month(maxInt(item.Card.ExpMonth, 1)), 1, 0, 0, 0, 0, time.UTC).
				AddDate(0, 1, -1)
			switch days := int(expiry.Sub(now).Hours() / 24); {
			case expiry.Before(now):
				report.Cards++
				add(item, KindCardExpired, SeverityMedium, "This card expired on %s.", expiry.Format("Jan 2006"))
			case days <= opts.CardWarningDays:
				report.Cards++
				add(item, KindCardExpiringSoon, SeverityLow, "This card expires on %s.", expiry.Format("Jan 2006"))
			}
		}

		if item.Password != "" {
			if strength := Score(item.Password); strength.Score <= opts.WeakScore && !opts.IgnoreWeak {
				report.Weak++
			}
		}
	}

	// Score: charge each item only for its most expensive finding.
	worst := map[string]int{}
	for _, finding := range findings {
		if finding.Priority > worst[finding.ItemID] {
			worst[finding.ItemID] = finding.Priority
		}
	}
	for _, penalty := range worst {
		report.Score -= penalty
	}
	if report.Score < 0 {
		report.Score = 0
	}
	report.Affected = len(worst)

	sort.SliceStable(findings, func(a, b int) bool {
		if findings[a].Severity != findings[b].Severity {
			return severityRank(findings[a].Severity) < severityRank(findings[b].Severity)
		}
		if findings[a].Priority != findings[b].Priority {
			return findings[a].Priority > findings[b].Priority
		}
		if findings[a].Title != findings[b].Title {
			return strings.ToLower(findings[a].Title) < strings.ToLower(findings[b].Title)
		}
		return findings[a].Kind < findings[b].Kind
	})
	report.Findings = findings
	return report, nil
}

// passwordChangedAt derives when the current password was set. A password that
// has been rotated has the timestamp of the rotation as the ChangedAt of the
// entry it pushed to history, so the newest history entry is exactly the moment
// the current password took over. Without history the password dates from the
// item's creation.
func passwordChangedAt(item *Item) time.Time {
	if len(item.History) > 0 && !item.History[0].ChangedAt.IsZero() {
		return item.History[0].ChangedAt
	}
	return item.CreatedAt
}

func (o AuditOptions) withDefaults() AuditOptions {
	def := DefaultAuditOptions()
	if o.StaleDays == 0 {
		o.StaleDays = def.StaleDays
	}
	if o.WeakScore == 0 {
		o.WeakScore = def.WeakScore
	}
	if o.CardWarningDays == 0 {
		o.CardWarningDays = def.CardWarningDays
	}
	return o
}

func severityRank(s AuditSeverity) int {
	switch s {
	case SeverityHigh:
		return 0
	case SeverityMedium:
		return 1
	default:
		return 2
	}
}

func humanDays(days int) string {
	switch {
	case days >= 730:
		return fmt.Sprintf("%d years", days/365)
	case days >= 365:
		return "over a year"
	case days >= 60:
		return fmt.Sprintf("%d months", days/30)
	default:
		return fmt.Sprintf("%d days", days)
	}
}

func plural(count int, noun string) string {
	if count == 1 {
		return noun
	}
	return noun + "s"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
