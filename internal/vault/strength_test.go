package vault

import (
	"strings"
	"testing"
)

func TestScoreIsMonotonicForIncreasingEntropy(t *testing.T) {
	order := []string{
		"",
		"password",
		"123456",
		"aaaaaaaaaaaa",
		"abcdefghijkl",
		"Tr0ub4dor",
		"a-very-long-but-all-lowercase-passphrase",
		"vE7#kQ2$mZ9!pL4&xR6@wT1",
		"J7$pQ2!vX9#mK4&zR6@wL1^nB8%cH3",
	}
	lastScore := -1
	for _, password := range order {
		got := Score(password)
		if got.Score < lastScore {
			t.Fatalf("score went backwards for %q: %+v", password, got)
		}
		if got.Score < 0 || got.Score > 4 {
			t.Fatalf("score out of range for %q: %+v", password, got)
		}
		lastScore = got.Score
	}
}

func TestCommonPasswordsAreRejected(t *testing.T) {
	for _, password := range []string{"password", "PASSWORD", "Password1", "p@ssw0rd", "123456", "letmein", "qwertyuiop", "trustno1", "1qaz2wsx3edc"} {
		got := Score(password)
		if got.Score != 0 {
			t.Fatalf("%q should score 0, got %+v", password, got)
		}
		if got.Warning == "" {
			t.Fatalf("%q should carry a warning", password)
		}
	}
}

func TestWeakPatternsArePenalised(t *testing.T) {
	plain := Score("abcdefghijkl")
	long := Score("A!bcdefghijklmnop")
	if long.EntropyBits <= plain.EntropyBits {
		t.Fatalf("longer mixed password should score higher: %v vs %v", long.EntropyBits, plain.EntropyBits)
	}
	repeat := Score("aaaaaaaaaaaaaaaa")
	if repeat.Score > 1 {
		t.Fatalf("a repeated character is not strong: %+v", repeat)
	}
	sequence := Score("123456789012")
	if sequence.Score > 1 {
		t.Fatalf("a numeric sequence is not strong: %+v", sequence)
	}
	year := Score("Summer2024!")
	if year.Warning != "" && !strings.Contains(strings.ToLower(strings.Join(year.Suggestions, " ")), "year") {
		t.Fatalf("expected a year suggestion: %+v", year)
	}
	pattern := Score("abcabcabcabcabc")
	if pattern.Score > 2 {
		t.Fatalf("a repeated pattern is not strong: %+v", pattern)
	}
}

func TestStrongPasswordsHaveNoSuggestions(t *testing.T) {
	got := Score("qZ4#vN8*mX2!kL6&wR9")
	if got.Score != 4 {
		t.Fatalf("want the top score, got %+v", got)
	}
	if len(got.Suggestions) != 0 {
		t.Fatalf("no suggestions expected for a strong password: %+v", got.Suggestions)
	}
	if got.Label != "Excellent" {
		t.Fatalf("unexpected label: %q", got.Label)
	}
}

func TestSuggestionsForShortSimplePasswords(t *testing.T) {
	got := Score("butterfly")
	if got.Score > 1 {
		t.Fatalf("unexpected score: %+v", got)
	}
	joined := strings.ToLower(strings.Join(got.Suggestions, " "))
	if !strings.Contains(joined, "12 characters") {
		t.Fatalf("expected a length suggestion: %+v", got.Suggestions)
	}
}
