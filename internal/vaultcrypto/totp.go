package vaultcrypto

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// ErrOTP is the one error of every rejected two-factor secret or parameter:
// an input that is not a Base32 secret or an otpauth URI, a non-canonical
// Base32 string, an unknown algorithm, or a digit count outside 6-8. It
// never says which, and never repeats the input.
var ErrOTP = errors.New("vaultcrypto: invalid two-factor secret")

// The algorithms an OTP secret may use. SHA1 is what every mainstream
// authenticator ships; the other two are accepted because some services
// offer them.
const (
	otpSHA1   = "SHA1"
	otpSHA256 = "SHA256"
	otpSHA512 = "SHA512"
)

// OTP is a stored two-factor secret: everything needed to compute a code,
// and nothing else. Secret holds the canonical Base32 form — uppercase,
// no padding — so the same secret compares equal however it was typed.
type OTP struct {
	Secret    string `json:"secret"`
	Digits    int    `json:"digits"`
	Period    int    `json:"period"`
	Algorithm string `json:"algorithm"`
}

// ParseOTP reads a two-factor secret: either a bare Base32 secret or an
// otpauth://totp/ URI carrying one. Missing fields default to 6 digits,
// 30 seconds and SHA1.
func ParseOTP(input string) (OTP, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return OTP{}, ErrOTP
	}
	if q, ok := otpauthQuery(trimmed); ok {
		o, err := parseOTPAuth(q)
		if err != nil {
			return OTP{}, err
		}
		return o, nil
	}
	secret, err := decodeBase32(trimmed)
	if err != nil {
		return OTP{}, ErrOTP
	}
	return OTP{Secret: secret, Digits: 6, Period: 30, Algorithm: otpSHA1}, nil
}

// Code is the code valid at t. The clock comes from the caller so tests
// pin it. The stored secret is Base32, so it is decoded once here rather
// than kept as raw bytes: the canonical form is what the item payload and
// the JSON round trip carry.
func (o OTP) Code(t time.Time) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(o.Secret)
	if err != nil {
		return "", ErrOTP
	}
	return TOTP(key, t, o.Period, o.Digits, o.Algorithm)
}

// Remaining is how long the current code stays valid. A code is valid for
// one whole period from the start of its own step, so at the start of a
// step the whole period is left.
func (o OTP) Remaining(t time.Time) time.Duration {
	if o.Period <= 0 {
		return 0
	}
	step := int64(o.Period) * int64(time.Second)
	elapsed := t.UnixNano() % step
	if elapsed < 0 {
		elapsed += step
	}
	return time.Duration(step - elapsed)
}

// minOTPKeyLen is the shortest secret accepted. RFC 4226 asks for 160 bits;
// ten bytes keeps an authenticator's real secret and refuses the one- and
// two-byte strings that come from a mis-copy rather than from a service.
const minOTPKeyLen = 10

// maxOTPKeyLen is the longest secret accepted: far past the 20 bytes RFC
// 4226 recommends, and short enough that a paste of a whole document is
// refused rather than turned into a key.
const maxOTPKeyLen = 128

// maxOTPPeriod is the longest period an otpauth URI may ask for. Authenticator
// apps use 30 or 60 seconds; anything longer is a broken link.
const maxOTPPeriod = 600

// TOTP is the RFC 6238 code for key at time t: the counter is the number of
// whole periods since the epoch, fed to RFC 4226 dynamic truncation.
func TOTP(key []byte, t time.Time, period, digits int, algorithm string) (string, error) {
	if period <= 0 || period > maxOTPPeriod || digits < 6 || digits > 8 {
		return "", ErrOTP
	}
	if len(key) < minOTPKeyLen || len(key) > maxOTPKeyLen {
		return "", ErrOTP
	}
	newHash := hashFor(algorithm)
	if newHash == nil {
		return "", ErrOTP
	}
	mac := hmac.New(newHash, key)
	counter := uint64(t.Unix()) / uint64(period)
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac.Write(buf[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := (uint32(sum[offset]&0x7f) << 24) |
		(uint32(sum[offset+1]) << 16) |
		(uint32(sum[offset+2]) << 8) |
		uint32(sum[offset+3])
	// The code is zero padded to its width: 07081804, not 7081804.
	return fmt.Sprintf("%0*d", digits, value%pow10(digits)), nil
}

// hashFor is nil for any algorithm that is not offered.
func hashFor(algorithm string) func() hash.Hash {
	switch strings.ToUpper(algorithm) {
	case otpSHA1:
		return sha1.New
	case otpSHA256:
		return sha256.New
	case otpSHA512:
		return sha512.New
	}
	return nil
}

func pow10(digits int) uint32 {
	p := uint32(1)
	for i := 0; i < digits; i++ {
		p *= 10
	}
	return p
}

// otpauthQuery splits an otpauth URI into its query. Only TOTP is offered:
// an HOTP URI needs a counter this build has nowhere to keep, so it is
// refused rather than answered with the wrong code.
func otpauthQuery(uri string) (url.Values, bool) {
	if len(uri) < 8 {
		return nil, false
	}
	u, err := url.Parse(uri)
	if err != nil || !strings.EqualFold(u.Scheme, "otpauth") {
		return nil, false
	}
	// otpauth://totp/<label>?<parameters>: the type is the host. Anything
	// else is not a TOTP URI, so it is read as a bare secret and fails there.
	if !strings.EqualFold(u.Host, "totp") {
		return nil, false
	}
	return u.Query(), true
}

// parseOTPAuth reads the secret and the parameters of an otpauth query.
func parseOTPAuth(q url.Values) (OTP, error) {
	secret, err := decodeBase32(q.Get("secret"))
	if err != nil {
		return OTP{}, ErrOTP
	}
	o := OTP{Secret: secret, Digits: 6, Period: 30, Algorithm: otpSHA1}
	// A parameter that is absent keeps the default. Anything else has to
	// name something real: a blank value is a typo, not a request for the
	// default, so it is refused rather than quietly answered as SHA1.
	if q.Has("algorithm") {
		if hashFor(q.Get("algorithm")) == nil {
			return OTP{}, ErrOTP
		}
		o.Algorithm = strings.ToUpper(q.Get("algorithm"))
	}
	if q.Has("digits") {
		d, err := strconv.Atoi(q.Get("digits"))
		if err != nil || d < 6 || d > 8 {
			return OTP{}, ErrOTP
		}
		o.Digits = d
	}
	if q.Has("period") {
		p, err := strconv.Atoi(q.Get("period"))
		if err != nil || p <= 0 || p > maxOTPPeriod {
			return OTP{}, ErrOTP
		}
		o.Period = p
	}
	return o, nil
}

// decodeBase32 reads a Base32 secret in the sloppy forms real services
// hand out: lowercase, in groups separated by spaces or hyphens, with or
// without padding. It returns the canonical form and refuses anything that
// is not exactly one encoding of its bytes, so a mistyped character cannot
// silently become a different secret.
func decodeBase32(s string) (string, error) {
	clean := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '-' {
			return -1
		}
		return r
	}, s)
	if clean == "" {
		return "", ErrOTP
	}
	upper := strings.ToUpper(strings.TrimRight(clean, "="))
	if upper == "" {
		return "", ErrOTP
	}
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	decoded, err := enc.DecodeString(upper)
	if err != nil {
		return "", ErrOTP
	}
	// Canonical round trip: the same bytes back, which fails when the last
	// group holds bits that do not belong to any byte. A mistyped character
	// there would otherwise become a different, working secret.
	if enc.EncodeToString(decoded) != upper {
		return "", ErrOTP
	}
	// The length limits are checked here, on the decoded secret, so a bare
	// secret and a URI secret are held to the same rule.
	if len(decoded) < minOTPKeyLen || len(decoded) > maxOTPKeyLen {
		return "", ErrOTP
	}
	return upper, nil
}

// String is the otpauth URI of the secret, the form a QR code carries.
func (o OTP) String() string {
	v := url.Values{}
	v.Set("secret", o.Secret)
	if o.Digits != 6 {
		v.Set("digits", strconv.Itoa(o.Digits))
	}
	if o.Period != 30 {
		v.Set("period", strconv.Itoa(o.Period))
	}
	if o.Algorithm != otpSHA1 {
		v.Set("algorithm", o.Algorithm)
	}
	return "otpauth://totp/?" + v.Encode()
}
