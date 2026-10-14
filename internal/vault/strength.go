package vault

import (
	"math"
	"regexp"
	"strings"
)

// Strength is a heuristic, offline password-strength estimate. It is not a
// zxcvbn replacement, but it is deterministic, dependency-free and good enough
// to steer users away from the passwords that actually get cracked.
type Strength struct {
	Score       int      `json:"score"` // 0 (very weak) .. 4 (excellent)
	Label       string   `json:"label"`
	EntropyBits float64  `json:"entropyBits"`
	Warning     string   `json:"warning,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
}

var (
	sequentialRun = regexp.MustCompile(`(?i)(0123|1234|2345|3456|4567|5678|6789|abcd|bcde|cdef|qwer|wert|erty|rtyu|tyui|yuio|uiop|asdf|sdfg|dfgh|fghj|ghjk|hjkl|zxcv|xcvb|cvbn|vbnm|pass|letme|admin|welcome|ilove|qaz|wsx|edc)`)
	yearPattern   = regexp.MustCompile(`(19|20)\d{2}`)
)

// commonPasswords is a short list of the credentials that dominate every
// public breach corpus, plus their leetspeak spellings.
var commonPasswords = strings.Fields(`
123456 password 123456789 12345678 12345 1234567 qwerty abc123 111111 123123
1234567890 1234 iloveyou 000000 admin monkey letmein dragon sunshine princess
football baseball welcome shadow master michael superman batman trustno1 hello
freedom whatever qazwsx qwerty123 zaq12wsx 1q2w3e4r 1qaz2wsx passw0rd p@ssw0rd
p@ssword password1 password123 qwertyuiop asdfghjkl zxcvbnm 987654321 654321
666666 888888 121212 112233 159753 1qaz2wsx3edc q1w2e3r4 loveme lovely
computer internet samsung google facebook amazon apple mustang jordan harley
ranger hunter buster soccer hockey killer george sebastian computer1
letmein1 welcome1 admin123 root toor test test123 guest changeme
`)

// Score estimates the strength of a password.
func Score(password string) Strength {
	if password == "" {
		return Strength{Score: 0, Label: "Empty", Warning: "No password set."}
	}

	runes := []rune(password)
	length := float64(len(runes))
	pool := float64(0)
	var hasLower, hasUpper, hasDigit, hasSymbol, hasOther bool
	for _, r := range runes {
		switch {
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= '0' && r <= '9':
			hasDigit = true
		case r < 128:
			hasSymbol = true
		default:
			hasOther = true
		}
	}
	switch {
	case hasLower:
		pool += 26
	case hasUpper:
		pool += 26
	default:
		pool += 10
	}
	if hasLower && hasUpper {
		pool += 26
	}
	if hasDigit {
		pool += 10
	}
	if hasSymbol {
		pool += 33
	}
	if hasOther {
		pool += 100
	}

	entropy := length * math.Log2(pool)
	var suggestions []string
	recommend := func(s string) {
		for _, existing := range suggestions {
			if existing == s {
				return
			}
		}
		suggestions = append(suggestions, s)
	}

	lower := strings.ToLower(password)
	// Repeated characters ("aaaa", "1111") add far less than their length.
	repeatPenalty := 0.0
	if longest := longestRun(runes); longest >= 3 {
		repeatPenalty += float64(longest-2) * 2
		recommend("Avoid repeating the same character.")
	}
	// Strictly increasing/decreasing sequences.
	if seq := sequentialPairs(runes); seq > 2 {
		repeatPenalty += float64(seq) * 1.5
		recommend("Avoid character sequences like abcd, 1234 or qwerty.")
	}
	if matched := sequentialRun.FindAllString(lower, -1); len(matched) > 0 {
		recommend("Avoid keyboard rows and common words inside passwords.")
	}
	// A short unit repeated to fill the length: "abcabcabc".
	if unit, reps := smallestRepeat(lower); reps >= 3 && len(unit) < len(lower) {
		entropy = float64(len(unit))*math.Log2(pool) + math.Log2(float64(reps))
		recommend("Avoid repeating a short pattern.")
	}
	if yearPattern.MatchString(lower) {
		repeatPenalty += 6
		recommend("Avoid years, dates and other predictable numbers.")
	}
	if isCommonPassword(lower) {
		return Strength{
			Score:       0,
			Label:       "Very weak",
			EntropyBits: 0,
			Warning:     "This is one of the most commonly used passwords.",
			Suggestions: []string{"Pick something unique, or generate a random password."},
		}
	}
	entropy -= repeatPenalty
	if entropy < 0 {
		entropy = 0
	}

	if len(runes) < 12 {
		recommend("Use at least 12 characters.")
	}
	if !hasSymbol && !hasOther {
		recommend("Adding symbols increases the search space.")
	}

	score, label := 0, "Very weak"
	switch {
	case entropy >= 100:
		score, label = 4, "Excellent"
	case entropy >= 70:
		score, label = 3, "Strong"
	case entropy >= 50:
		score, label = 2, "Fair"
	case entropy >= 30:
		score, label = 1, "Weak"
	}
	if score >= 3 {
		suggestions = nil
	}
	return Strength{
		Score:       score,
		Label:       label,
		EntropyBits: math.Round(entropy*10) / 10,
		Suggestions: suggestions,
	}
}

// isCommonPassword also folds common leetspeak substitutions before checking,
// so "p@ssw0rd" is caught as "password".
func isCommonPassword(lower string) bool {
	if contains(commonPasswords, lower) {
		return true
	}
	replacer := strings.NewReplacer("@", "a", "4", "a", "3", "e", "1", "i", "0", "o", "5", "s", "$", "s", "7", "t", "!", "i")
	folded := replacer.Replace(lower)
	return contains(commonPasswords, folded)
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func longestRun(runes []rune) int {
	best, run := 1, 1
	for i := 1; i < len(runes); i++ {
		if runes[i] == runes[i-1] {
			run++
			if run > best {
				best = run
			}
		} else {
			run = 1
		}
	}
	return best
}

func sequentialPairs(runes []rune) int {
	count := 0
	for i := 1; i < len(runes); i++ {
		d := int(runes[i]) - int(runes[i-1])
		if d == 1 || d == -1 {
			count++
		}
	}
	return count
}

// smallestRepeat returns the shortest repeating unit of s and how often it
// repeats (reps == 1 when s does not repeat).
func smallestRepeat(s string) (string, int) {
	for size := 1; size <= len(s)/2; size++ {
		if len(s)%size != 0 {
			continue
		}
		unit := s[:size]
		if strings.Repeat(unit, len(s)/size) == s {
			return unit, len(s) / size
		}
	}
	return s, 1
}
