package vaultcrypto

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"crypto/sha512"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// MnemonicWords is the length of every bipPass mnemonic: 24 words carry
// 256 bits of entropy and an 8-bit checksum.
const MnemonicWords = 24

// entropySize is the entropy of a 24-word mnemonic, in bytes.
const entropySize = 32

// englishWordlist is the BIP39 English wordlist, one word per line.
//
//go:embed english.txt
var englishWordlist string

var (
	englishWords = strings.Fields(englishWordlist)
	englishIndex = func() map[string]int {
		if len(englishWords) != 2048 {
			panic("vaultcrypto: english.txt does not hold 2048 words")
		}
		index := make(map[string]int, len(englishWords))
		for i, w := range englishWords {
			index[w] = i
		}
		return index
	}()
)

// The kinds of mnemonic errors. ValidateMnemonic wraps them in a
// *MnemonicError, which tells where the problem is without repeating any
// word of the mnemonic.
var (
	ErrMnemonicLength   = errors.New("mnemonic must be 24 words")
	ErrMnemonicWord     = errors.New("mnemonic word not in the BIP39 English list")
	ErrMnemonicChecksum = errors.New("mnemonic checksum mismatch")
)

// MnemonicError tells what is wrong with a mnemonic.
type MnemonicError struct {
	// Err is ErrMnemonicLength, ErrMnemonicWord or ErrMnemonicChecksum.
	Err error
	// Words is the number of words the mnemonic holds.
	Words int
	// Position is the 1-based position of the first word not in the
	// list, for ErrMnemonicWord; 0 otherwise.
	Position int
}

func (e *MnemonicError) Error() string {
	switch e.Err {
	case ErrMnemonicLength:
		return fmt.Sprintf("%v, got %d", e.Err, e.Words)
	case ErrMnemonicWord:
		return fmt.Sprintf("%v: word %d", e.Err, e.Position)
	}
	return e.Err.Error()
}

func (e *MnemonicError) Unwrap() error { return e.Err }

// GenerateMnemonic makes a new 24-word mnemonic from 32 bytes of r, which
// is crypto/rand.Reader outside tests.
func GenerateMnemonic(r io.Reader) (string, error) {
	entropy := make([]byte, entropySize)
	defer clear(entropy)
	if _, err := io.ReadFull(r, entropy); err != nil {
		return "", fmt.Errorf("vaultcrypto: reading mnemonic entropy: %w", err)
	}
	return MnemonicFromEntropy(entropy)
}

// MnemonicFromEntropy encodes 32 bytes of entropy as a BIP39 mnemonic.
func MnemonicFromEntropy(entropy []byte) (string, error) {
	if len(entropy) != entropySize {
		return "", fmt.Errorf("vaultcrypto: mnemonic entropy is %d bytes, want %d", len(entropy), entropySize)
	}
	sum := sha256.Sum256(entropy)
	data := make([]byte, 0, entropySize+1)
	data = append(append(data, entropy...), sum[0])
	defer clear(data)
	words := make([]string, MnemonicWords)
	for i := range words {
		words[i] = englishWords[readBits11(data, i*11)]
	}
	return strings.Join(words, " "), nil
}

// NormalizeMnemonic puts a mnemonic as the user typed or pasted it into
// its canonical form: NFKD, lowercase, invisible format characters
// (zero-width spaces, byte order marks, soft hyphens) removed, and every
// run of white space, line breaks and ideographic spaces included, made
// one space.
func NormalizeMnemonic(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) {
			return -1
		}
		return unicode.ToLower(r)
	}, norm.NFKD.String(s))
	return strings.Join(strings.Fields(s), " ")
}

// ValidateMnemonic reports whether s, once normalized, is a 24-word BIP39
// English mnemonic with a valid checksum. Its errors are *MnemonicError.
func ValidateMnemonic(s string) error {
	words := strings.Fields(NormalizeMnemonic(s))
	if len(words) != MnemonicWords {
		return &MnemonicError{Err: ErrMnemonicLength, Words: len(words)}
	}
	data := make([]byte, entropySize+1)
	defer clear(data)
	for i, w := range words {
		index, ok := englishIndex[w]
		if !ok {
			return &MnemonicError{Err: ErrMnemonicWord, Words: len(words), Position: i + 1}
		}
		writeBits11(data, i*11, index)
	}
	sum := sha256.Sum256(data[:entropySize])
	if sum[0] != data[entropySize] {
		return &MnemonicError{Err: ErrMnemonicChecksum, Words: len(words)}
	}
	return nil
}

// MnemonicSeed derives the 64-byte BIP39 seed of a mnemonic:
// PBKDF2-HMAC-SHA512 of the normalized mnemonic with the salt "mnemonic"
// followed by the NFKD form of passphrase (BIP39's optional 25th word),
// 2048 iterations. It does not validate the mnemonic.
func MnemonicSeed(mnemonic, passphrase string) []byte {
	salt := []byte("mnemonic" + norm.NFKD.String(passphrase))
	seed, err := pbkdf2.Key(sha512.New, NormalizeMnemonic(mnemonic), salt, 2048, 64)
	if err != nil {
		// crypto/pbkdf2 fails only in FIPS 140-only mode, whose minimum salt
		// length rules out BIP39; bipPass does not run in it.
		panic("vaultcrypto: " + err.Error())
	}
	return seed
}

// readBits11 reads the 11 bits of data from bit offset on, the most
// significant bit first.
func readBits11(data []byte, offset int) int {
	v := 0
	for bit := offset; bit < offset+11; bit++ {
		v = v<<1 | int(data[bit/8]>>(7-bit%8)&1)
	}
	return v
}

// writeBits11 writes the low 11 bits of v into data from bit offset on,
// the most significant bit first.
func writeBits11(data []byte, offset, v int) {
	for i := range 11 {
		bit := offset + i
		if v>>(10-i)&1 == 1 {
			data[bit/8] |= 1 << (7 - bit%8)
		}
	}
}
