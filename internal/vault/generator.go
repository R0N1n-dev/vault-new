package vault

import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"strings"
)

// Generator modes.
const (
	ModePassword   = "password"
	ModePassphrase = "passphrase"
)

// symbolSet avoids quotes, backslashes, spaces and backticks: they break shells,
// CSVs and many login forms for no entropy benefit.
const symbolSet = "!@#$%^&*()-_=+[]{};:,.?/"

// ambiguousChars are removed when ExcludeAmbiguous is set because they are easy
// to confuse when reading a password off screen.
const ambiguousChars = "Il1O0o|"

// GeneratorOptions configures password and passphrase generation.
type GeneratorOptions struct {
	Mode string `json:"mode"`
	// Password mode.
	Length               int  `json:"length"`
	Uppercase            bool `json:"uppercase"`
	Lowercase            bool `json:"lowercase"`
	Digits               bool `json:"digits"`
	Symbols              bool `json:"symbols"`
	ExcludeAmbiguous     bool `json:"excludeAmbiguous"`
	RequireEverySelected bool `json:"requireEverySelected"`
	// Passphrase mode.
	Words        int    `json:"words"`
	Separator    string `json:"separator"`
	Capitalize   bool   `json:"capitalize"`
	AppendNumber bool   `json:"appendNumber"`
}

// DefaultGeneratorOptions returns the defaults used for new vaults: a 20
// character mixed-class password, or a six-word passphrase (~65 bits).
func DefaultGeneratorOptions() GeneratorOptions {
	return GeneratorOptions{
		Mode:                 ModePassword,
		Length:               20,
		Uppercase:            true,
		Lowercase:            true,
		Digits:               true,
		Symbols:              true,
		ExcludeAmbiguous:     true,
		RequireEverySelected: true,
		Words:                6,
		Separator:            "-",
		Capitalize:           true,
		AppendNumber:         true,
	}
}

// normalize fills in defaults and clamps out-of-range values so the UI can send
// a partially specified options object.
func (o *GeneratorOptions) normalize() error {
	if o.Mode == "" {
		o.Mode = ModePassword
	}
	switch o.Mode {
	case ModePassword:
		if o.Length == 0 {
			o.Length = 20
		}
		if o.Length < 8 || o.Length > 128 {
			return fmt.Errorf("%w: password length must be between 8 and 128", ErrInvalid)
		}
		if o.poolSize() < 2 {
			return fmt.Errorf("%w: select at least two character sets", ErrInvalid)
		}
	case ModePassphrase:
		if o.Words == 0 {
			o.Words = 6
		}
		if o.Words < 3 || o.Words > 12 {
			return fmt.Errorf("%w: passphrases use between 3 and 12 words", ErrInvalid)
		}
		if o.Separator == "" {
			o.Separator = "-"
		}
		if len([]rune(o.Separator)) > 3 {
			return fmt.Errorf("%w: the separator can be at most three characters", ErrInvalid)
		}
	default:
		return fmt.Errorf("%w: unknown generator mode %q", ErrInvalid, o.Mode)
	}
	return nil
}

func (o *GeneratorOptions) excludes(r rune) bool {
	return o.ExcludeAmbiguous && strings.ContainsRune(ambiguousChars, r)
}

func (o *GeneratorOptions) filter(s string) []rune {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if !o.excludes(r) {
			out = append(out, r)
		}
	}
	return out
}

func (o *GeneratorOptions) classes() [][]rune {
	var classes [][]rune
	if o.Lowercase {
		classes = append(classes, o.filter("abcdefghijklmnopqrstuvwxyz"))
	}
	if o.Uppercase {
		classes = append(classes, o.filter("ABCDEFGHIJKLMNOPQRSTUVWXYZ"))
	}
	if o.Digits {
		classes = append(classes, o.filter("0123456789"))
	}
	if o.Symbols {
		classes = append(classes, o.filter(symbolSet))
	}
	return classes
}

func (o *GeneratorOptions) poolSize() int {
	total := 0
	for _, c := range o.classes() {
		total += len(c)
	}
	return total
}

// Generated is the result of a generation request.
type Generated struct {
	Value       string   `json:"value"`
	Strength    Strength `json:"strength"`
	EntropyBits float64  `json:"entropyBits"`
}

// GeneratePassword returns a random password or passphrase. Randomness comes
// from crypto/rand with rejection sampling (via math/big), so every character
// is uniformly distributed and nonce-style modulo bias is impossible.
func GeneratePassword(opts GeneratorOptions) (Generated, error) {
	if err := opts.normalize(); err != nil {
		return Generated{}, err
	}
	if opts.Mode == ModePassphrase {
		return generatePassphrase(opts)
	}
	return generateRandom(opts)
}

func generateRandom(opts GeneratorOptions) (Generated, error) {
	classes := opts.classes()
	pool := opts.poolSize()

	chars := make([]rune, 0, opts.Length)
	if opts.RequireEverySelected {
		for _, class := range classes {
			r, err := pickRune(class)
			if err != nil {
				return Generated{}, err
			}
			chars = append(chars, r)
		}
	}
	all := make([]rune, 0, pool)
	for _, class := range classes {
		all = append(all, class...)
	}
	for len(chars) < opts.Length {
		r, err := pickRune(all)
		if err != nil {
			return Generated{}, err
		}
		chars = append(chars, r)
	}
	if err := shuffle(chars); err != nil {
		return Generated{}, err
	}
	password := string(chars[:opts.Length])
	entropy := float64(opts.Length) * math.Log2(float64(pool))
	return Generated{
		Value:       password,
		EntropyBits: math.Round(entropy*10) / 10,
		Strength:    Score(password),
	}, nil
}

func generatePassphrase(opts GeneratorOptions) (Generated, error) {
	// Draw from a list that cannot contain the separator: a word such as
	// "yo-yo" joined with "-" produces a passphrase that cannot be split back
	// into its parts, which makes it ambiguous to read aloud and to parse.
	choices := passphraseWords(opts.Separator)
	words := make([]string, 0, opts.Words+1)
	for range opts.Words {
		idx, err := randIndex(len(choices))
		if err != nil {
			return Generated{}, err
		}
		word := choices[idx]
		if opts.Capitalize {
			word = strings.ToUpper(word[:1]) + word[1:]
		}
		words = append(words, word)
	}
	bits := float64(opts.Words) * math.Log2(float64(len(choices)))
	if opts.AppendNumber {
		digit, err := randIndex(10)
		if err != nil {
			return Generated{}, err
		}
		bits += math.Log2(10)
		words = append(words, fmt.Sprintf("%d", digit))
	}
	value := strings.Join(words, opts.Separator)
	return Generated{
		Value:       value,
		EntropyBits: math.Round(bits*10) / 10,
		Strength:    Score(value),
	}, nil
}

// passphraseWords returns the word list without any entry that contains the
// separator. The fallback keeps a pathological separator (one that matches
// every word) from producing an empty list.
func passphraseWords(separator string) []string {
	if separator == "" {
		return wordList
	}
	words := make([]string, 0, len(wordList))
	for _, word := range wordList {
		if strings.Contains(word, separator) {
			continue
		}
		words = append(words, word)
	}
	if len(words) == 0 {
		return wordList
	}
	return words
}

func pickRune(set []rune) (rune, error) {
	idx, err := randIndex(len(set))
	if err != nil {
		return 0, err
	}
	return set[idx], nil
}

// randIndex returns a uniform random integer in [0, n).
func randIndex(n int) (int, error) {
	if n <= 0 {
		return 0, fmt.Errorf("%w: empty character set", ErrInvalid)
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, fmt.Errorf("read random bytes: %w", err)
	}
	return int(v.Int64()), nil
}

func shuffle(runes []rune) error {
	for i := len(runes) - 1; i > 0; i-- {
		j, err := randIndex(i + 1)
		if err != nil {
			return err
		}
		runes[i], runes[j] = runes[j], runes[i]
	}
	return nil
}
