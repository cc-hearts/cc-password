package vaultcrypto

import (
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
)

// NonceSize is the size of an XChaCha20-Poly1305 nonce.
const NonceSize = chacha20poly1305.NonceSizeX

// Overhead is the size of the Poly1305 tag that a sealed message carries.
const Overhead = chacha20poly1305.Overhead

// ErrAuth is the one error of every failed decryption: a wrong key or
// passphrase, a changed ciphertext, nonce or tag, or other associated
// data. It never says which.
var ErrAuth = errors.New("vaultcrypto: authentication failed")

// Seal encrypts plaintext with XChaCha20-Poly1305 under key, binding aad,
// with a fresh 24-byte nonce read from r (crypto/rand.Reader outside
// tests). ct holds the ciphertext followed by the 16-byte tag.
func Seal(key, plaintext, aad []byte, r io.Reader) (nonce, ct []byte, err error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, nil, fmt.Errorf("vaultcrypto: %w", err)
	}
	nonce = make([]byte, NonceSize)
	if _, err := io.ReadFull(r, nonce); err != nil {
		return nil, nil, fmt.Errorf("vaultcrypto: reading a nonce: %w", err)
	}
	return nonce, aead.Seal(nil, nonce, plaintext, aad), nil
}

// Open decrypts what Seal sealed. Any failure is ErrAuth, and no part of
// the plaintext is returned with it.
func Open(key, nonce, ct, aad []byte) ([]byte, error) {
	if len(key) != KeySize || len(nonce) != NonceSize || len(ct) < Overhead {
		return nil, ErrAuth
	}
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, ErrAuth
	}
	pt, err := aead.Open(nil, nonce, ct, aad)
	if err != nil {
		return nil, ErrAuth
	}
	return pt, nil
}
