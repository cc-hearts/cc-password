package vaultcrypto

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"testing"
)

func TestSealMatchesXChaChaDraftVector(t *testing.T) {
	// draft-irtf-cfrg-xchacha-03, appendix A.3.1.
	key := seqBytes(0x80, 32)
	nonce := seqBytes(0x40, 24)
	aad := unhex(t, "50515253c0c1c2c3c4c5c6c7")
	pt := []byte("Ladies and Gentlemen of the class of '99: If I could offer you only one tip for the future, sunscreen would be it.")
	const want = "bd6d179d3e83d43b9576579493c0e939572a1700252bfaccbed2902c21396cbb731c7f1b0b4aa6440bf3a82f4eda7e39ae64c6708c54c216cb96b72e1213b4522f8c9ba40db5d945b11b69b982c1bb9e3f3fac2bc369488f76b2383565d3fff921f9664c97637da9768812f615c68b13b52ec0875924c1c7987947deafd8780acf49"

	gotNonce, ct, err := Seal(key, pt, aad, bytes.NewReader(nonce))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotNonce, nonce) {
		t.Errorf("nonce %x, want %x", gotNonce, nonce)
	}
	if got := hex.EncodeToString(ct); got != want {
		t.Fatalf("ciphertext and tag\n got %s\nwant %s", got, want)
	}
	back, err := Open(key, nonce, ct, aad)
	if err != nil || !bytes.Equal(back, pt) {
		t.Fatalf("Open = %q, %v", back, err)
	}
}

func TestSealDrawsAFreshNonceEachTime(t *testing.T) {
	key := seqBytes(0x80, 32)
	n1, c1, err1 := Seal(key, []byte("same"), nil, rand.Reader)
	n2, c2, err2 := Seal(key, []byte("same"), nil, rand.Reader)
	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	if bytes.Equal(n1, n2) || bytes.Equal(c1, c2) {
		t.Fatal("two seals of the same plaintext share a nonce or a ciphertext")
	}
}

func TestSealFailsOnShortRandomness(t *testing.T) {
	if _, _, err := Seal(seqBytes(0, 32), []byte("x"), nil, bytes.NewReader(make([]byte, 23))); err == nil {
		t.Fatal("23 bytes of randomness made a nonce")
	}
}

func TestOpenFailsWithTheOneAuthError(t *testing.T) {
	key := seqBytes(0x80, 32)
	aad := []byte("aad")
	nonce, ct, err := Seal(key, []byte("secret"), aad, bytes.NewReader(seqBytes(0, 24)))
	if err != nil {
		t.Fatal(err)
	}
	flip := func(b []byte, i int) []byte {
		c := bytes.Clone(b)
		c[i] ^= 1
		return c
	}
	cases := []struct {
		name                string
		key, nonce, ct, aad []byte
	}{
		{"other key", seqBytes(0x81, 32), nonce, ct, aad},
		{"other aad", key, nonce, ct, []byte("aae")},
		{"no aad", key, nonce, ct, nil},
		{"flipped ciphertext bit", key, nonce, flip(ct, 0), aad},
		{"flipped tag bit", key, nonce, flip(ct, len(ct)-1), aad},
		{"flipped nonce bit", key, flip(nonce, 23), ct, aad},
		{"truncated", key, nonce, ct[:len(ct)-1], aad},
		{"short nonce", key, nonce[:12], ct, aad},
		{"no ciphertext", key, nonce, nil, aad},
		{"short key", key[:16], nonce, ct, aad},
	}
	for _, tc := range cases {
		pt, err := Open(tc.key, tc.nonce, tc.ct, tc.aad)
		if err != ErrAuth || pt != nil {
			t.Errorf("%s: Open = %q, %v; want nil, ErrAuth", tc.name, pt, err)
		}
	}
}
