package vaultcrypto

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// vectorsFile is the byte-level contract every implementation of the format
// has to agree on. tools/vectors writes it; this test replays it.
const vectorsFile = "../../spec/crypto-vectors.json"

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

type bip39Vec struct {
	Name     string `json:"name"`
	Entropy  string `json:"entropy"`
	Mnemonic string `json:"mnemonic"`
	Seed     string `json:"seed"`
	SeedPF   string `json:"seedPassphrase"`
	Valid    bool   `json:"valid"`
}

type keyTreeVec struct {
	Name       string `json:"name"`
	Seed       string `json:"seed"`
	RootKey    string `json:"rootKey"`
	IdentitySk string `json:"identitySeed"`
	IdentityPk string `json:"identityPublic"`
	AccountID  string `json:"accountID"`
	VaultID    string `json:"vaultID"`
	VaultKey   string `json:"vaultKey"`
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

type itemVec struct {
	Name       string        `json:"name"`
	VaultKey   string        `json:"vaultKey"`
	Header     itemHeaderVec `json:"header"`
	AAD        string        `json:"aad"`
	Nonce      string        `json:"nonce"`
	Payload    string        `json:"payload"`
	Ciphertext string        `json:"ciphertext"`
}

type itemHeaderVec struct {
	AccountID string `json:"accountID"`
	VaultID   string `json:"vaultID"`
	ItemID    string `json:"itemID"`
	Rev       int64  `json:"rev"`
	UpdatedAt int64  `json:"updatedAt"`
	Deleted   bool   `json:"deleted"`
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

type vaultMetaHeaderVec struct {
	AccountID string `json:"accountID"`
	VaultID   string `json:"vaultID"`
	Rev       int64  `json:"rev"`
	UpdatedAt int64  `json:"updatedAt"`
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

type generatorVec struct {
	Name     string     `json:"name"`
	Options  genOptsVec `json:"options"`
	Random   string     `json:"random"`
	Password string     `json:"password"`
	Entropy  float64    `json:"entropyBits"`
	Strength int        `json:"strength"`
}

type genOptsVec struct {
	Length      int  `json:"length"`
	Lower       bool `json:"lower"`
	Upper       bool `json:"upper"`
	Digits      bool `json:"digits"`
	Symbols     bool `json:"symbols"`
	NoAmbiguous bool `json:"noAmbiguous"`
}

func loadVectors(t *testing.T) vectorsDoc {
	t.Helper()
	raw, err := os.ReadFile(vectorsFile)
	if err != nil {
		t.Fatalf("read %s: %v", vectorsFile, err)
	}
	var doc vectorsDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", vectorsFile, err)
	}
	return doc
}

// b64Of decodes a base64url field of the vectors file.
func b64Of(t *testing.T, s string) []byte {
	t.Helper()
	if s == "" {
		return nil
	}
	b, err := DecodeB64(s)
	if err != nil {
		t.Fatalf("base64url %q: %v", s, err)
	}
	return b
}

// hexOf decodes a hex field: the BIP39 entropy and the generator's bytes.
func hexOf(t *testing.T, s string) []byte {
	t.Helper()
	if s == "" {
		return nil
	}
	b, err := decodeHex(s)
	if err != nil {
		t.Fatalf("hex %q: %v", s, err)
	}
	return b
}

func decodeHex(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, errHex
	}
	out := make([]byte, len(s)/2)
	for i := range out {
		hi, ok1 := hexVal(s[2*i])
		lo, ok2 := hexVal(s[2*i+1])
		if !ok1 || !ok2 {
			return nil, errHex
		}
		out[i] = hi<<4 | lo
	}
	return out, nil
}

func hexVal(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

var errHex = errHexType{}

type errHexType struct{}

func (errHexType) Error() string { return "odd or invalid hex" }

func TestVectorsReplayBIP39(t *testing.T) {
	doc := loadVectors(t)
	if doc.Version != 1 {
		t.Fatalf("version = %d, want 1", doc.Version)
	}
	if len(doc.BIP39) == 0 {
		t.Fatal("no BIP39 vectors")
	}
	for _, v := range doc.BIP39 {
		if v.Valid {
			// The entropy produces exactly this mnemonic, and the mnemonic
			// validates and seeds.
			got, err := MnemonicFromEntropy(hexOf(t, v.Entropy))
			if err != nil {
				t.Errorf("%s: %v", v.Name, err)
				continue
			}
			if got != v.Mnemonic {
				t.Errorf("%s: mnemonic = %q, want %q", v.Name, got, v.Mnemonic)
			}
			if err := ValidateMnemonic(v.Mnemonic); err != nil {
				t.Errorf("%s: ValidateMnemonic: %v", v.Name, err)
			}
			if EncodeB64(MnemonicSeed(v.Mnemonic, "")) != v.Seed {
				t.Errorf("%s: seed = %s, want %s", v.Name, EncodeB64(MnemonicSeed(v.Mnemonic, "")), v.Seed)
			}
			if EncodeB64(MnemonicSeed(v.Mnemonic, v.Name)) != v.SeedPF {
				t.Errorf("%s: seed with passphrase = %s, want %s", v.Name, EncodeB64(MnemonicSeed(v.Mnemonic, v.Name)), v.SeedPF)
			}
			continue
		}
		// An invalid vector carries a mnemonic that has to be refused. Its
		// entropy is still valid, so the check is on the words, not the
		// entropy: a mnemonic with a broken checksum, or the wrong length.
		if err := ValidateMnemonic(v.Mnemonic); err == nil {
			t.Errorf("%s: ValidateMnemonic accepted %q", v.Name, v.Mnemonic)
		}
	}
}

func TestVectorsReplayKeyTree(t *testing.T) {
	for _, v := range loadVectors(t).KeyTree {
		seed := hexOf(t, v.Seed)
		root := RootKey(seed)
		if EncodeB64(root) != v.RootKey {
			t.Errorf("%s: root = %s, want %s", v.Name, EncodeB64(root), v.RootKey)
		}
		sk := IdentityKey(root)
		pub, ok := sk.Public().(ed25519.PublicKey)
		if !ok {
			t.Fatalf("%s: identity key has no public half", v.Name)
		}
		if EncodeB64(sk.Seed()) != v.IdentitySk {
			t.Errorf("%s: identity seed = %s, want %s", v.Name, EncodeB64(sk.Seed()), v.IdentitySk)
		}
		if EncodeB64(pub) != v.IdentityPk {
			t.Errorf("%s: identity public = %s, want %s", v.Name, EncodeB64(pub), v.IdentityPk)
		}
		if id := AccountID(pub); id != v.AccountID {
			t.Errorf("%s: account id = %s, want %s", v.Name, id, v.AccountID)
		}
		vaultID := DefaultVaultID(root)
		if vaultID != v.VaultID {
			t.Errorf("%s: vault id = %s, want %s", v.Name, vaultID, v.VaultID)
		}
		vk := VaultKey(root, v.VaultID)
		if EncodeB64(vk) != v.VaultKey {
			t.Errorf("%s: vault key = %s, want %s", v.Name, EncodeB64(vk), v.VaultKey)
		}
	}
}

func TestVectorsReplayArgon2(t *testing.T) {
	for _, v := range loadVectors(t).Argon2 {
		p := Argon2Params{MKiB: v.MKiB, T: v.T, P: v.P}
		kek := DeriveKEK(v.Passphrase, b64Of(t, v.Salt), p)
		if EncodeB64(kek) != v.KEK {
			t.Errorf("%s: kek = %s, want %s", v.Name, EncodeB64(kek), v.KEK)
		}
	}
}

func TestVectorsReplayWrap(t *testing.T) {
	for _, v := range loadVectors(t).Wrap {
		root := b64Of(t, v.RootKey)
		p := Argon2Params{MKiB: v.MKiB, T: v.T, P: v.P}
		// The nonce is fixed by the vector, so the wrapped key is reproducible.
		r := bytes.NewReader(append(b64Of(t, v.Salt), b64Of(t, v.Nonce)...))
		w, err := WrapRoot(root, v.Passphrase, v.AccountID, p, r)
		if err != nil {
			t.Errorf("%s: WrapRoot: %v", v.Name, err)
			continue
		}
		if EncodeB64(w.Nonce) != v.Nonce {
			t.Errorf("%s: nonce = %s, want %s", v.Name, EncodeB64(w.Nonce), v.Nonce)
		}
		if EncodeB64(w.Salt) != v.Salt {
			t.Errorf("%s: salt = %s, want %s", v.Name, EncodeB64(w.Salt), v.Salt)
		}
		if EncodeB64(w.CT) != v.Ciphertext {
			t.Errorf("%s: wrapped = %s, want %s", v.Name, EncodeB64(w.CT), v.Ciphertext)
		}
		back, err := UnwrapRoot(w, v.Passphrase, v.AccountID)
		if err != nil {
			t.Errorf("%s: UnwrapRoot: %v", v.Name, err)
			continue
		}
		if !bytes.Equal(back, root) {
			t.Errorf("%s: unwrapped %s, want %s", v.Name, EncodeB64(back), v.RootKey)
		}
		// A wrong passphrase is ErrAuth, never a different root key.
		if _, err := UnwrapRoot(w, v.Passphrase+"x", v.AccountID); err != ErrAuth {
			t.Errorf("%s: wrong passphrase: err = %v", v.Name, err)
		}
		// Another account's ID is a different AAD, so the same passphrase fails.
		if _, err := UnwrapRoot(w, v.Passphrase, v.AccountID+"x"); err != ErrAuth {
			t.Errorf("%s: wrong account id: err = %v", v.Name, err)
		}
	}
}

func TestVectorsReplayItem(t *testing.T) {
	for _, v := range loadVectors(t).Item {
		vk := b64Of(t, v.VaultKey)
		h := ItemHeader{
			AccountID: v.Header.AccountID,
			VaultID:   v.Header.VaultID,
			ItemID:    v.Header.ItemID,
			Rev:       v.Header.Rev,
			UpdatedAt: v.Header.UpdatedAt,
			Deleted:   v.Header.Deleted,
		}
		if got := string(h.AAD()); got != v.AAD {
			t.Errorf("%s: aad = %q, want %q", v.Name, got, v.AAD)
		}
		payload := b64Of(t, v.Payload)
		nonce, ct, err := SealItem(vk, h, payload, bytes.NewReader(b64Of(t, v.Nonce)))
		if err != nil {
			t.Errorf("%s: SealItem: %v", v.Name, err)
			continue
		}
		if EncodeB64(nonce) != v.Nonce || EncodeB64(ct) != v.Ciphertext {
			t.Errorf("%s: sealed %s/%s, want %s/%s", v.Name, EncodeB64(nonce), EncodeB64(ct), v.Nonce, v.Ciphertext)
		}
		back, err := OpenItem(vk, h, nonce, ct)
		if err != nil {
			t.Errorf("%s: OpenItem: %v", v.Name, err)
			continue
		}
		if !bytes.Equal(back, payload) {
			t.Errorf("%s: opened %s, want %s", v.Name, EncodeB64(back), v.Payload)
		}
		// Every field of the header is bound: changing any of them fails.
		for _, bad := range []ItemHeader{
			{AccountID: v.Header.AccountID + "x", VaultID: v.Header.VaultID, ItemID: v.Header.ItemID, Rev: v.Header.Rev, UpdatedAt: v.Header.UpdatedAt, Deleted: v.Header.Deleted},
			{AccountID: v.Header.AccountID, VaultID: v.Header.VaultID + "x", ItemID: v.Header.ItemID, Rev: v.Header.Rev, UpdatedAt: v.Header.UpdatedAt, Deleted: v.Header.Deleted},
			{AccountID: v.Header.AccountID, VaultID: v.Header.VaultID, ItemID: v.Header.ItemID + "x", Rev: v.Header.Rev, UpdatedAt: v.Header.UpdatedAt, Deleted: v.Header.Deleted},
			{AccountID: v.Header.AccountID, VaultID: v.Header.VaultID, ItemID: v.Header.ItemID, Rev: v.Header.Rev + 1, UpdatedAt: v.Header.UpdatedAt, Deleted: v.Header.Deleted},
			{AccountID: v.Header.AccountID, VaultID: v.Header.VaultID, ItemID: v.Header.ItemID, Rev: v.Header.Rev, UpdatedAt: v.Header.UpdatedAt + 1, Deleted: v.Header.Deleted},
			{AccountID: v.Header.AccountID, VaultID: v.Header.VaultID, ItemID: v.Header.ItemID, Rev: v.Header.Rev, UpdatedAt: v.Header.UpdatedAt, Deleted: !v.Header.Deleted},
		} {
			if _, err := OpenItem(vk, bad, nonce, ct); err != ErrAuth {
				t.Errorf("%s: altered header opened: %v", v.Name, err)
			}
		}
	}
}

func TestVectorsReplayVaultMeta(t *testing.T) {
	for _, v := range loadVectors(t).VaultMeta {
		vk := b64Of(t, v.VaultKey)
		h := VaultMetaHeader{
			AccountID: v.Header.AccountID,
			VaultID:   v.Header.VaultID,
			Rev:       v.Header.Rev,
			UpdatedAt: v.Header.UpdatedAt,
		}
		if got := string(h.AAD()); got != v.AAD {
			t.Errorf("%s: aad = %q, want %q", v.Name, got, v.AAD)
		}
		payload := b64Of(t, v.Payload)
		nonce, ct, err := SealVaultMeta(vk, h, payload, bytes.NewReader(b64Of(t, v.Nonce)))
		if err != nil {
			t.Errorf("%s: SealVaultMeta: %v", v.Name, err)
			continue
		}
		if EncodeB64(nonce) != v.Nonce || EncodeB64(ct) != v.Ciphertext {
			t.Errorf("%s: sealed %s/%s, want %s/%s", v.Name, EncodeB64(nonce), EncodeB64(ct), v.Nonce, v.Ciphertext)
		}
		back, err := OpenVaultMeta(vk, h, nonce, ct)
		if err != nil {
			t.Errorf("%s: OpenVaultMeta: %v", v.Name, err)
			continue
		}
		if !bytes.Equal(back, payload) {
			t.Errorf("%s: opened %s, want %s", v.Name, EncodeB64(back), v.Payload)
		}
	}
}

func TestVectorsReplayTOTP(t *testing.T) {
	for _, v := range loadVectors(t).TOTP {
		// The vector spells the secret as the otpauth URI the tool derived it
		// from, so the parameters are read back from the URI, not the bare
		// secret.
		o, err := ParseOTP(v.Secret)
		if err != nil {
			t.Errorf("%s: ParseOTP: %v", v.Name, err)
			continue
		}
		if o.Digits != v.Digits || o.Period != v.Period || o.Algorithm != v.Algorithm {
			t.Errorf("%s: %d/%d/%s, want %d/%d/%s", v.Name, o.Digits, o.Period, o.Algorithm, v.Digits, v.Period, v.Algorithm)
		}
		at := time.Unix(v.At, 0).UTC()
		got, err := o.Code(at)
		if err != nil {
			t.Errorf("%s: Code: %v", v.Name, err)
			continue
		}
		if got != v.Code {
			t.Errorf("%s: code = %s, want %s", v.Name, got, v.Code)
		}
		// The vector holds whole seconds; Remaining is exact to the
		// nanosecond, so a second inside the step rounds up.
		if d := o.Remaining(at); d/time.Second != time.Duration(v.Remaining) {
			t.Errorf("%s: remaining = %v, want %ds", v.Name, d, v.Remaining)
		}
	}
}

func TestVectorsReplayBase32(t *testing.T) {
	for _, v := range loadVectors(t).Base32 {
		o, err := ParseOTP(v.Input)
		if v.Valid {
			if err != nil {
				t.Errorf("%s: %v", v.Name, err)
				continue
			}
			if o.Secret != v.Secret {
				t.Errorf("%s: secret = %q, want %q", v.Name, o.Secret, v.Secret)
			}
		} else if err == nil {
			t.Errorf("%s: accepted as %q", v.Name, o.Secret)
		}
	}
}

func TestVectorsReplayGenerator(t *testing.T) {
	for _, v := range loadVectors(t).Generator {
		o := GenOptions{
			Length:      v.Options.Length,
			Lower:       v.Options.Lower,
			Upper:       v.Options.Upper,
			Digits:      v.Options.Digits,
			Symbols:     v.Options.Symbols,
			NoAmbiguous: v.Options.NoAmbiguous,
		}
		pw, err := GeneratePassword(o, bytes.NewReader(hexOf(t, v.Random)))
		if err != nil {
			t.Errorf("%s: %v", v.Name, err)
			continue
		}
		if pw != v.Password {
			t.Errorf("%s: password = %q, want %q", v.Name, pw, v.Password)
		}
		if bits := o.EntropyBits(); bits != v.Entropy {
			t.Errorf("%s: entropy = %v, want %v", v.Name, bits, v.Entropy)
		}
		if got := StrengthLevel(v.Entropy); got != v.Strength {
			t.Errorf("%s: strength = %d, want %d", v.Name, got, v.Strength)
		}
	}
}

// The vectors file is a contract, not a snapshot: every one of its sections
// has to be present, or the replay above is silently skipping work.
func TestVectorsFileCoversEverySection(t *testing.T) {
	doc := loadVectors(t)
	for _, c := range []struct {
		name string
		n    int
	}{
		{"bip39", len(doc.BIP39)},
		{"keyTree", len(doc.KeyTree)},
		{"argon2", len(doc.Argon2)},
		{"wrap", len(doc.Wrap)},
		{"item", len(doc.Item)},
		{"vaultMeta", len(doc.VaultMeta)},
		{"totp", len(doc.TOTP)},
		{"base32", len(doc.Base32)},
		{"generator", len(doc.Generator)},
	} {
		if c.n == 0 {
			t.Errorf("section %s is empty", c.name)
		}
	}
}
