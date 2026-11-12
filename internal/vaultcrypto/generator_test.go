package vaultcrypto

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

// The four classes, in the order the generator reads them. Every character of
// a password comes from the union, so the order is only about which class a
// character happens to land in.
const (
	classLower   = "abcdefghijklmnopqrstuvwxyz"
	classUpper   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	classDigits  = "0123456789"
	classSymbols = "!@#$%^&*"
)

// charset is the union of the four classes, in the order DefaultGenOptions
// enables them.
func charset(lower, upper, digits, symbols, noAmbiguous bool) string {
	var b strings.Builder
	if lower {
		b.WriteString(classLower)
	}
	if upper {
		b.WriteString(classUpper)
	}
	if digits {
		b.WriteString(classDigits)
	}
	if symbols {
		b.WriteString(classSymbols)
	}
	s := b.String()
	if noAmbiguous {
		s = strings.Map(func(r rune) rune {
			if strings.ContainsRune("0O1lI", r) {
				return -1
			}
			return r
		}, s)
	}
	return s
}

// byteStream is a deterministic reader: the bytes are handed out in order.
// A byte the generator rejects costs an extra draw, so callers that want
// exactly one draw per character pass more bytes than the length.
func byteStream(b ...byte) *bytes.Reader {
	return bytes.NewReader(b)
}

// fillStream is enough bytes for a password of n characters with room for
// retries, in case a drawn string misses an enabled class.
func fillStream(n int) []byte {
	return mustFill(n * 4)
}

func TestGeneratePasswordLength(t *testing.T) {
	for _, n := range []int{8, 9, 20, 64, 127, 128} {
		p, err := GeneratePassword(GenOptions{Length: n, Lower: true, Upper: true, Digits: true, Symbols: true}, byteStream(fillStream(n)...))
		if err != nil {
			t.Fatalf("length %d: %v", n, err)
		}
		if len(p) != n {
			t.Errorf("length %d gave %d characters", n, len(p))
		}
	}
}

// classStart is the index of the first character of each class in the default
// 70-character alphabet: lower, upper, digits, symbols.
var classStart = []byte{0, 26, 52, 62}

// mustFill is a byte stream that draws one character from each class in turn,
// so the first four characters cover all four and every byte stays below the
// rejection limit of the default alphabet.
func mustFill(n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = classStart[i%len(classStart)] + byte(i/len(classStart))
	}
	return out
}

func TestGeneratePasswordDrawsTheClassesInAlphabetOrder(t *testing.T) {
	// 0, 26, 52 and 63 index lower, upper, digits and symbols in the default
	// 70-character alphabet, so this stream spells "aA0&" for the first four
	// characters of a length-8 password.
	opts := GenOptions{Length: 8, Lower: true, Upper: true, Digits: true, Symbols: true}
	p, err := GeneratePassword(opts, byteStream(0, 26, 52, 62, 1, 27, 53, 63))
	if err != nil {
		t.Fatal(err)
	}
	if want := "aA0!bB1@"; p != want {
		t.Errorf("password = %q, want %q", p, want)
	}
}

func TestGeneratePasswordRetriesWhenAClassIsMissing(t *testing.T) {
	// The first eight characters cover three classes; the string is discarded
	// whole and drawn again, not patched, so the retry reproduces the same
	// spelling shifted by one.
	opts := GenOptions{Length: 8, Lower: true, Upper: true, Digits: true, Symbols: true}
	stream := append([]byte{0, 26, 52, 1, 27, 53, 2, 28}, 0, 26, 52, 62, 1, 27, 53, 63)
	p, err := GeneratePassword(opts, byteStream(stream...))
	if err != nil {
		t.Fatal(err)
	}
	if want := "aA0!bB1@"; p != want {
		t.Errorf("after a missing class = %q, want %q", p, want)
	}
	// Eight characters of one class alone is refused forever, so the stream
	// runs out and the error is the reader's, not a short password.
	if _, err := GeneratePassword(opts, byteStream(bytes.Repeat([]byte{0}, 24)...)); err == nil {
		t.Error("an all-lowercase draw was accepted")
	}
}

func TestGeneratePasswordRejectsBytesPastTheLimit(t *testing.T) {
	// 256 % 70 is 46, so bytes 46..255 would bias the alphabet. The generator
	// draws again rather than use them: byte 255 is skipped and the next byte
	// decides.
	opts := GenOptions{Length: 8, Lower: true}
	p, err := GeneratePassword(opts, byteStream(255, 0, 1, 2, 3, 4, 5, 6, 7))
	if err != nil {
		t.Fatal(err)
	}
	if want := "abcdefgh"; p != want {
		t.Errorf("byte 255 then 0..6 = %q, want %q", p, want)
	}
	// A stream that runs out after a rejected byte is an error, not a short
	// password.
	if _, err := GeneratePassword(opts, byteStream(255)); err == nil {
		t.Error("an exhausted stream was not reported")
	}
	// The boundary itself is kept: byte 45 is the last one that maps evenly.
	short := GenOptions{Length: 8, Digits: true}
	if got, err := GeneratePassword(short, byteStream(bytes.Repeat([]byte{45}, 8)...)); err != nil || got != "55555555" {
		t.Errorf("byte 45 with the digit alphabet = %q, %v", got, err)
	}
}

func TestGeneratePasswordNoAmbiguousDropsLookalikes(t *testing.T) {
	full := charset(true, true, true, true, false)
	trimmed := charset(true, true, true, true, true)
	for _, r := range "0O1lI" {
		if strings.ContainsRune(full, r) {
			continue
		}
		t.Errorf("%q is not in the default alphabet", r)
	}
	for _, r := range "0O1lI" {
		if strings.ContainsRune(trimmed, r) {
			t.Errorf("%q survived NoAmbiguous", r)
		}
	}
	if len(trimmed) != 65 {
		t.Errorf("NoAmbiguous alphabet has %d characters, want 65", len(trimmed))
	}
	p, err := GeneratePassword(GenOptions{Length: 30, NoAmbiguous: true, Lower: true, Upper: true, Digits: true, Symbols: true}, byteStream(fillStream(30)...))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range p {
		if strings.ContainsRune("0O1lI", r) {
			t.Errorf("password contains the ambiguous %q", r)
		}
	}
}

func TestGeneratePasswordRefusesAnEmptyAlphabet(t *testing.T) {
	for _, o := range []GenOptions{
		{Length: 20},
		{Length: 20, Lower: false, Upper: false, Digits: false, Symbols: false},
	} {
		if _, err := GeneratePassword(o, byteStream(1, 2, 3)); err != ErrGenOptions {
			t.Errorf("no classes enabled: err = %v", err)
		}
	}
}

func TestGeneratePasswordRefusesABadLength(t *testing.T) {
	for _, n := range []int{0, -1, 7, 129, 1000} {
		if _, err := GeneratePassword(GenOptions{Length: n, Lower: true}, byteStream(1, 2, 3)); err != ErrGenOptions {
			t.Errorf("length %d: err = %v", n, err)
		}
	}
}

func TestGeneratePasswordEveryLengthIsReachable(t *testing.T) {
	// The shortest and longest accepted lengths both produce a password of
	// that exact length, and the alphabet is only as wide as the classes.
	for _, n := range []int{8, 128} {
		o := GenOptions{Length: n, Lower: true}
		p, err := GeneratePassword(o, byteStream(fillStream(n)...))
		if err != nil || len(p) != n {
			t.Errorf("length %d = %q, %v", n, p, err)
		}
	}
}

func TestDefaultGenOptions(t *testing.T) {
	o := DefaultGenOptions()
	if o.Length != 20 {
		t.Errorf("default length = %d", o.Length)
	}
	if !o.Lower || !o.Upper || !o.Digits || !o.Symbols {
		t.Errorf("default classes = %+v", o)
	}
	if o.NoAmbiguous {
		t.Error("NoAmbiguous is on by default")
	}
}

func TestEntropyBits(t *testing.T) {
	for _, c := range []struct {
		o    GenOptions
		want float64
	}{
		{GenOptions{Length: 20, Lower: true, Upper: true, Digits: true, Symbols: true}, 122.58566033889933},
		{GenOptions{Length: 20, Lower: true, Upper: true, Digits: true, Symbols: true, NoAmbiguous: true}, 120.44735626056908},
		{GenOptions{Length: 12, Lower: true, Upper: true, Digits: true, Symbols: true}, 73.55139620333959},
		{GenOptions{Length: 8, Lower: true}, 37.603517745128734},
		{GenOptions{Length: 8, Digits: true}, 26.575424759098897},
		{GenOptions{Length: 20}, 0},
	} {
		got := c.o.EntropyBits()
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%+v entropy = %v, want %v", c.o, got, c.want)
		}
	}
}

func TestStrengthLevel(t *testing.T) {
	for _, c := range []struct {
		bits float64
		want int
	}{
		{0, 0},
		{49.9, 0},
		{50, 1},
		{74.9, 1},
		{75, 2},
		{99.9, 2},
		{100, 3},
		{200, 3},
	} {
		if got := StrengthLevel(c.bits); got != c.want {
			t.Errorf("StrengthLevel(%v) = %d, want %d", c.bits, got, c.want)
		}
	}
}

func TestGeneratePasswordIsDeterministicForASeededReader(t *testing.T) {
	// Two runs over the same bytes agree, which is what makes the vectors
	// replayable.
	o := GenOptions{Length: 32, Lower: true, Upper: true, Digits: true, Symbols: true}
	seed := fillStream(32)
	a, err := GeneratePassword(o, byteStream(seed...))
	if err != nil {
		t.Fatal(err)
	}
	b, err := GeneratePassword(o, byteStream(seed...))
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Errorf("same seed gave %q and %q", a, b)
	}
}

func TestGeneratePasswordDrawsFromTheWholeAlphabet(t *testing.T) {
	// Every one of the 70 default characters is reachable: 70 consecutive
	// bytes index the whole alphabet, and none is rejected.
	opts := GenOptions{Length: 70, Lower: true, Upper: true, Digits: true, Symbols: true}
	seed := make([]byte, 70)
	for i := range seed {
		seed[i] = byte(i)
	}
	p, err := GeneratePassword(opts, byteStream(seed...))
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 70 {
		t.Fatalf("%d characters", len(p))
	}
	seen := map[rune]bool{}
	for _, r := range p {
		seen[r] = true
	}
	if len(seen) != 70 {
		t.Errorf("saw %d distinct characters, want 70", len(seen))
	}
}
