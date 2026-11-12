package vaultcrypto

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"strings"
	"testing"
)

func unhex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// seqBytes returns n bytes counting up from first: seqBytes(0x40, 3) is
// 40 41 42, as the test vectors of the RFCs spell their inputs.
func seqBytes(first byte, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = first + byte(i)
	}
	return b
}

func TestHKDFMatchesRFC5869(t *testing.T) {
	cases := []struct {
		name            string
		ikm, salt, info []byte
		length          int
		okm             string
	}{
		{"A.1", bytes.Repeat([]byte{0x0b}, 22), seqBytes(0x00, 13), seqBytes(0xf0, 10), 42,
			"3cb25f25faacd57a90434f64d0362f2a2d2d0a90cf1a5a4c5db02d56ecc4c5bf34007208d5b887185865"},
		{"A.2", seqBytes(0x00, 80), seqBytes(0x60, 80), seqBytes(0xb0, 80), 82,
			"b11e398dc80327a1c8e7f78c596a49344f012eda2d4efad8a050cc4c19afa97c59045a99cac7827271cb41c65e590e09da3275600c2f09b8367793a9aca3db71cc30c58179ec3e87c14c01d5c1f3434f1d87"},
		// A.3 has no salt: RFC 5869's default, as many zeros as the hash
		// is long, which every subkey of the key tree uses.
		{"A.3", bytes.Repeat([]byte{0x0b}, 22), nil, nil, 42,
			"8da4e775a563c18f715f802a063c5a31b8a11f5c5ee1879ec3454e5f3c738d2d9d201395faa4b61a96c8"},
	}
	for _, tc := range cases {
		got := hex.EncodeToString(hkdfSHA256(tc.ikm, tc.salt, string(tc.info), tc.length))
		if got != tc.okm {
			t.Errorf("%s: OKM %s, want %s", tc.name, got, tc.okm)
		}
	}
}

// The key tree of the mnemonic of zero entropy, "abandon" x23 "art", with
// no passphrase, computed independently with Node's crypto module
// (OpenSSL's PBKDF2, HKDF and Ed25519).
const (
	zeroSeed       = "408b285c123836004f4b8842c89324c1f01382450c0d439af345ba7fc49acf705489c6fc77dbd4e3dc1dd8cc6bc9f043db8ada1e243c4a0eafb290d399480840"
	zeroRoot       = "c5e0234571788988a7902b9cc125bafccc4d13cb7c9479b36f8e78b4125d18fb"
	zeroIdentity   = "20e6cebcb6549778c9780f04d1e1a67193a5636841d107cc919db4770bdee6b3"
	zeroPub        = "f041014fc127c6a7baac689e72f4258ae3a732aa718ec78f546b6ee9fc6e1ed7"
	zeroAccount    = "8EEBT8Enxqe6rGiecvQliuOnMqpxjsePVGtu6fxuHtc"
	zeroVaultID    = "s6WU001xGMDVUu6lu5mSNA"
	zeroVaultKey   = "7bb1317fcbb2bf95dbaa826664fbe0267bf89f3842f743b2322e664b2ba33d83"
	zeroMnemonicTx = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art"
)

func TestRootKeyAndSubkeysOfTheZeroMnemonic(t *testing.T) {
	seed := MnemonicSeed(zeroMnemonicTx, "")
	if got := hex.EncodeToString(seed); got != zeroSeed {
		t.Fatalf("seed %s, want %s", got, zeroSeed)
	}
	root := RootKey(seed)
	if got := hex.EncodeToString(root); got != zeroRoot {
		t.Fatalf("root %s, want %s", got, zeroRoot)
	}
	priv := IdentityKey(root)
	if got := hex.EncodeToString(priv.Seed()); got != zeroIdentity {
		t.Errorf("identity seed %s, want %s", got, zeroIdentity)
	}
	pub := priv.Public().(ed25519.PublicKey)
	if got := hex.EncodeToString(pub); got != zeroPub {
		t.Errorf("identity public key %s, want %s", got, zeroPub)
	}
	if got := AccountID(pub); got != zeroAccount {
		t.Errorf("account id %s, want %s", got, zeroAccount)
	}
	if got := DefaultVaultID(root); got != zeroVaultID {
		t.Errorf("default vault id %s, want %s", got, zeroVaultID)
	}
	if got := hex.EncodeToString(VaultKey(root, zeroVaultID)); got != zeroVaultKey {
		t.Errorf("default vault key %s, want %s", got, zeroVaultKey)
	}
	const other = "86b5950aed5c97ab7931080e023db5ce1929f2121a35e6d68c616c7f20b78cf0"
	if got := hex.EncodeToString(VaultKey(root, "AAAAAAAAAAAAAAAAAAAAAA")); got != other {
		t.Errorf("key of vault AAAA… %s, want %s", got, other)
	}
}

func TestNewIDIsSixteenBytesOfBase64URL(t *testing.T) {
	id, err := NewID(bytes.NewReader(seqBytes(0, 16)))
	if err != nil {
		t.Fatal(err)
	}
	if id != "AAECAwQFBgcICQoLDA0ODw" {
		t.Errorf("NewID = %q, want AAECAwQFBgcICQoLDA0ODw", id)
	}
	if _, err := NewID(bytes.NewReader(make([]byte, 15))); err == nil {
		t.Error("15 bytes of randomness made an id")
	}
}

func TestDecodeB64AcceptsOnlyCanonicalBase64URL(t *testing.T) {
	got, err := DecodeB64("AAECAwQFBgcICQoLDA0ODw")
	if err != nil || !bytes.Equal(got, seqBytes(0, 16)) {
		t.Fatalf("DecodeB64 = %x, %v", got, err)
	}
	got, err = DecodeB64(strings.Repeat("_", 21) + "w")
	if err != nil || !bytes.Equal(got, bytes.Repeat([]byte{0xff}, 16)) {
		t.Fatalf("DecodeB64(___…w) = %x, %v", got, err)
	}
	if got := EncodeB64(bytes.Repeat([]byte{0xff}, 16)); got != strings.Repeat("_", 21)+"w" {
		t.Errorf("EncodeB64(ff…) = %q", got)
	}
	for _, bad := range []string{
		"AAECAwQFBgcICQoLDA0ODw==", // padded
		"AAECAwQFBgcICQoLDA0ODx",   // stray bits after the last byte
		"AAECAwQFBgcICQoLDA0OD+",   // the standard alphabet
		"AAECAwQFBgcI\nCQoLDA0ODw", // a line break
		"AAECAwQFBgcICQoLDA0OD",    // an impossible length
		" AAECAwQFBgcICQoLDA0ODw",  // a space
	} {
		if _, err := DecodeB64(bad); err == nil {
			t.Errorf("DecodeB64(%q) accepted", bad)
		}
	}
}
