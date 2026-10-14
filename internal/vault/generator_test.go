package vault

import (
	"errors"
	"math"
	"strings"
	"testing"
	"unicode"
)

func options(all bool) GeneratorOptions {
	o := DefaultGeneratorOptions()
	o.Uppercase, o.Lowercase, o.Digits, o.Symbols = all, all, all, all
	return o
}

func TestGeneratedPasswordHonoursLengthAndClasses(t *testing.T) {
	for _, length := range []int{8, 12, 20, 64, 128} {
		opts := options(true)
		opts.Length = length
		got, err := GeneratePassword(opts)
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if len([]rune(got.Value)) != length {
			t.Fatalf("want length %d, got %d (%q)", length, len([]rune(got.Value)), got.Value)
		}
		var hasUpper, hasLower, hasDigit, hasSymbol bool
		for _, r := range got.Value {
			switch {
			case unicode.IsUpper(r):
				hasUpper = true
			case unicode.IsLower(r):
				hasLower = true
			case unicode.IsDigit(r):
				hasDigit = true
			default:
				hasSymbol = true
			}
		}
		if !hasUpper || !hasLower || !hasDigit || !hasSymbol {
			t.Fatalf("RequireEverySelected was not honoured: %q", got.Value)
		}
		if got.Strength.Score < 2 {
			t.Fatalf("a 20+ character random password should score well: %+v", got.Strength)
		}
		if want := float64(length) * 6.0; got.EntropyBits < want {
			t.Fatalf("entropy for %d characters looks too low: %v (want >= %v)", length, got.EntropyBits, want)
		}
	}
}

func TestGeneratedPasswordRespectsClassSelection(t *testing.T) {
	opts := DefaultGeneratorOptions()
	opts.Uppercase, opts.Digits, opts.Symbols = false, false, false
	opts.Lowercase, opts.Length = true, 40
	got, err := GeneratePassword(opts)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, r := range got.Value {
		if r < 'a' || r > 'z' {
			t.Fatalf("lowercase-only request produced %q", got.Value)
		}
	}
}

func TestExcludeAmbiguous(t *testing.T) {
	opts := options(true)
	opts.Length = 128
	opts.ExcludeAmbiguous = true
	got, err := GeneratePassword(opts)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if strings.ContainsAny(got.Value, ambiguousChars) {
		t.Fatalf("ambiguous characters present: %q", got.Value)
	}
}

func TestGeneratorRejectsImpossibleOptions(t *testing.T) {
	cases := map[string]GeneratorOptions{
		"no classes":       {Mode: ModePassword, Length: 20},
		"length too short": {Mode: ModePassword, Length: 4, Lowercase: true, Digits: true},
		"length too long":  {Mode: ModePassword, Length: 500, Lowercase: true, Digits: true},
		"unknown mode":     {Mode: "quantum", Length: 20, Lowercase: true, Digits: true},
		"too few words":    {Mode: ModePassphrase, Words: 2},
		"too many words":   {Mode: ModePassphrase, Words: 40},
		"long separator":   {Mode: ModePassphrase, Words: 4, Separator: "----"},
	}
	for name, opts := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := GeneratePassword(opts); !errors.Is(err, ErrInvalid) {
				t.Fatalf("want ErrInvalid, got %v", err)
			}
		})
	}
}

func TestGenerateDoesNotRepeat(t *testing.T) {
	seen := map[string]bool{}
	opts := DefaultGeneratorOptions()
	for range 200 {
		got, err := GeneratePassword(opts)
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if seen[got.Value] {
			t.Fatalf("duplicate password generated: %q", got.Value)
		}
		seen[got.Value] = true
	}
}

func TestCharacterDistributionIsReasonablyUniform(t *testing.T) {
	opts := DefaultGeneratorOptions()
	opts.Length = 128
	opts.RequireEverySelected = false
	counts := map[rune]int{}
	const rounds = 100
	for range rounds {
		got, err := GeneratePassword(opts)
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		for _, r := range got.Value {
			counts[r]++
		}
	}
	total := rounds * opts.Length
	// Every character in the pool should appear; none should dominate.
	distinct := len(counts)
	expectedDistinct := 26 + 26 + 10 + len([]rune(symbolSet))
	if distinct < expectedDistinct-12 {
		t.Fatalf("too many characters never appeared: %d of %d", distinct, expectedDistinct)
	}
	for r, count := range counts {
		share := float64(count) / float64(total)
		if share > 0.5 {
			t.Fatalf("character %q dominates the output: %.2f", r, share)
		}
	}
}

func TestPassphraseShape(t *testing.T) {
	opts := DefaultGeneratorOptions()
	opts.Mode = ModePassphrase
	opts.Words = 6
	opts.Separator = "-"
	opts.Capitalize = true
	opts.AppendNumber = true
	got, err := GeneratePassword(opts)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	parts := strings.Split(got.Value, "-")
	if len(parts) != 7 {
		t.Fatalf("want 6 words plus a number, got %q", got.Value)
	}
	if len(parts[6]) != 1 || parts[6] < "0" || parts[6] > "9" {
		t.Fatalf("expected a trailing digit: %q", parts[6])
	}
	for _, word := range parts[:6] {
		if word == "" || word[:1] != strings.ToUpper(word[:1]) {
			t.Fatalf("words should be capitalised: %q", word)
		}
		if !contains(wordList, strings.ToLower(word)) {
			t.Fatalf("word %q is not in the word list", word)
		}
	}
	// 6 words x log2(1296) + log2(10) = 65.3 bits
	if got.EntropyBits < 60 || got.EntropyBits > 70 {
		t.Fatalf("unexpected passphrase entropy: %v", got.EntropyBits)
	}
	if got.Strength.Score < 3 {
		t.Fatalf("a six word passphrase should score well: %+v", got.Strength)
	}
}

// Regression: a word that contains the separator used to be drawn into the
// passphrase, which made the result impossible to split into its parts.
func TestPassphraseNeverContainsTheSeparator(t *testing.T) {
	for _, separator := range []string{"-", ".", "_", "~"} {
		opts := DefaultGeneratorOptions()
		opts.Mode = ModePassphrase
		opts.Words = 6
		opts.Separator = separator
		opts.Capitalize = true
		opts.AppendNumber = true
		for attempt := 0; attempt < 250; attempt++ {
			got, err := GeneratePassword(opts)
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			parts := strings.Split(got.Value, separator)
			if len(parts) != opts.Words+1 {
				t.Fatalf("separator %q split into %d parts instead of %d: %q",
					separator, len(parts), opts.Words+1, got.Value)
			}
		}
	}
}

func TestPassphraseEntropyUsesTheUsableWordList(t *testing.T) {
	opts := DefaultGeneratorOptions()
	opts.Mode = ModePassphrase
	opts.Separator = "-"
	got, err := GeneratePassword(opts)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	usable := len(passphraseWords("-"))
	if usable == len(wordList) {
		t.Skip("the word list happens to contain no separator characters")
	}
	bits := float64(opts.Words)*math.Log2(float64(usable)) + math.Log2(10)
	if math.Abs(got.EntropyBits-(math.Round(bits*10)/10)) > 0.05 {
		t.Fatalf("want %v bits from %d usable words, got %v", bits, usable, got.EntropyBits)
	}
}

func TestPassphraseWithoutNumberOrCapitalisation(t *testing.T) {
	opts := DefaultGeneratorOptions()
	opts.Mode = ModePassphrase
	opts.Words = 4
	opts.Separator = "."
	opts.Capitalize = false
	opts.AppendNumber = false
	got, err := GeneratePassword(opts)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	parts := strings.Split(got.Value, ".")
	if len(parts) != 4 {
		t.Fatalf("want 4 words: %q", got.Value)
	}
	for _, word := range parts {
		if word != strings.ToLower(word) {
			t.Fatalf("words should be lowercase: %q", word)
		}
	}
}

func TestWordListIntegrity(t *testing.T) {
	if len(wordList) != 1296 {
		t.Fatalf("want the full EFF short list, got %d words", len(wordList))
	}
	seen := map[string]bool{}
	for _, word := range wordList {
		if seen[word] {
			t.Fatalf("duplicate word in the list: %q", word)
		}
		seen[word] = true
	}
}
