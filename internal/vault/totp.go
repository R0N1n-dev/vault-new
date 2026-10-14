package vault

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"hash"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// TOTP defaults from RFC 6238, which is what every authenticator app uses.
const (
	DefaultTOTPDigits = 6
	DefaultTOTPPeriod = 30
)

// TOTPConfig is a parsed one-time password secret. Coffer stores the secret in
// the vault and generates codes locally: the seed never leaves the machine.
type TOTPConfig struct {
	Secret    string `json:"secret"`
	Digits    int    `json:"digits"`
	Period    int    `json:"period"`
	Algorithm string `json:"algorithm"`
	Issuer    string `json:"issuer,omitempty"`
	Account   string `json:"account,omitempty"`
}

// TOTPResult is what the UI needs to render a live code with a countdown.
type TOTPResult struct {
	Code      string `json:"code"`
	Digits    int    `json:"digits"`
	Period    int    `json:"period"`
	Remaining int    `json:"remaining"`
}

// ParseTOTPSecret accepts either a bare base32 secret (as shown by any site's
// "set up 2FA" page) or a full otpauth:// URI, which is what QR codes encode.
func ParseTOTPSecret(raw string) (TOTPConfig, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return TOTPConfig{}, fmt.Errorf("missing one-time password secret")
	}
	cfg := TOTPConfig{
		Digits:    DefaultTOTPDigits,
		Period:    DefaultTOTPPeriod,
		Algorithm: "SHA1",
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "otpauth://") {
		u, err := url.Parse(trimmed)
		if err != nil {
			return TOTPConfig{}, fmt.Errorf("invalid otpauth URI: %w", err)
		}
		if !strings.EqualFold(u.Host, "totp") {
			return TOTPConfig{}, fmt.Errorf("only totp:// secrets are supported (got %q)", u.Host)
		}
		label := strings.TrimPrefix(u.Path, "/")
		if decoded, err := url.PathUnescape(label); err == nil {
			label = decoded
		}
		if issuer, account, ok := strings.Cut(label, ":"); ok {
			cfg.Issuer = strings.TrimSpace(issuer)
			cfg.Account = strings.TrimSpace(account)
		} else {
			cfg.Account = label
		}
		q := u.Query()
		cfg.Secret = q.Get("secret")
		if issuer := q.Get("issuer"); issuer != "" {
			cfg.Issuer = issuer
		}
		if v := q.Get("digits"); v != "" {
			digits, err := strconv.Atoi(v)
			if err != nil || digits < 6 || digits > 10 {
				return TOTPConfig{}, fmt.Errorf("unsupported digit count %q", v)
			}
			cfg.Digits = digits
		}
		if v := q.Get("period"); v != "" {
			period, err := strconv.Atoi(v)
			if err != nil || period < 10 || period > 300 {
				return TOTPConfig{}, fmt.Errorf("unsupported period %q", v)
			}
			cfg.Period = period
		}
		if v := q.Get("algorithm"); v != "" {
			cfg.Algorithm = strings.ToUpper(v)
		}
	} else {
		cfg.Secret = trimmed
	}

	switch cfg.Algorithm {
	case "SHA1", "SHA256", "SHA512":
	default:
		return TOTPConfig{}, fmt.Errorf("unsupported algorithm %q", cfg.Algorithm)
	}
	normalized, err := normalizeBase32(cfg.Secret)
	if err != nil {
		return TOTPConfig{}, err
	}
	cfg.Secret = normalized
	return cfg, nil
}

// MinTOTPSecretBytes is the smallest accepted secret. RFC 4226 requires at
// least 128 bits when provisioning, but 80 bits (10 bytes) is what authenticator
// apps and every real site issue, so it is the floor here. The check is on the
// decoded length: ten base32 characters are only 50 bits.
const MinTOTPSecretBytes = 10

// normalizeBase32 strips grouping characters and restores the padding that
// users routinely lose when copying a secret by hand.
func normalizeBase32(secret string) (string, error) {
	cleaned := strings.ToUpper(strings.NewReplacer(" ", "", "-", "", "\t", "").Replace(secret))
	cleaned = strings.TrimRight(cleaned, "=")
	if cleaned == "" {
		return "", fmt.Errorf("missing one-time password secret")
	}
	if missing := len(cleaned) % 8; missing != 0 {
		cleaned += strings.Repeat("=", 8-missing)
	}
	decoded, err := base32.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return "", fmt.Errorf("one-time password secret is not valid base32")
	}
	if len(decoded) < MinTOTPSecretBytes {
		return "", fmt.Errorf("one-time password secret is too short")
	}
	return strings.TrimRight(cleaned, "="), nil
}

func (c TOTPConfig) hash() (func() hash.Hash, error) {
	switch c.Algorithm {
	case "SHA1", "":
		return sha1.New, nil
	case "SHA256":
		return sha256.New, nil
	case "SHA512":
		return sha512.New, nil
	}
	return nil, fmt.Errorf("unsupported algorithm %q", c.Algorithm)
}

// Code returns the code valid at the given instant, implementing the HOTP
// truncation of RFC 4226 with the time-based counter of RFC 6238.
func (c TOTPConfig) Code(at time.Time) (string, error) {
	newHash, err := c.hash()
	if err != nil {
		return "", err
	}
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(c.Secret)
	if err != nil {
		if secret, err = base32.StdEncoding.DecodeString(c.Secret); err != nil {
			return "", fmt.Errorf("one-time password secret is not valid base32")
		}
	}
	period := c.Period
	if period <= 0 {
		period = DefaultTOTPPeriod
	}
	digits := c.Digits
	if digits <= 0 {
		digits = DefaultTOTPDigits
	}

	counter := uint64(at.UTC().Unix() / int64(period))
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)

	mac := hmac.New(newHash, secret)
	mac.Write(buf[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	truncated := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff

	mod := uint32(1)
	for range digits {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", digits, truncated%mod), nil
}

// Remaining returns how many seconds the current code stays valid.
func (c TOTPConfig) Remaining(at time.Time) int {
	period := c.Period
	if period <= 0 {
		period = DefaultTOTPPeriod
	}
	elapsed := at.UTC().Unix() % int64(period)
	return int(int64(period) - elapsed)
}

// CodeForSecret is a convenience wrapper used by the app layer.
func CodeForSecret(secret string, at time.Time) (TOTPResult, error) {
	cfg, err := ParseTOTPSecret(secret)
	if err != nil {
		return TOTPResult{}, err
	}
	code, err := cfg.Code(at)
	if err != nil {
		return TOTPResult{}, err
	}
	return TOTPResult{Code: code, Digits: cfg.Digits, Period: cfg.Period, Remaining: cfg.Remaining(at)}, nil
}
