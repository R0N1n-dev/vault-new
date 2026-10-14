package vault

import (
	"encoding/base32"
	"strings"
	"testing"
	"time"
)

// rfc6238Secret holds the ASCII seeds from RFC 6238 appendix B.
var rfc6238Seeds = map[string]string{
	"SHA1":   "12345678901234567890",
	"SHA256": "12345678901234567890123456789012",
	"SHA512": "1234567890123456789012345678901234567890123456789012345678901234",
}

// TestRFC6238Vectors pins the implementation to the published test vectors.
func TestRFC6238Vectors(t *testing.T) {
	vectors := []struct {
		unix   int64
		sha1   string
		sha256 string
		sha512 string
	}{
		{59, "94287082", "46119246", "90693936"},
		{1111111109, "07081804", "68084774", "25091201"},
		{1111111111, "14050471", "67062674", "99943326"},
		{1234567890, "89005924", "91819424", "93441116"},
		{2000000000, "69279037", "90698825", "38618901"},
		{20000000000, "65353130", "77737706", "47863826"},
	}
	expected := map[string]map[int64]string{
		"SHA1": {}, "SHA256": {}, "SHA512": {},
	}
	for _, v := range vectors {
		expected["SHA1"][v.unix] = v.sha1
		expected["SHA256"][v.unix] = v.sha256
		expected["SHA512"][v.unix] = v.sha512
	}

	for algorithm, seed := range rfc6238Seeds {
		encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(seed))
		cfg := TOTPConfig{Secret: encoded, Digits: 8, Period: 30, Algorithm: algorithm}
		for _, vector := range vectors {
			at := time.Unix(vector.unix, 0).UTC()
			code, err := cfg.Code(at)
			if err != nil {
				t.Fatalf("%s @ %d: %v", algorithm, vector.unix, err)
			}
			if want := expected[algorithm][vector.unix]; code != want {
				t.Fatalf("%s @ %d: got %s want %s", algorithm, vector.unix, code, want)
			}
		}
	}
}

func TestBareSecretDefaults(t *testing.T) {
	cfg, err := ParseTOTPSecret("JBSWY3DPEHPK3PXP")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Digits != 6 || cfg.Period != 30 || cfg.Algorithm != "SHA1" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	code, err := cfg.Code(time.Unix(0, 0))
	if err != nil {
		t.Fatalf("code: %v", err)
	}
	if len(code) != 6 {
		t.Fatalf("want a 6 digit code, got %q", code)
	}
}

func TestSecretCleanupAndPadding(t *testing.T) {
	// Users paste secrets with spaces, dashes and lowercase, and often without
	// the base32 padding.
	for _, raw := range []string{
		"jbswy3dpehpk3pxp",
		"JBSW Y3DP EHPK 3PXP",
		"JBSW-Y3DP-EHPK-3PXP",
		"  JBSWY3DPEHPK3PXP  ",
	} {
		cfg, err := ParseTOTPSecret(raw)
		if err != nil {
			t.Fatalf("parse %q: %v", raw, err)
		}
		if cfg.Secret != "JBSWY3DPEHPK3PXP" {
			t.Fatalf("parse %q produced %q", raw, cfg.Secret)
		}
	}
}

func TestParseOtpauthURI(t *testing.T) {
	uri := "otpauth://totp/GitHub:ada%40example.com?secret=JBSWY3DPEHPK3PXP&issuer=GitHub&algorithm=SHA256&digits=8&period=60"
	cfg, err := ParseTOTPSecret(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Issuer != "GitHub" || cfg.Account != "ada@example.com" {
		t.Fatalf("unexpected label parsing: %+v", cfg)
	}
	if cfg.Digits != 8 || cfg.Period != 60 || cfg.Algorithm != "SHA256" {
		t.Fatalf("unexpected parameters: %+v", cfg)
	}
}

func TestParseTOTPErrors(t *testing.T) {
	cases := map[string]string{
		"empty":            "",
		"not base32":       "not base32!!",
		"too short":        "JBSWY3DP",
		"hotp unsupported": "otpauth://hotp/x?secret=JBSWY3DPEHPK3PXP",
		"bad digits":       "otpauth://totp/x?secret=JBSWY3DPEHPK3PXP&digits=3",
		"bad algorithm":    "otpauth://totp/x?secret=JBSWY3DPEHPK3PXP&algorithm=MD5",
		"no secret":        "otpauth://totp/x?issuer=GitHub",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseTOTPSecret(raw); err == nil {
				t.Fatalf("expected an error for %q", raw)
			}
		})
	}
}

func TestRemainingCountsDown(t *testing.T) {
	cfg := TOTPConfig{Secret: "JBSWY3DPEHPK3PXP", Digits: 6, Period: 30, Algorithm: "SHA1"}
	if got := cfg.Remaining(time.Unix(0, 0)); got != 30 {
		t.Fatalf("want 30, got %d", got)
	}
	if got := cfg.Remaining(time.Unix(29, 0)); got != 1 {
		t.Fatalf("want 1, got %d", got)
	}
	if got := cfg.Remaining(time.Unix(30, 0)); got != 30 {
		t.Fatalf("want 30 at a period boundary, got %d", got)
	}
}

func TestCodeIsStableWithinAPeriod(t *testing.T) {
	cfg, err := ParseTOTPSecret("JBSWY3DPEHPK3PXP")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Start on a period boundary so +29s stays inside the same window.
	base := time.Unix(1_700_000_000, 0).UTC().Truncate(30 * time.Second)
	first, err := cfg.Code(base)
	if err != nil {
		t.Fatalf("code: %v", err)
	}
	second, err := cfg.Code(base.Add(29 * time.Second))
	if err != nil {
		t.Fatalf("code: %v", err)
	}
	if first != second {
		t.Fatalf("code changed inside the period: %s vs %s", first, second)
	}
	third, err := cfg.Code(base.Add(30 * time.Second))
	if err != nil {
		t.Fatalf("code: %v", err)
	}
	if first == third {
		t.Fatal("expected a different code in the next period")
	}
}

func TestCodeForSecret(t *testing.T) {
	got, err := CodeForSecret("JBSWY3DPEHPK3PXP", time.Unix(1_700_000_000, 0).UTC())
	if err != nil {
		t.Fatalf("code: %v", err)
	}
	if len(got.Code) != 6 || got.Digits != 6 || got.Period != 30 {
		t.Fatalf("unexpected result: %+v", got)
	}
	if got.Remaining < 1 || got.Remaining > 30 {
		t.Fatalf("unexpected remaining: %+v", got)
	}
	if _, err := CodeForSecret("nope", time.Now()); err == nil {
		t.Fatal("expected an error for an invalid secret")
	}
}

func TestSecretByteLengthIsEnforcedOnTheDecodedSecret(t *testing.T) {
	// Ten base32 characters are only fifty bits of entropy, so a secret that is
	// long enough as text but too short as bytes must still be rejected.
	if _, err := ParseTOTPSecret("NOTASECRET"); err == nil {
		t.Fatal("expected a ten character secret to be rejected")
	}
	// Sixteen base32 characters decode to the ten byte minimum and are accepted.
	if _, err := ParseTOTPSecret("JBSWY3DPEHPK3PXP"); err != nil {
		t.Fatalf("expected a ten byte secret to be accepted: %v", err)
	}
}

func TestPaddedSecretIsAccepted(t *testing.T) {
	// A secret whose base32 length is not a multiple of 8 needs padding.
	const padded = "MFRGGZDFMZTWQ2LK"
	if len(padded)%8 != 0 {
		t.Fatalf("test fixture is already padded: %d", len(padded))
	}
	cfg, err := ParseTOTPSecret(padded + "===")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if strings.Contains(cfg.Secret, "=") {
		t.Fatalf("normalised secret should carry no padding: %q", cfg.Secret)
	}
	if _, err := cfg.Code(time.Now()); err != nil {
		t.Fatalf("code: %v", err)
	}
}
