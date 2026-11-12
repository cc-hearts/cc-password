package vaultcrypto

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

func TestDeriveKEKMatchesArgon2idReferenceVector(t *testing.T) {
	// The Argon2id vector of the reference implementation, version 0x13:
	// "password", salt "somesalt", t=2, m=64 MiB, p=1, 32 bytes.
	got := DeriveKEK("password", []byte("somesalt"), Argon2Params{MKiB: 65536, T: 2, P: 1})
	if want := "09316115d5cf24ed5a15a31a3ba326e5cf32edc24702987c02b6566f61913cf7"; hex.EncodeToString(got) != want {
		t.Fatalf("Argon2id %x, want %s", got, want)
	}
}

func TestDeriveKEKNormalizesThePassphrase(t *testing.T) {
	p := Argon2Params{MKiB: 64, T: 1, P: 1} // small: this test is about the passphrase
	salt := seqBytes(0, 16)
	same := [][2]string{
		{"caf\U000000E9", "cafe\U00000301"},                  // NFC and NFD
		{"\U0000FF50\U0000FF41\U0000FF53\U0000FF53", "pass"}, // full-width letters
	}
	for _, pr := range same {
		if !bytes.Equal(DeriveKEK(pr[0], salt, p), DeriveKEK(pr[1], salt, p)) {
			t.Errorf("%q and %q derive different keys", pr[0], pr[1])
		}
	}
	if bytes.Equal(DeriveKEK("pass", salt, p), DeriveKEK("Pass", salt, p)) {
		t.Error("the passphrase lost its case")
	}
}

func TestArgon2ParamsCheck(t *testing.T) {
	for _, p := range []Argon2Params{DefaultArgon2, {MKiB: 131072, T: 4, P: 4}, {MKiB: 4 << 20, T: 64, P: 16}} {
		if err := p.Check(); err != nil {
			t.Errorf("%+v: %v", p, err)
		}
	}
	for _, p := range []Argon2Params{
		{MKiB: 65535, T: 3, P: 1},
		{MKiB: 65536, T: 2, P: 1},
		{MKiB: 65536, T: 3, P: 0},
		{MKiB: 4<<20 + 1, T: 3, P: 1},
		{MKiB: 65536, T: 65, P: 1},
		{MKiB: 65536, T: 3, P: 17},
	} {
		if err := p.Check(); !errors.Is(err, ErrArgon2Params) {
			t.Errorf("%+v: got %v, want ErrArgon2Params", p, err)
		}
	}
}

const wrapPassphrase = "correct horse battery staple"

func TestWrapRootMatchesIndependentVector(t *testing.T) {
	// Computed independently with Node's crypto module: OpenSSL's Argon2id
	// and ChaCha20-Poly1305, HChaCha20 written out by hand. The salt is
	// bytes 00..0f and the nonce 10..27, read from the random source in
	// that order.
	root := unhex(t, zeroRoot)
	w, err := WrapRoot(root, wrapPassphrase, zeroAccount, DefaultArgon2, bytes.NewReader(seqBytes(0, 40)))
	if err != nil {
		t.Fatal(err)
	}
	if w.Params != (Argon2Params{MKiB: 65536, T: 3, P: 1}) {
		t.Errorf("params %+v", w.Params)
	}
	if !bytes.Equal(w.Salt, seqBytes(0, 16)) || !bytes.Equal(w.Nonce, seqBytes(16, 24)) {
		t.Errorf("salt %x nonce %x", w.Salt, w.Nonce)
	}
	const want = "8ed98665d95af066bf50ccf4e8833a190ed6c18ef989c13f0278c5f0cce7494a637e7a25b9961584a89983061f9fd728"
	if got := hex.EncodeToString(w.CT); got != want {
		t.Fatalf("wrapped root\n got %s\nwant %s", got, want)
	}
	got, err := UnwrapRoot(w, wrapPassphrase, zeroAccount)
	if err != nil || !bytes.Equal(got, root) {
		t.Fatalf("UnwrapRoot = %x, %v", got, err)
	}
}

func TestUnwrapRootFailsWithTheOneAuthError(t *testing.T) {
	root := unhex(t, zeroRoot)
	w, err := WrapRoot(root, wrapPassphrase, zeroAccount, DefaultArgon2, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	with := func(f func(*Wrapped)) Wrapped {
		c := Wrapped{Params: w.Params, Salt: bytes.Clone(w.Salt), Nonce: bytes.Clone(w.Nonce), CT: bytes.Clone(w.CT)}
		f(&c)
		return c
	}
	cases := []struct {
		name          string
		w             Wrapped
		pass, account string
	}{
		{"wrong passphrase", w, wrapPassphrase + "s", zeroAccount},
		{"flipped ciphertext bit", with(func(c *Wrapped) { c.CT[5] ^= 0x10 }), wrapPassphrase, zeroAccount},
		{"other account", w, wrapPassphrase, strings.Repeat("A", 43)},
		{"other nonce", with(func(c *Wrapped) { c.Nonce[0] ^= 1 }), wrapPassphrase, zeroAccount},
		{"other salt", with(func(c *Wrapped) { c.Salt[15] ^= 1 }), wrapPassphrase, zeroAccount},
		{"short salt", with(func(c *Wrapped) { c.Salt = c.Salt[:8] }), wrapPassphrase, zeroAccount},
		{"stronger parameters than it was wrapped with", with(func(c *Wrapped) { c.Params.T = 4 }), wrapPassphrase, zeroAccount},
	}
	for _, tc := range cases {
		got, err := UnwrapRoot(tc.w, tc.pass, tc.account)
		if err != ErrAuth || got != nil {
			t.Errorf("%s: UnwrapRoot = %x, %v; want nil, ErrAuth", tc.name, got, err)
		}
	}
}

func TestUnwrapRootChecksParametersBeforeArgon2(t *testing.T) {
	// 2 TiB: Argon2 run with it would not come back.
	w := Wrapped{Params: Argon2Params{MKiB: 1 << 31, T: 3, P: 1}, Salt: seqBytes(0, 16), Nonce: seqBytes(0, 24), CT: make([]byte, 48)}
	if _, err := UnwrapRoot(w, wrapPassphrase, zeroAccount); !errors.Is(err, ErrArgon2Params) {
		t.Errorf("huge parameters: %v, want ErrArgon2Params", err)
	}
	w.Params = Argon2Params{MKiB: 65536, T: 1, P: 1}
	if _, err := UnwrapRoot(w, wrapPassphrase, zeroAccount); !errors.Is(err, ErrArgon2Params) {
		t.Errorf("weak parameters: %v, want ErrArgon2Params", err)
	}
}

func TestWrapRootRefusals(t *testing.T) {
	root := unhex(t, zeroRoot)
	if _, err := WrapRoot(root, wrapPassphrase, zeroAccount, Argon2Params{MKiB: 65536, T: 1, P: 1}, rand.Reader); !errors.Is(err, ErrArgon2Params) {
		t.Errorf("weak parameters: %v, want ErrArgon2Params", err)
	}
	if _, err := WrapRoot(root[:16], wrapPassphrase, zeroAccount, DefaultArgon2, rand.Reader); err == nil {
		t.Error("wrapped a 16-byte root")
	}
	if _, err := WrapRoot(root, wrapPassphrase, zeroAccount, DefaultArgon2, bytes.NewReader(make([]byte, 39))); err == nil {
		t.Error("39 bytes of randomness made a salt and a nonce")
	}
}
