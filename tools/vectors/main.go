// Command vectors writes spec/crypto-vectors.json, the byte-level contract
// every implementation of the ccpass format has to agree on: BIP39, the key
// tree, Argon2id, root-key wrapping, the item and vault-meta envelopes, TOTP,
// Base32 and the password generator.
//
//	go run ./tools/vectors          # write the file
//	go run ./tools/vectors -check   # fail if the file differs
//
// The file is committed, so -check is what CI runs: a change to the crypto
// that alters a vector is a format change and has to be a deliberate edit,
// not an accident of a refactor.
package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"ccpass/internal/vaultcrypto"
)

// vectorsVersion is the schema of the file. A breaking change to a section
// bumps it, so a reader can refuse a file it does not understand.
const vectorsVersion = 1

const vectorsPath = "spec/crypto-vectors.json"

// The fixed inputs every vector is derived from. Deterministic on purpose:
// the point of the file is that two implementations agree on the same bytes,
// which is impossible with fresh randomness.
var (
	entropyHex = []string{
		"0000000000000000000000000000000000000000000000000000000000000000",
		"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		"0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f",
		"77c2b778b5c0b6d45a0f0e6ab6ae1e00e0f2a3b4c5d6e7f8091a2b3c4d5e6f70",
	}
	// The RFC 6238 appendix B seeds: ASCII "1234567890" repeated to the
	// length each algorithm uses.
	seedSHA1   = "12345678901234567890"
	seedSHA256 = "12345678901234567890123456789012"
	seedSHA512 = "1234567890123456789012345678901234567890123456789012345678901234"
	// The RFC 4648 appendix examples: "Hello!" followed by the bytes
	// deadbeef, and the sixteen bytes 00 through 0f.
	base32Hello = "Hello!\xde\xad\xbe\xef"
	base32Seq   = "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f"

	_ = base32Hello
	_ = base32Seq

	// bip39Words is the BIP39 English word list, as a single space-separated
	// string, so a vector can try words by name.
	bip39Words = strings.Join(mustBip39Words(), " ")
)

// fixedNonce is the 24-byte nonce of every envelope vector, and fixedSalt the
// 16-byte Argon2id salt. A vector is only replayable with a fixed one.
var (
	fixedNonce = bytes.Repeat([]byte{0x42}, vaultcrypto.NonceSize)
	fixedSalt  = bytes.Repeat([]byte{0x5a}, vaultcrypto.SaltSize)
	// fixedRoot is the root key of the envelope vectors.
	fixedRoot     = bytes.Repeat([]byte{0x11}, vaultcrypto.KeySize)
	fixedVaultKey = bytes.Repeat([]byte{0x22}, vaultcrypto.KeySize)
)

// The IDs of the envelope vectors, derived from fixed seeds so that they are
// real base64url IDs of the right length rather than typed-out strings.
var (
	vecAccountID = func() string {
		root := vaultcrypto.RootKey(bytes.Repeat([]byte{0x33}, 32))
		return vaultcrypto.AccountID(vaultcrypto.IdentityKey(root).Public().(ed25519.PublicKey))
	}()
	vecVaultID = mustID(0x44)
	vecItemID  = mustID(0x55)
)

// mustID is a fixed 16-byte ID, spelled the way NewID spells one.
func mustID(b byte) string {
	id, err := vaultcrypto.NewID(bytes.NewReader(bytes.Repeat([]byte{b}, vaultcrypto.IDSize)))
	if err != nil {
		fatal(err)
	}
	return id
}

// weakArgon2 is the parameter set the Argon2id vectors use. DeriveKEK does
// not Check its parameters, so a vector may show a deliberately weak one: it
// documents the KDF, not the product's cost, and it keeps the file quick to
// regenerate. Nothing that opens a keyfile accepts it.
var weakArgon2 = vaultcrypto.Argon2Params{MKiB: 8, T: 1, P: 1}

// wrapArgon2 is what the wrapping vectors use, and it is DefaultArgon2: a
// keyfile is wrapped with the real cost, so the vectors carry it.
var wrapArgon2 = vaultcrypto.DefaultArgon2

type bip39Vec struct {
	Name           string `json:"name"`
	Entropy        string `json:"entropy"`
	Mnemonic       string `json:"mnemonic"`
	Seed           string `json:"seed"`
	SeedPassphrase string `json:"seedPassphrase"`
	Valid          bool   `json:"valid"`
}

type keyTreeVec struct {
	Name           string `json:"name"`
	Seed           string `json:"seed"`
	RootKey        string `json:"rootKey"`
	IdentitySeed   string `json:"identitySeed"`
	IdentityPublic string `json:"identityPublic"`
	AccountID      string `json:"accountID"`
	VaultID        string `json:"vaultID"`
	VaultKey       string `json:"vaultKey"`
}

type argon2Vec struct {
	Name       string `json:"name"`
	Passphrase string `json:"passphrase"`
	Salt       string `json:"salt"`
	MKiB       uint32 `json:"mKiB"`
	T          uint32 `json:"t"`
	P          uint8  `json:"p"`
	KEK        string `json:"kek"`
}

type wrapVec struct {
	Name       string `json:"name"`
	Passphrase string `json:"passphrase"`
	AccountID  string `json:"accountID"`
	MKiB       uint32 `json:"mKiB"`
	T          uint32 `json:"t"`
	P          uint8  `json:"p"`
	Salt       string `json:"salt"`
	Nonce      string `json:"nonce"`
	RootKey    string `json:"rootKey"`
	Ciphertext string `json:"ciphertext"`
}

type itemHeaderVec struct {
	AccountID string `json:"accountID"`
	VaultID   string `json:"vaultID"`
	ItemID    string `json:"itemID"`
	Rev       int64  `json:"rev"`
	UpdatedAt int64  `json:"updatedAt"`
	Deleted   bool   `json:"deleted"`
}

type itemVec struct {
	Name       string        `json:"name"`
	VaultKey   string        `json:"vaultKey"`
	Header     itemHeaderVec `json:"header"`
	AAD        string        `json:"aad"`
	Nonce      string        `json:"nonce"`
	Payload    string        `json:"payload"`
	Ciphertext string        `json:"ciphertext"`
}

type vaultMetaHeaderVec struct {
	AccountID string `json:"accountID"`
	VaultID   string `json:"vaultID"`
	Rev       int64  `json:"rev"`
	UpdatedAt int64  `json:"updatedAt"`
}

type vaultMetaVec struct {
	Name       string             `json:"name"`
	VaultKey   string             `json:"vaultKey"`
	Header     vaultMetaHeaderVec `json:"header"`
	AAD        string             `json:"aad"`
	Nonce      string             `json:"nonce"`
	Payload    string             `json:"payload"`
	Ciphertext string             `json:"ciphertext"`
}

type totpVec struct {
	Name      string `json:"name"`
	Secret    string `json:"secret"`
	Digits    int    `json:"digits"`
	Period    int    `json:"period"`
	Algorithm string `json:"algorithm"`
	At        int64  `json:"at"`
	Code      string `json:"code"`
	Remaining int64  `json:"remaining"`
}

type base32Vec struct {
	Name   string `json:"name"`
	Input  string `json:"input"`
	Secret string `json:"secret"`
	Valid  bool   `json:"valid"`
}

type genOptsVec struct {
	Length      int  `json:"length"`
	Lower       bool `json:"lower"`
	Upper       bool `json:"upper"`
	Digits      bool `json:"digits"`
	Symbols     bool `json:"symbols"`
	NoAmbiguous bool `json:"noAmbiguous"`
}

type generatorVec struct {
	Name        string     `json:"name"`
	Options     genOptsVec `json:"options"`
	Random      string     `json:"random"`
	Password    string     `json:"password"`
	EntropyBits float64    `json:"entropyBits"`
	Strength    int        `json:"strength"`
}

type vectorsDoc struct {
	Version   int            `json:"version"`
	BIP39     []bip39Vec     `json:"bip39"`
	KeyTree   []keyTreeVec   `json:"keyTree"`
	Argon2    []argon2Vec    `json:"argon2"`
	Wrap      []wrapVec      `json:"wrap"`
	Item      []itemVec      `json:"item"`
	VaultMeta []vaultMetaVec `json:"vaultMeta"`
	TOTP      []totpVec      `json:"totp"`
	Base32    []base32Vec    `json:"base32"`
	Generator []generatorVec `json:"generator"`
}

func b64(b []byte) string { return vaultcrypto.EncodeB64(b) }

// mustBip39Words reads the BIP39 English word list. The list is embedded in
// the vaultcrypto package, so it is reached through the mnemonic it produces:
// the vector that needs it starts from a known mnemonic and walks the list by
// deriving words from it.
func mustBip39Words() []string {
	// The mnemonic of zero entropy is the first 23 words of the list, which
	// gives the tool 23 words to try without reading a file.
	m, err := vaultcrypto.MnemonicFromEntropy(make([]byte, 32))
	if err != nil {
		fatal(err)
	}
	return strings.Fields(m)
}

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		fatal(err)
	}
	return b
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "vectors:", err)
	os.Exit(1)
}

func bip39Vectors() []bip39Vec {
	var out []bip39Vec
	for i, h := range entropyHex {
		entropy := mustHex(h)
		m, err := vaultcrypto.MnemonicFromEntropy(entropy)
		if err != nil {
			fatal(err)
		}
		name := fmt.Sprintf("entropy-%02d", i)
		out = append(out, bip39Vec{
			Name:           name,
			Entropy:        h,
			Mnemonic:       m,
			Seed:           b64(vaultcrypto.MnemonicSeed(m, "")),
			SeedPassphrase: b64(vaultcrypto.MnemonicSeed(m, name)),
			Valid:          true,
		})
	}
	// A mnemonic whose last word breaks the checksum is refused. The word is
	// swapped for the first one that actually fails, so the vector proves the
	// checksum is checked rather than depending on a lucky word.
	out = append(out, bip39Vec{
		Name:     "bad-checksum",
		Entropy:  entropyHex[0],
		Mnemonic: withBadChecksum(out[0].Mnemonic),
		Valid:    false,
	})
	// A short mnemonic is refused too.
	out = append(out, bip39Vec{
		Name:     "short",
		Entropy:  entropyHex[0],
		Mnemonic: "abandon abandon abandon",
		Valid:    false,
	})
	return out
}

// withBadChecksum changes the last word of a valid mnemonic to another word
// of the list that breaks its checksum. The result has to be a real word: a
// word outside the list fails for the wrong reason, and the vector would then
// prove nothing about the checksum.
func withBadChecksum(mnemonic string) string {
	words := strings.Fields(mnemonic)
	if len(words) == 0 {
		fatal(errors.New("an empty mnemonic has no last word"))
	}
	for _, candidate := range strings.Fields(bip39Words) {
		if candidate == words[len(words)-1] {
			continue
		}
		words[len(words)-1] = candidate
		try := strings.Join(words, " ")
		err := vaultcrypto.ValidateMnemonic(try)
		if err == nil {
			continue
		}
		// A word outside the list is the wrong failure.
		if strings.Contains(err.Error(), "not in the BIP39 English list") {
			continue
		}
		return try
	}
	fatal(errors.New("no word of the list broke the checksum"))
	return ""
}

func keyTreeVectors() []keyTreeVec {
	var out []keyTreeVec
	for i, h := range entropyHex {
		seed := mustHex(h)
		root := vaultcrypto.RootKey(seed)
		sk := vaultcrypto.IdentityKey(root)
		pub, _ := sk.Public().(ed25519.PublicKey)
		accountID := vaultcrypto.AccountID(pub)
		vaultID := vaultcrypto.DefaultVaultID(root)
		out = append(out, keyTreeVec{
			Name:           fmt.Sprintf("seed-%02d", i),
			Seed:           h,
			RootKey:        b64(root),
			IdentitySeed:   b64(sk.Seed()),
			IdentityPublic: b64(pub),
			AccountID:      accountID,
			VaultID:        vaultID,
			VaultKey:       b64(vaultcrypto.VaultKey(root, vaultID)),
		})
	}
	return out
}

func argon2Vectors() []argon2Vec {
	var out []argon2Vec
	for _, c := range []struct {
		name string
		pw   string
		p    vaultcrypto.Argon2Params
	}{
		{"weak", "correct horse battery staple", weakArgon2},
		{"empty-passphrase", "", weakArgon2},
		{"unicode-passphrase", "\u00e9\u00e8\u4e2d\u6587", vaultcrypto.Argon2Params{MKiB: 8, T: 2, P: 1}},
	} {
		kek := vaultcrypto.DeriveKEK(c.pw, fixedSalt, c.p)
		out = append(out, argon2Vec{
			Name:       c.name,
			Passphrase: c.pw,
			Salt:       b64(fixedSalt),
			MKiB:       c.p.MKiB,
			T:          c.p.T,
			P:          c.p.P,
			KEK:        b64(kek),
		})
	}
	return out
}

func wrapVectors() []wrapVec {
	var out []wrapVec
	for _, c := range []struct {
		name      string
		pw        string
		accountID string
	}{
		{"default-account", "correct horse battery staple", vecAccountID},
		{"empty-passphrase", "", vecAccountID},
		{"other-account", "correct horse battery staple", vecAccountID + "x"},
	} {
		w, err := vaultcrypto.WrapRoot(fixedRoot, c.pw, c.accountID, wrapArgon2,
			bytes.NewReader(append(append([]byte{}, fixedSalt...), fixedNonce...)))
		if err != nil {
			fatal(err)
		}
		out = append(out, wrapVec{
			Name:       c.name,
			Passphrase: c.pw,
			AccountID:  c.accountID,
			MKiB:       w.Params.MKiB,
			T:          w.Params.T,
			P:          w.Params.P,
			Salt:       b64(w.Salt),
			Nonce:      b64(w.Nonce),
			RootKey:    b64(fixedRoot),
			Ciphertext: b64(w.CT),
		})
	}
	return out
}

func itemVectors() []itemVec {
	var out []itemVec
	for _, c := range []struct {
		name    string
		payload string
		rev     int64
		updated int64
		deleted bool
	}{
		{"login", "{\"title\":\"Bank\",\"username\":\"me\"}", 1, 1730000000, false},
		{"empty", "", 1, 1730000000, false},
		{"tombstone", "", 7, 1730009999, true},
	} {
		h := vaultcrypto.ItemHeader{
			AccountID: vecAccountID,
			VaultID:   vecVaultID,
			ItemID:    vecItemID,
			Rev:       c.rev,
			UpdatedAt: c.updated,
			Deleted:   c.deleted,
		}
		nonce, ct, err := vaultcrypto.SealItem(fixedVaultKey, h, []byte(c.payload), bytes.NewReader(fixedNonce))
		if err != nil {
			fatal(err)
		}
		out = append(out, itemVec{
			Name:       c.name,
			VaultKey:   b64(fixedVaultKey),
			Header:     itemHeaderVec{AccountID: h.AccountID, VaultID: h.VaultID, ItemID: h.ItemID, Rev: h.Rev, UpdatedAt: h.UpdatedAt, Deleted: h.Deleted},
			AAD:        string(h.AAD()),
			Nonce:      b64(nonce),
			Payload:    b64([]byte(c.payload)),
			Ciphertext: b64(ct),
		})
	}
	return out
}

func vaultMetaVectors() []vaultMetaVec {
	var out []vaultMetaVec
	for _, c := range []struct {
		name    string
		payload string
		rev     int64
		updated int64
	}{
		{"default-vault", "{\"name\":\"Personal\"}", 1, 1730000000},
		{"empty", "", 1, 1730000000},
	} {
		h := vaultcrypto.VaultMetaHeader{
			AccountID: vecAccountID,
			VaultID:   vecVaultID,
			Rev:       c.rev,
			UpdatedAt: c.updated,
		}
		nonce, ct, err := vaultcrypto.SealVaultMeta(fixedVaultKey, h, []byte(c.payload), bytes.NewReader(fixedNonce))
		if err != nil {
			fatal(err)
		}
		out = append(out, vaultMetaVec{
			Name:       c.name,
			VaultKey:   b64(fixedVaultKey),
			Header:     vaultMetaHeaderVec{AccountID: h.AccountID, VaultID: h.VaultID, Rev: h.Rev, UpdatedAt: h.UpdatedAt},
			AAD:        string(h.AAD()),
			Nonce:      b64(nonce),
			Payload:    b64([]byte(c.payload)),
			Ciphertext: b64(ct),
		})
	}
	return out
}

func totpVectors() []totpVec {
	var out []totpVec
	for _, c := range []struct {
		name string
		seed string
		algo string
	}{
		{"sha1", seedSHA1, "SHA1"},
		{"sha256", seedSHA256, "SHA256"},
		{"sha512", seedSHA512, "SHA512"},
	} {
		for _, at := range []int64{59, 1111111109, 1111111111, 1234567890, 2000000000, 20000000000} {
			uri := "otpauth://totp/x?secret=" + b32(c.seed) +
				"&algorithm=" + c.algo + "&digits=8"
			o, err := vaultcrypto.ParseOTP(uri)
			if err != nil {
				fatal(err)
			}
			code, err := o.Code(time.Unix(at, 0).UTC())
			if err != nil {
				fatal(err)
			}
			out = append(out, totpVec{
				Name:      fmt.Sprintf("%s-t%d", c.name, at),
				Secret:    uri,
				Digits:    o.Digits,
				Period:    o.Period,
				Algorithm: o.Algorithm,
				At:        at,
				Code:      code,
				Remaining: int64(o.Remaining(time.Unix(at, 0).UTC()) / time.Second),
			})
		}
	}
	return out
}

// b32 is the canonical Base32 spelling of s: RFC 4648 base32 of its bytes.
func b32(s string) string {
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(s))
}

func base32Vectors() []base32Vec {
	var out []base32Vec
	for _, c := range []struct {
		name  string
		input string
		valid bool
	}{
		{"plain", b32(seedSHA1), true},
		{"spaced", spaced(b32(seedSHA1)), true},
		{"hyphenated", strings.ReplaceAll(spaced(b32(seedSHA1)), " ", "-"), true},
		{"lowercase", strings.ToLower(b32(seedSHA1)), true},
		{"padded", b32(seedSHA1) + "====", true},
		{"sha256-seed", b32(seedSHA256), true},
		{"sha512-seed", b32(seedSHA512), true},
		{"rfc4648-hello", b32(base32Hello), true},
		{"rfc4648-bytes", b32(base32Seq), true},
		{"non-canonical", nonCanonical(""), false},
		{"not-alphabet", "0O1lI", false},
		{"one-byte", "MY", false},
		{"over-long", strings.Repeat("A", 400), false},
		{"empty", "", false},
	} {
		v := base32Vec{Name: c.name, Input: c.input, Valid: c.valid}
		if c.valid {
			o, err := vaultcrypto.ParseOTP(c.input)
			if err != nil {
				fatal(fmt.Errorf("%s: %w", c.name, err))
			}
			v.Secret = o.Secret
		}
		out = append(out, v)
	}
	return out
}

// spaced inserts a space every four characters, the way authenticators print
// a secret.
func spaced(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i += 4 {
		if i > 0 {
			b.WriteByte(' ')
		}
		end := i + 4
		if end > len(s) {
			end = len(s)
		}
		b.WriteString(s[i:end])
	}
	return b.String()
}

// nonCanonical changes the last character of a Base32 string so that it
// carries bits belonging to no byte. The result still decodes, to different
// bytes, which is exactly the mistype the canonical round trip catches.
func nonCanonical(s string) string {
	// A 32-character Base32 string is 20 bytes with no spare bits, so every
	// spelling of it is canonical. The probe instead uses a string with spare
	// bits: the SHA256 seed is 32 bytes, whose Base32 form is 52 characters,
	// and the last character of a 52-character group carries 4 spare bits.
	probe := b32(seedSHA256)
	if len(probe) < 2 {
		fatal(errors.New("the probe is too short"))
	}
	for c := 'A'; c <= 'Z'; c++ {
		if c == rune(probe[len(probe)-1]) {
			continue
		}
		try := probe[:len(probe)-1] + string(c)
		decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(try)
		if err != nil {
			continue
		}
		if base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(decoded) != try {
			return try
		}
	}
	fatal(errors.New("no non-canonical spelling of the probe"))
	return ""
}

// genStream is a deterministic byte stream that produces the password of o in
// one draw: it starts at the first character of each enabled class and walks
// the alphabet, so every enabled class appears in the first four characters
// and every byte stays below the rejection limit of the widest alphabet.
func genStream(o vaultcrypto.GenOptions) []byte {
	var start []byte
	if o.Lower {
		start = append(start, 0)
	}
	if o.Upper {
		start = append(start, 26)
	}
	if o.Digits {
		start = append(start, 52)
	}
	if o.Symbols {
		start = append(start, 62)
	}
	if len(start) == 0 {
		fatal(errors.New("no character class is enabled"))
	}
	out := make([]byte, 0, o.Length+8)
	for i := 0; len(out) < o.Length; i++ {
		out = append(out, start[i%len(start)]+byte(i/len(start)))
	}
	// Room for retries, in case a drawn string misses a class.
	for i := 0; i < 8; i++ {
		out = append(out, start[i%len(start)])
	}
	return out
}

func generatorVectors() []generatorVec {
	var out []generatorVec
	for _, c := range []struct {
		name string
		o    vaultcrypto.GenOptions
	}{
		{"default", vaultcrypto.DefaultGenOptions()},
		{"no-ambiguous", vaultcrypto.GenOptions{Length: 24, Lower: true, Upper: true, Digits: true, Symbols: true, NoAmbiguous: true}},
		{"lower-only", vaultcrypto.GenOptions{Length: 16, Lower: true}},
		{"digits-only", vaultcrypto.GenOptions{Length: 12, Digits: true}},
		{"all-classes-long", vaultcrypto.GenOptions{Length: 128, Lower: true, Upper: true, Digits: true, Symbols: true}},
	} {
		// A stream that covers every class in turn and stays under the
		// rejection limit, with room for a retry if a drawn string misses an
		// enabled class.
		stream := genStream(c.o)
		pw, err := vaultcrypto.GeneratePassword(c.o, bytes.NewReader(stream))
		if err != nil {
			fatal(err)
		}
		out = append(out, generatorVec{
			Name: c.name,
			Options: genOptsVec{
				Length: c.o.Length, Lower: c.o.Lower, Upper: c.o.Upper,
				Digits: c.o.Digits, Symbols: c.o.Symbols, NoAmbiguous: c.o.NoAmbiguous,
			},
			Random:      hex.EncodeToString(stream),
			Password:    pw,
			EntropyBits: c.o.EntropyBits(),
			Strength:    vaultcrypto.StrengthLevel(c.o.EntropyBits()),
		})
	}
	return out
}

func build() []byte {
	doc := vectorsDoc{
		Version:   vectorsVersion,
		BIP39:     bip39Vectors(),
		KeyTree:   keyTreeVectors(),
		Argon2:    argon2Vectors(),
		Wrap:      wrapVectors(),
		Item:      itemVectors(),
		VaultMeta: vaultMetaVectors(),
		TOTP:      totpVectors(),
		Base32:    base32Vectors(),
		Generator: generatorVectors(),
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		fatal(err)
	}
	return buf.Bytes()
}

func main() {
	check := flag.Bool("check", false, "fail if the file differs from what this tool generates")
	flag.Parse()

	want := build()
	if !*check {
		if err := os.WriteFile(vectorsPath, want, 0o644); err != nil {
			fatal(err)
		}
		fmt.Printf("vectors: wrote %s (%d bytes)\n", vectorsPath, len(want))
		return
	}
	have, err := os.ReadFile(vectorsPath)
	if err != nil {
		fatal(err)
	}
	if !bytes.Equal(have, want) {
		fmt.Fprintln(os.Stderr, "vectors: "+vectorsPath+" differs from what this tool generates; run go run ./tools/vectors")
		os.Exit(1)
	}
	fmt.Println("vectors: ok")
}
