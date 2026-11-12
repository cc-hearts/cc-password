package vaultcrypto

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"
)

const (
	envAccount = zeroAccount
	envVault   = zeroVaultID
	envItem    = "AAECAwQFBgcICQoLDA0ODw"
)

func envHeader() ItemHeader {
	return ItemHeader{AccountID: envAccount, VaultID: envVault, ItemID: envItem, Rev: 7, UpdatedAt: 1762934400}
}

func TestItemHeaderAAD(t *testing.T) {
	h := envHeader()
	const want = "ccpass/v1/item|8EEBT8Enxqe6rGiecvQliuOnMqpxjsePVGtu6fxuHtc|s6WU001xGMDVUu6lu5mSNA|AAECAwQFBgcICQoLDA0ODw|7|1762934400|0"
	if got := string(h.AAD()); got != want {
		t.Errorf("AAD\n got %s\nwant %s", got, want)
	}
	h.Deleted = true
	if got := string(h.AAD()); got != want[:len(want)-1]+"1" {
		t.Errorf("AAD of a deleted item %s", got)
	}
}

func TestVaultMetaHeaderAAD(t *testing.T) {
	h := VaultMetaHeader{AccountID: envAccount, VaultID: envVault, Rev: 1, UpdatedAt: 1762934400}
	const want = "ccpass/v1/vault-meta|8EEBT8Enxqe6rGiecvQliuOnMqpxjsePVGtu6fxuHtc|s6WU001xGMDVUu6lu5mSNA|1|1762934400"
	if got := string(h.AAD()); got != want {
		t.Errorf("AAD\n got %s\nwant %s", got, want)
	}
}

func TestSealItemMatchesIndependentVector(t *testing.T) {
	// Computed with Node's crypto module and a hand-written HChaCha20,
	// the nonce bytes 20..37 read from the random source.
	key := unhex(t, zeroVaultKey)
	payload := []byte(`{"schema":1,"type":"login","title":"Example"}`)
	nonce, ct, err := SealItem(key, envHeader(), payload, bytes.NewReader(seqBytes(0x20, 24)))
	if err != nil {
		t.Fatal(err)
	}
	if got := EncodeB64(nonce); got != "ICEiIyQlJicoKSorLC0uLzAxMjM0NTY3" {
		t.Errorf("nonce %s", got)
	}
	if got := EncodeB64(ct); got != "o5Ga1REbCj5UmvRH3gttJL67Uq5ZImRPnpaIMwcMCKS_inpxbz0HE_9rh1Ai2JVLKZd0vJSU6ab_i7Wmjg" {
		t.Fatalf("ct %s", got)
	}
	back, err := OpenItem(key, envHeader(), nonce, ct)
	if err != nil || !bytes.Equal(back, payload) {
		t.Fatalf("OpenItem = %q, %v", back, err)
	}
}

func TestSealVaultMetaMatchesIndependentVector(t *testing.T) {
	key := unhex(t, zeroVaultKey)
	h := VaultMetaHeader{AccountID: envAccount, VaultID: envVault, Rev: 1, UpdatedAt: 1762934400}
	payload := []byte(`{"schema":1,"name":"Personal"}`)
	nonce, ct, err := SealVaultMeta(key, h, payload, bytes.NewReader(seqBytes(0x30, 24)))
	if err != nil {
		t.Fatal(err)
	}
	if got := EncodeB64(ct); got != "PRj3IKq-07pkck6xCqbHsTW3m_ytOxvpsZG4gb_rc0pfRZGSasVIO7Y8nvhVhQ" {
		t.Fatalf("ct %s", got)
	}
	back, err := OpenVaultMeta(key, h, nonce, ct)
	if err != nil || !bytes.Equal(back, payload) {
		t.Fatalf("OpenVaultMeta = %q, %v", back, err)
	}
}

func TestOpenItemRejectsEveryMovedOrChangedRecord(t *testing.T) {
	key := unhex(t, zeroVaultKey)
	h := envHeader()
	nonce, ct, err := SealItem(key, h, []byte(`{"title":"bank"}`), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	with := func(f func(*ItemHeader)) ItemHeader {
		c := h
		f(&c)
		return c
	}
	flip := func(b []byte, i int) []byte {
		c := bytes.Clone(b)
		c[i] ^= 0x01
		return c
	}
	cases := []struct {
		name      string
		h         ItemHeader
		nonce, ct []byte
	}{
		{"moved to another account", with(func(c *ItemHeader) { c.AccountID = "_" + envAccount[1:] }), nonce, ct},
		{"moved to another vault", with(func(c *ItemHeader) { c.VaultID = "AAAAAAAAAAAAAAAAAAAAAA" }), nonce, ct},
		{"moved to another item", with(func(c *ItemHeader) { c.ItemID = "AAAAAAAAAAAAAAAAAAAAAA" }), nonce, ct},
		{"replayed as a newer rev", with(func(c *ItemHeader) { c.Rev = 8 }), nonce, ct},
		{"replayed as an older rev", with(func(c *ItemHeader) { c.Rev = 6 }), nonce, ct},
		{"updatedAt changed", with(func(c *ItemHeader) { c.UpdatedAt++ }), nonce, ct},
		{"marked deleted", with(func(c *ItemHeader) { c.Deleted = true }), nonce, ct},
		{"flipped ciphertext bit", h, nonce, flip(ct, 3)},
		{"flipped nonce bit", h, flip(nonce, 0), ct},
	}
	for _, tc := range cases {
		pt, err := OpenItem(key, tc.h, tc.nonce, tc.ct)
		if err != ErrAuth || pt != nil {
			t.Errorf("%s: OpenItem = %q, %v; want nil, ErrAuth", tc.name, pt, err)
		}
	}
	if pt, err := OpenItem(VaultKey(unhex(t, zeroRoot), "AAAAAAAAAAAAAAAAAAAAAA"), h, nonce, ct); err != ErrAuth || pt != nil {
		t.Errorf("another vault's key: %q, %v", pt, err)
	}
}

func TestItemAndVaultMetaCiphertextsDoNotCross(t *testing.T) {
	key := unhex(t, zeroVaultKey)
	// An item header and a vault-meta header with the same IDs and numbers
	// must not open each other's ciphertexts.
	ih := ItemHeader{AccountID: envAccount, VaultID: envVault, ItemID: envVault, Rev: 1, UpdatedAt: 5}
	mh := VaultMetaHeader{AccountID: envAccount, VaultID: envVault, Rev: 1, UpdatedAt: 5}
	n1, c1, err1 := SealItem(key, ih, []byte("x"), rand.Reader)
	n2, c2, err2 := SealVaultMeta(key, mh, []byte("x"), rand.Reader)
	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	if _, err := OpenVaultMeta(key, mh, n1, c1); err != ErrAuth {
		t.Errorf("an item ciphertext opened as vault meta: %v", err)
	}
	if _, err := OpenItem(key, ih, n2, c2); err != ErrAuth {
		t.Errorf("a vault-meta ciphertext opened as an item: %v", err)
	}
}

func TestEmptyPayloadIsATombstone(t *testing.T) {
	key := unhex(t, zeroVaultKey)
	h := envHeader()
	h.Deleted = true
	nonce, ct, err := SealItem(key, h, nil, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pt, err := OpenItem(key, h, nonce, ct)
	if err != nil || len(pt) != 0 {
		t.Fatalf("tombstone opened as %q, %v", pt, err)
	}
}

func TestHeadersThatCannotBeSpelledUnambiguouslyAreRefused(t *testing.T) {
	key := unhex(t, zeroVaultKey)
	bad := []ItemHeader{
		{AccountID: envAccount + "|x", VaultID: envVault, ItemID: envItem, Rev: 1},
		{AccountID: envAccount, VaultID: "a|b", ItemID: envItem, Rev: 1},
		{AccountID: envAccount, VaultID: envVault, ItemID: "", Rev: 1},
		{AccountID: envAccount, VaultID: envVault, ItemID: "id with space", Rev: 1},
		{AccountID: envAccount, VaultID: envVault, ItemID: "AAAA+AAA", Rev: 1},
		{AccountID: envAccount, VaultID: envVault, ItemID: envItem, Rev: 0},
		{AccountID: envAccount, VaultID: envVault, ItemID: envItem, Rev: 1, UpdatedAt: -1},
	}
	for _, h := range bad {
		if _, _, err := SealItem(key, h, []byte("x"), rand.Reader); !errors.Is(err, ErrHeader) {
			t.Errorf("SealItem(%+v): %v, want ErrHeader", h, err)
		}
		if _, err := OpenItem(key, h, make([]byte, NonceSize), make([]byte, Overhead)); err != ErrAuth {
			t.Errorf("OpenItem(%+v): %v, want ErrAuth", h, err)
		}
	}
	if _, _, err := SealVaultMeta(key, VaultMetaHeader{AccountID: envAccount, VaultID: "x|y", Rev: 1}, []byte("x"), rand.Reader); !errors.Is(err, ErrHeader) {
		t.Errorf("SealVaultMeta with | in the vault id: %v", err)
	}
}
