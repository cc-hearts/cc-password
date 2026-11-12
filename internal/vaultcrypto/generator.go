package vaultcrypto

import (
	"errors"
	"io"
	"math"
)

// ErrGenOptions is the one error of a generation request that cannot be
// honoured: a length outside 8-128, or every character class turned off, which
// leaves no alphabet to draw from.
var ErrGenOptions = errors.New("vaultcrypto: unusable password options")

// The bounds of a generated password. Eight is the shortest that still has
// room for every class and some entropy; 128 is past anything a person types
// by hand and still well inside a field's limits.
const (
	minGenLen = 8
	maxGenLen = 128
)

// The four classes a password may draw from, in the order they are joined.
// The symbol class is the printable punctuation that every field accepts and
// no service asks users to escape.
const (
	genLower   = "abcdefghijklmnopqrstuvwxyz"
	genUpper   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	genDigits  = "0123456789"
	genSymbols = "!@#$%^&*"
)

// The characters that look alike in the fonts password fields use: zero and
// capital O, one and lowercase L, capital I. NoAmbiguous leaves them out.
const ambiguousChars = "0O1lI"

// GenOptions is a generation request. A character class that is off is absent
// from the alphabet; at least one has to be on.
type GenOptions struct {
	Length      int  `json:"length"`
	Lower       bool `json:"lower"`
	Upper       bool `json:"upper"`
	Digits      bool `json:"digits"`
	Symbols     bool `json:"symbols"`
	NoAmbiguous bool `json:"noAmbiguous"`
}

// DefaultGenOptions is what the new-password screen offers: 20 characters,
// all four classes, ambiguous characters kept because a person can still read
// them next to their neighbours.
func DefaultGenOptions() GenOptions {
	return GenOptions{Length: 20, Lower: true, Upper: true, Digits: true, Symbols: true}
}

// alphabet is the set of characters these options can produce. It is empty
// when every class is off.
func (o GenOptions) alphabet() string {
	var b []byte
	if o.Lower {
		b = append(b, genLower...)
	}
	if o.Upper {
		b = append(b, genUpper...)
	}
	if o.Digits {
		b = append(b, genDigits...)
	}
	if o.Symbols {
		b = append(b, genSymbols...)
	}
	if o.NoAmbiguous {
		out := b[:0]
		for _, c := range b {
			if !isAmbiguous(c) {
				out = append(out, c)
			}
		}
		b = out
	}
	return string(b)
}

func isAmbiguous(c byte) bool {
	for i := 0; i < len(ambiguousChars); i++ {
		if ambiguousChars[i] == c {
			return true
		}
	}
	return false
}

// valid reports whether these options describe a password that can be
// generated at all.
func (o GenOptions) valid() bool {
	return o.Length >= minGenLen && o.Length <= maxGenLen && len(o.alphabet()) > 0
}

// GeneratePassword draws a password of o.Length characters from the alphabet
// of the enabled classes. Randomness comes from r, which the caller supplies
// so tests can pin it; a production caller passes crypto/rand.
//
// Each character reads one byte and uses it only if it is below the largest
// multiple of the alphabet size that fits in a byte, so no character is more
// likely than any other. A byte past that limit is skipped and the next one
// read.
//
// If a drawn password is missing a character from an enabled class, the whole
// string is drawn again rather than one character being replaced: replacing
// one would make the class distribution uneven, and a shorter password is not
// what was asked for.
func GeneratePassword(o GenOptions, r io.Reader) (string, error) {
	if !o.valid() {
		return "", ErrGenOptions
	}
	chars := o.alphabet()
	n := len(chars)
	// The largest multiple of n that fits in a byte. A byte at or above it is
	// discarded, so every kept byte maps to a character with equal
	// probability.
	limit := 256 - 256%n
	out := make([]byte, 0, o.Length)
	for {
		out = out[:0]
		for len(out) < o.Length {
			var buf [1]byte
			if _, err := io.ReadFull(r, buf[:]); err != nil {
				return "", err
			}
			b := int(buf[0])
			if b >= limit {
				continue
			}
			out = append(out, chars[b%n])
		}
		if o.hasEveryClass(out) {
			return string(out), nil
		}
	}
}

// hasEveryClass reports whether the drawn password holds at least one
// character of every class the options enabled. A class the options left out
// is not missing, whatever the string happens to hold.
func (o GenOptions) hasEveryClass(pw []byte) bool {
	var lower, upper, digits, symbols bool
	for _, c := range pw {
		switch {
		case c >= 'a' && c <= 'z':
			lower = true
		case c >= 'A' && c <= 'Z':
			upper = true
		case c >= '0' && c <= '9':
			digits = true
		default:
			symbols = true
		}
	}
	return (!o.Lower || lower) &&
		(!o.Upper || upper) &&
		(!o.Digits || digits) &&
		(!o.Symbols || symbols)
}

// EntropyBits is how much guessing work a password of these options costs, in
// bits: the alphabet size raised to the length, as a base-2 logarithm.
func (o GenOptions) EntropyBits() float64 {
	n := len(o.alphabet())
	if n == 0 || o.Length <= 0 {
		return 0
	}
	return float64(o.Length) * math.Log2(float64(n))
}

// StrengthLevel grades entropy the way the generator screen shows it: 0 weak
// under 50 bits, 1 fair under 75, 2 strong under 100, 3 very strong at 100
// and above.
func StrengthLevel(bits float64) int {
	switch {
	case bits < 50:
		return 0
	case bits < 75:
		return 1
	case bits < 100:
		return 2
	default:
		return 3
	}
}
