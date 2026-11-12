package vaultcrypto

import (
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

// KeySize is the size of every symmetric key of the key tree.
const KeySize = 32

// IDSize is the size of account-independent IDs (vaults, items), in bytes.
const IDSize = 16

// rootSalt is the HKDF salt of the root key: SHA-256("ccpass/v1/root").
var rootSalt = func() []byte {
	sum := sha256.Sum256([]byte("ccpass/v1/root"))
	return sum[:]
}()

// hkdfSHA256 is HKDF-SHA256 (RFC 5869). A nil salt is the RFC's default,
// as many zero bytes as the hash is long.
func hkdfSHA256(secret, salt []byte, info string, length int) []byte {
	key, err := hkdf.Key(sha256.New, secret, salt, info, length)
	if err != nil {
		// crypto/hkdf fails only for lengths over 255 hashes, or in FIPS
		// 140-only mode, which ccpass does not run in.
		panic("vaultcrypto: " + err.Error())
	}
	return key
}

// RootKey derives the root key from a BIP39 seed:
// HKDF-SHA256(seed, salt = SHA-256("ccpass/v1/root"), info = "root").
// It never changes for an account.
func RootKey(seed []byte) []byte {
	return hkdfSHA256(seed, rootSalt, "root", KeySize)
}

// IdentityKey derives the account's Ed25519 key, whose seed is
// HKDF-SHA256(root, info = "ccpass/v1/identity").
func IdentityKey(root []byte) ed25519.PrivateKey {
	seed := hkdfSHA256(root, nil, "ccpass/v1/identity", ed25519.SeedSize)
	defer clear(seed)
	return ed25519.NewKeyFromSeed(seed)
}

// AccountID is the account's ID: its Ed25519 public key in base64url.
func AccountID(pub ed25519.PublicKey) string {
	return EncodeB64(pub)
}

// DefaultVaultID derives the ID of the account's default vault, so that
// every device recovering the account names it alike:
// HKDF-SHA256(root, info = "ccpass/v1/vault-id|default"), 16 bytes.
func DefaultVaultID(root []byte) string {
	return EncodeB64(hkdfSHA256(root, nil, "ccpass/v1/vault-id|default", IDSize))
}

// VaultKey derives the key of a vault:
// HKDF-SHA256(root, info = "ccpass/v1/vault|" + vaultID).
func VaultKey(root []byte, vaultID string) []byte {
	return hkdfSHA256(root, nil, "ccpass/v1/vault|"+vaultID, KeySize)
}

// NewID makes a random 16-byte ID from r, which is crypto/rand.Reader
// outside tests.
func NewID(r io.Reader) (string, error) {
	b := make([]byte, IDSize)
	if _, err := io.ReadFull(r, b); err != nil {
		return "", fmt.Errorf("vaultcrypto: reading an id: %w", err)
	}
	return EncodeB64(b), nil
}

// EncodeB64 encodes b in base64url without padding (RFC 4648 §5).
func EncodeB64(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

var errB64 = errors.New("vaultcrypto: not canonical base64url")

// DecodeB64 decodes canonical base64url without padding: one spelling
// per value, so no padding, line breaks or stray trailing bits.
func DecodeB64(s string) ([]byte, error) {
	if strings.ContainsAny(s, "\r\n") {
		return nil, errB64
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(s)
	if err != nil {
		return nil, errB64
	}
	return b, nil
}
