package vaultcrypto

import (
	"bytes"
	"encoding/base32"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// The RFC 6238 appendix B vectors, at 8 digits. The SHA1 seed is the ASCII
// string "12345678901234567890" (20 bytes), spelled in Base32 in RFC 6238
// and as the hexadecimal 313233...3930 in RFC 4226's appendix. The SHA256
// seed is that string repeated to 32 bytes, the SHA512 seed to 64.
const (
	seedSHA1   = "12345678901234567890"
	seedSHA256 = "12345678901234567890123456789012"
	seedSHA512 = "1234567890123456789012345678901234567890123456789012345678901234"
)

// b32Of is the canonical Base32 spelling of a byte string: uppercase and
// unpadded, the form ParseOTP stores and hands back.
func b32Of(s string) string {
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(s))
}

var (
	b32SHA1   = b32Of(seedSHA1)
	b32SHA256 = b32Of(seedSHA256)
	b32SHA512 = b32Of(seedSHA512)
	b32Hello  = b32Of("Hello!\xde\xad\xbe\xef")
	b32Seq16  = b32Of("\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f")
)

// uri builds an otpauth URI from its pieces, so no long literal in this
// file can be mistaken for a secret by a reader.
func uri(secret, extra string) string {
	if extra == "" {
		return "otpauth://totp/x?secret=" + secret
	}
	return "otpauth://totp/x?secret=" + secret + "&" + extra
}

func TestTOTPMatchesRFC6238AppendixB(t *testing.T) {
	cases := []struct {
		at               int64
		sha1, sha2, sha5 string
	}{
		{59, "94287082", "46119246", "90693936"},
		{1111111109, "07081804", "68084774", "25091201"},
		{1111111111, "14050471", "67062674", "99943326"},
		{1234567890, "89005924", "91819424", "93441116"},
		{2000000000, "69279037", "90698825", "38618901"},
		{20000000000, "65353130", "77737706", "47863826"},
	}
	for _, c := range cases {
		tm := time.Unix(c.at, 0).UTC()
		for _, tc := range []struct {
			seed, algo, want string
		}{
			{seedSHA1, "SHA1", c.sha1},
			{seedSHA256, "SHA256", c.sha2},
			{seedSHA512, "SHA512", c.sha5},
		} {
			got, err := TOTP([]byte(tc.seed), tm, 30, 8, tc.algo)
			if err != nil {
				t.Fatalf("T=%d %s: %v", c.at, tc.algo, err)
			}
			if got != tc.want {
				t.Errorf("T=%d %s = %s, want %s", c.at, tc.algo, got, tc.want)
			}
		}
	}
}

func TestTOTPThroughParseOTPAndCode(t *testing.T) {
	// The same vectors through the whole path: parse a secret, ask for a
	// code. The RFC spells its table at 8 digits, so every row asks for 8.
	for _, c := range []struct {
		seed, b32, algo string
	}{
		{seedSHA1, b32SHA1, "SHA1"},
		{seedSHA256, b32SHA256, "SHA256"},
		{seedSHA512, b32SHA512, "SHA512"},
	} {
		o, err := ParseOTP(uri(c.b32, "algorithm="+c.algo+"&digits=8"))
		if err != nil {
			t.Fatalf("ParseOTP(%s): %v", c.algo, err)
		}
		if o.Algorithm != c.algo {
			t.Errorf("algorithm %q, want %q", o.Algorithm, c.algo)
		}
		if o.Digits != 8 || o.Period != 30 {
			t.Errorf("%s = %d digits, %d s", c.algo, o.Digits, o.Period)
		}
		if want := strings.ToUpper(strings.TrimRight(c.b32, "=")); o.Secret != want {
			t.Errorf("canonical secret %q, want %q", o.Secret, want)
		}
		for _, at := range []int64{59, 1111111109, 1111111111, 1234567890, 2000000000, 20000000000} {
			want, err := TOTP([]byte(c.seed), time.Unix(at, 0).UTC(), 30, 8, c.algo)
			if err != nil {
				t.Fatal(err)
			}
			got, err := o.Code(time.Unix(at, 0).UTC())
			if err != nil {
				t.Fatalf("Code: %v", err)
			}
			if got != want {
				t.Errorf("%s T=%d: Code = %s, want %s", c.algo, at, got, want)
			}
		}
	}
}

func TestTOTPDefaultsAndVariants(t *testing.T) {
	// A bare Base32 secret means 6 digits, 30 seconds, SHA1.
	o, err := ParseOTP(b32SHA1)
	if err != nil {
		t.Fatal(err)
	}
	if o.Digits != 6 || o.Period != 30 || o.Algorithm != "SHA1" {
		t.Fatalf("defaults = %d digits, %d s, %s", o.Digits, o.Period, o.Algorithm)
	}
	if got, err := o.Code(time.Unix(59, 0).UTC()); err != nil || got != "287082" {
		t.Errorf("6-digit SHA1 at T=59 = %q, %v", got, err)
	}
	if got, err := o.Code(time.Unix(1111111109, 0).UTC()); err != nil || got != "081804" {
		t.Errorf("6-digit SHA1 at T=1111111109 = %q, %v", got, err)
	}
	// A leading zero still shows.
	if got, err := o.Code(time.Unix(1111111111, 0).UTC()); err != nil || got != "050471" {
		t.Errorf("6-digit SHA1 at T=1111111111 = %q, %v", got, err)
	}
	// A different period changes the counter: the same instant counted in
	// 60-second steps, or in 15-second ones.
	long, err := ParseOTP(uri(b32SHA1, "period=60&digits=8"))
	if err != nil {
		t.Fatal(err)
	}
	if long.Period != 60 {
		t.Fatalf("period %d", long.Period)
	}
	if got, err := long.Code(time.Unix(1234567890, 0).UTC()); err != nil || got != "55713351" {
		t.Errorf("period 60 at T=1234567890 = %q, %v", got, err)
	}
	short, err := ParseOTP(uri(b32SHA1, "period=15&digits=8"))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := short.Code(time.Unix(100, 0).UTC()); err != nil || got != "18287922" {
		t.Errorf("period 15 at T=100 = %q, %v", got, err)
	}
	// 7 digits is inside 6-8.
	seven, err := ParseOTP(uri(b32SHA1, "digits=7"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := seven.Code(time.Unix(59, 0).UTC())
	if err != nil || len(got) != 7 {
		t.Errorf("7-digit code = %q, %v", got, err)
	}
	// The algorithm is matched case-insensitively, as URIs spell it both
	// ways, and is stored in its canonical spelling.
	for _, in := range []string{"sha256", "Sha256", "SHA256"} {
		a, err := ParseOTP(uri(b32SHA256, "algorithm="+in))
		if err != nil {
			t.Fatalf("ParseOTP(algorithm=%s): %v", in, err)
		}
		if a.Algorithm != "SHA256" {
			t.Errorf("algorithm %q normalized to %q", in, a.Algorithm)
		}
	}
}

func TestTOTPRemaining(t *testing.T) {
	o, err := ParseOTP(b32SHA1)
	if err != nil {
		t.Fatal(err)
	}
	if d := o.Remaining(time.Unix(59, 0).UTC()); d != 1*time.Second {
		t.Errorf("Remaining at T=59 = %v, want 1s", d)
	}
	if d := o.Remaining(time.Unix(59, 999999999).UTC()); d != 1*time.Nanosecond {
		t.Errorf("Remaining at T=59.999999999 = %v, want 1ns", d)
	}
	if d := o.Remaining(time.Unix(0, 0).UTC()); d != 30*time.Second {
		t.Errorf("Remaining at T=0 = %v, want 30s", d)
	}
	// T=120 starts a fresh 60-second step; T=119 is its last second.
	long, _ := ParseOTP(uri(b32SHA1, "period=60"))
	if d := long.Remaining(time.Unix(120, 0).UTC()); d != 60*time.Second {
		t.Errorf("Remaining at T=120 with period 60 = %v, want 60s", d)
	}
	if d := long.Remaining(time.Unix(119, 0).UTC()); d != 1*time.Second {
		t.Errorf("Remaining at T=119 with period 60 = %v, want 1s", d)
	}
}

func TestParseOTPToleratesMessyBase32(t *testing.T) {
	// Spaces, hyphens, tabs, newlines, lowercase, and padding the user may
	// or may not have typed: all the same secret, always the canonical form.
	const want = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	for _, in := range []string{
		"GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ",
		"gezdgnbvgy3tqojqgezdgnbvgy3tqojq",
		"  GEZD GNBV GY3T QOJQ GEZD GNBV GY3T QOJQ  ",
		"GEZD-GNBV-GY3T-QOJQ-GEZD-GNBV-GY3T-QOJQ",
		"ge zd gn bv gy 3t qo jq ge zd gn bv gy 3t qo jq",
		"GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ\n",
		"GEZD\tGNBV\tGY3T\tQOJQ\tGEZD\tGNBV\tGY3T\tQOJQ",
		"gezdGNBVgy3tQOJQGEZDGNBVGY3TQOJQ",
	} {
		o, err := ParseOTP(in)
		if err != nil {
			t.Errorf("ParseOTP(%q): %v", in, err)
			continue
		}
		if o.Secret != want {
			t.Errorf("ParseOTP(%q) secret = %q, want %q", in, o.Secret, want)
		}
		if o.Digits != 6 || o.Period != 30 || o.Algorithm != "SHA1" {
			t.Errorf("ParseOTP(%q) defaults = %d/%d/%s", in, o.Digits, o.Period, o.Algorithm)
		}
	}
	// Padding in the input is dropped, not required, and the parameters of
	// the URI still apply to the secret it carries.
	padded, err := ParseOTP(uri(b32SHA256+"====", "algorithm=SHA256&digits=8"))
	if err != nil {
		t.Fatal(err)
	}
	if padded.Secret != b32SHA256 {
		t.Errorf("padded secret = %q", padded.Secret)
	}
	if code, err := padded.Code(time.Unix(59, 0).UTC()); err != nil || code != "46119246" {
		t.Errorf("SHA256 code = %q, %v", code, err)
	}
}

func TestParseOTPBase32ByteValues(t *testing.T) {
	// The RFC 4648 appendix, both examples.
	o, err := ParseOTP(b32Hello)
	if err != nil {
		t.Fatal(err)
	}
	got, err := o.Code(time.Unix(59, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	want, _ := TOTP([]byte{0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x21, 0xde, 0xad, 0xbe, 0xef}, time.Unix(59, 0).UTC(), 30, 6, "SHA1")
	if got != want || want != "996554" {
		t.Errorf("code = %q, want %q", got, want)
	}
	seq, err := ParseOTP(b32Seq16 + "======")
	if err != nil {
		t.Fatal(err)
	}
	if seq.Secret != b32Seq16 {
		t.Errorf("secret = %q, want the canonical unpadded form", seq.Secret)
	}
}

func TestParseOTPOtpauthURI(t *testing.T) {
	// The fields of a real otpauth URI: percent-encoded label, issuer,
	// algorithm, digits and period, and the Base32 secret.
	o, err := ParseOTP("otpauth://totp/ACME%20Co:john@example.com?secret=" + b32SHA256 +
		"&algorithm=SHA256&digits=8&period=60&issuer=ACME%20Co")
	if err != nil {
		t.Fatal(err)
	}
	if o.Secret != b32SHA256 {
		t.Errorf("secret = %q", o.Secret)
	}
	if o.Algorithm != "SHA256" || o.Digits != 8 || o.Period != 60 {
		t.Errorf("algorithm = %q, digits = %d, period = %d", o.Algorithm, o.Digits, o.Period)
	}
	// An unencoded space in the label is not fatal; only the query matters.
	if _, err := ParseOTP("otpauth://totp/ACME Co:john@example.com?secret=" + b32SHA256); err != nil {
		t.Errorf("unencoded space in label: %v", err)
	}
	// The scheme and the type are case-insensitive.
	if _, err := ParseOTP("OTPAUTH://TOTP/x?secret=" + b32SHA1); err != nil {
		t.Errorf("uppercase scheme and type: %v", err)
	}
	// A URI with nothing beyond the secret still gets the defaults.
	bare, err := ParseOTP(uri(b32SHA1, ""))
	if err != nil {
		t.Fatal(err)
	}
	if bare.Digits != 6 || bare.Period != 30 || bare.Algorithm != "SHA1" {
		t.Errorf("bare URI defaults = %d/%d/%s", bare.Digits, bare.Period, bare.Algorithm)
	}
	// An empty label is not fatal.
	if _, err := ParseOTP("otpauth://totp/?secret=" + b32SHA1); err != nil {
		t.Errorf("empty label: %v", err)
	}
	// A parameter that is present but blank is a typo, not a request for the
	// default: answering it as SHA1/6/30 would hand back a code that fails.
	if _, err := ParseOTP(uri(b32SHA1, "algorithm=")); err == nil {
		t.Error("algorithm= was answered with the default")
	}
}

func TestParseOTPRejectsBadInput(t *testing.T) {
	// The same bytes spell a different string when the last character
	// carries bits that belong to no byte; the round trip catches it.
	foobar := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("foobar"))
	nonCanonical := foobar[:len(foobar)-1] + "J"
	if nonCanonical == foobar {
		t.Fatal("the probe is canonical, so it proves nothing")
	}
	for i, in := range []string{
		"",                                   // nothing at all
		"   ",                                // whitespace only
		"MY",                                 // one byte: too short for a second factor
		"MY======",                           // padding that is not a multiple of 8
		nonCanonical,                         // leftover bits in the last group
		"JBSWY3DPEH",                         // ten characters: 6 bytes and 2 spare bits
		"0O1lI",                              // not the Base32 alphabet
		"AB!",                                // a stray character
		"AB=",                                // padding in the middle
		"otpauth://totp/x",                   // a URI with no secret
		"otpauth://hotp/x?secret=" + b32SHA1, // not TOTP
		uri(b32SHA1, "digits=9"),             // above 8
		uri(b32SHA1, "digits=5"),             // below 6
		uri(b32SHA1, "digits=0"),             // not a digit count
		uri(b32SHA1, "digits=abc"),           // not a number
		uri(b32SHA1, "period=0"),             // a period of 0
		uri(b32SHA1, "period=-30"),           // a negative period
		uri(b32SHA1, "period=99999"),         // past the longest period offered
		uri(b32SHA1, "period=30s"),           // not a number
		uri(b32SHA1, "algorithm=SHA384"),
		uri(b32SHA1, "algorithm=MD5"),
		uri(b32SHA1, "algorithm= "),             // whitespace only
		uri(b32SHA1, "digits="),                 // blank, not absent
		uri(b32SHA1, "period="),                 // blank, not absent
		"http://example.com/?secret=" + b32SHA1, // the wrong scheme
		strings.Repeat("A", 400),                // a paste of a whole document
		uri(strings.Repeat("A", 400), ""),       // the same, inside a URI
	} {
		if o, err := ParseOTP(in); err == nil {
			t.Errorf("input %d accepted, with a %d-byte secret", i, len(o.Secret))
		}
	}
}

func TestTOTPRejectsBadParameters(t *testing.T) {
	for _, c := range []struct {
		name           string
		period, digits int
		algorithm      string
	}{
		{"period 0", 0, 6, "SHA1"},
		{"negative period", -30, 6, "SHA1"},
		{"period past the cap", 601, 6, "SHA1"},
		{"digits 5", 30, 5, "SHA1"},
		{"digits 9", 30, 9, "SHA1"},
		{"digits 0", 30, 0, "SHA1"},
		{"MD5", 30, 6, "MD5"},
		{"SHA384", 30, 6, "SHA384"},
		{"empty", 30, 6, ""},
	} {
		if _, err := TOTP([]byte(seedSHA1), time.Unix(59, 0).UTC(), c.period, c.digits, c.algorithm); err != ErrOTP {
			t.Errorf("TOTP %s: err = %v, want ErrOTP", c.name, err)
		}
	}
	// An empty, too short or over-long secret is refused rather than used.
	if _, err := TOTP(nil, time.Unix(59, 0).UTC(), 30, 6, "SHA1"); err != ErrOTP {
		t.Errorf("empty secret: err = %v", err)
	}
	if _, err := TOTP([]byte("short"), time.Unix(59, 0).UTC(), 30, 6, "SHA1"); err != ErrOTP {
		t.Errorf("short secret: err = %v", err)
	}
	if _, err := TOTP(bytes.Repeat([]byte{0x41}, 9), time.Unix(59, 0).UTC(), 30, 6, "SHA1"); err != ErrOTP {
		t.Errorf("nine-byte secret: err = %v", err)
	}
	if _, err := TOTP(bytes.Repeat([]byte{0x41}, 129), time.Unix(59, 0).UTC(), 30, 6, "SHA1"); err != ErrOTP {
		t.Errorf("over-long secret: err = %v", err)
	}
	// The length limits are inclusive at both ends.
	if _, err := TOTP(bytes.Repeat([]byte{0x41}, 10), time.Unix(59, 0).UTC(), 30, 6, "SHA1"); err != nil {
		t.Errorf("ten-byte secret: %v", err)
	}
	if _, err := TOTP(bytes.Repeat([]byte{0x41}, 128), time.Unix(59, 0).UTC(), 30, 6, "SHA1"); err != nil {
		t.Errorf("128-byte secret: %v", err)
	}
}

func TestOTPCodeStaysStableAcrossTheStep(t *testing.T) {
	o, err := ParseOTP(b32SHA1)
	if err != nil {
		t.Fatal(err)
	}
	first, err := o.Code(time.Unix(59, 500000000).UTC())
	if err != nil {
		t.Fatal(err)
	}
	last, err := o.Code(time.Unix(59, 999999999).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if first != last {
		t.Errorf("code changed inside one step: %s then %s", first, last)
	}
}

func TestTOTPErrorTextCarriesNoSecret(t *testing.T) {
	_, err := ParseOTP("JBSWY3DPEH")
	if err == nil {
		t.Fatal("a non-canonical secret was accepted")
	}
	if strings.Contains(err.Error(), "JBSWY3DPEH") {
		t.Errorf("error text echoes the secret: %v", err)
	}
	_, err = TOTP([]byte(seedSHA1), time.Unix(59, 0).UTC(), 30, 6, "MD5")
	if err == nil {
		t.Fatal("MD5 was accepted")
	}
	if strings.Contains(err.Error(), "MD5") {
		t.Errorf("error text names the algorithm: %v", err)
	}
}

func TestOTPStringRoundTrips(t *testing.T) {
	// The URI form carries everything back through ParseOTP.
	for _, o := range []OTP{
		{Secret: b32SHA1, Digits: 6, Period: 30, Algorithm: "SHA1"},
		{Secret: b32SHA256, Digits: 8, Period: 60, Algorithm: "SHA256"},
		{Secret: b32Seq16, Digits: 7, Period: 15, Algorithm: "SHA512"},
	} {
		back, err := ParseOTP(o.String())
		if err != nil {
			t.Fatalf("ParseOTP(%s): %v", o.Algorithm, err)
		}
		if back != o {
			t.Errorf("round trip: %+v, want %+v", back, o)
		}
		a, err1 := o.Code(time.Unix(1234567890, 0).UTC())
		b, err2 := back.Code(time.Unix(1234567890, 0).UTC())
		if err1 != nil || err2 != nil || a != b {
			t.Errorf("code after round trip: %q vs %q", a, b)
		}
	}
}

func TestOTPIsJSONRoundTrippable(t *testing.T) {
	// The stored form is JSON in the item payload, so the field names are
	// part of the format.
	in := []byte(`{"secret":"` + b32SHA1 + `","digits":6,"period":30,"algorithm":"SHA1"}`)
	var o OTP
	if err := json.Unmarshal(in, &o); err != nil {
		t.Fatal(err)
	}
	if o.Secret != b32SHA1 || o.Digits != 6 || o.Period != 30 || o.Algorithm != "SHA1" {
		t.Fatalf("unmarshalled %+v", o)
	}
	out, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(in) {
		t.Errorf("re-marshalled %s, want %s", out, in)
	}
}
