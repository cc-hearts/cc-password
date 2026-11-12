package vaultcrypto

import (
	"errors"
	"io"
	"strconv"
)

// ErrHeader is the error of sealing a record whose header cannot be
// spelled unambiguously in associated data: an ID that is empty or not
// base64url, a rev below 1 or a negative time.
var ErrHeader = errors.New("vaultcrypto: invalid record header")

// ItemHeader is what an item record keeps in plaintext. All of it is
// bound to the ciphertext, so a record moved to another account, vault or
// item, replayed at another rev, re-dated or marked deleted fails to open.
type ItemHeader struct {
	AccountID string
	VaultID   string
	ItemID    string
	Rev       int64
	UpdatedAt int64
	Deleted   bool
}

// AAD is the associated data of the item:
// "ccpass/v1/item|account|vault|item|rev|updatedAt|deleted", the numbers
// in decimal and deleted as 1 or 0.
func (h ItemHeader) AAD() []byte {
	deleted := "0"
	if h.Deleted {
		deleted = "1"
	}
	return []byte("ccpass/v1/item|" + h.AccountID + "|" + h.VaultID + "|" + h.ItemID + "|" +
		strconv.FormatInt(h.Rev, 10) + "|" + strconv.FormatInt(h.UpdatedAt, 10) + "|" + deleted)
}

func (h ItemHeader) valid() bool {
	return isID(h.AccountID) && isID(h.VaultID) && isID(h.ItemID) && h.Rev >= 1 && h.UpdatedAt >= 0
}

// SealItem encrypts an item's payload under its vault's key, reading a
// fresh nonce from r. An empty payload is the tombstone of a purged item.
func SealItem(vaultKey []byte, h ItemHeader, payload []byte, r io.Reader) (nonce, ct []byte, err error) {
	if !h.valid() {
		return nil, nil, ErrHeader
	}
	return Seal(vaultKey, payload, h.AAD(), r)
}

// OpenItem decrypts an item's payload. Every failure is ErrAuth.
func OpenItem(vaultKey []byte, h ItemHeader, nonce, ct []byte) ([]byte, error) {
	if !h.valid() {
		return nil, ErrAuth
	}
	return Open(vaultKey, nonce, ct, h.AAD())
}

// VaultMetaHeader is what a vault record keeps in plaintext; its name and
// everything else about it is in the ciphertext.
type VaultMetaHeader struct {
	AccountID string
	VaultID   string
	Rev       int64
	UpdatedAt int64
}

// AAD is the associated data of the vault's metadata:
// "ccpass/v1/vault-meta|account|vault|rev|updatedAt".
func (h VaultMetaHeader) AAD() []byte {
	return []byte("ccpass/v1/vault-meta|" + h.AccountID + "|" + h.VaultID + "|" +
		strconv.FormatInt(h.Rev, 10) + "|" + strconv.FormatInt(h.UpdatedAt, 10))
}

func (h VaultMetaHeader) valid() bool {
	return isID(h.AccountID) && isID(h.VaultID) && h.Rev >= 1 && h.UpdatedAt >= 0
}

// SealVaultMeta encrypts a vault's metadata under the vault's key.
func SealVaultMeta(vaultKey []byte, h VaultMetaHeader, payload []byte, r io.Reader) (nonce, ct []byte, err error) {
	if !h.valid() {
		return nil, nil, ErrHeader
	}
	return Seal(vaultKey, payload, h.AAD(), r)
}

// OpenVaultMeta decrypts a vault's metadata. Every failure is ErrAuth.
func OpenVaultMeta(vaultKey []byte, h VaultMetaHeader, nonce, ct []byte) ([]byte, error) {
	if !h.valid() {
		return nil, ErrAuth
	}
	return Open(vaultKey, nonce, ct, h.AAD())
}

// isID reports whether s is a non-empty run of the base64url alphabet,
// which never holds the "|" that separates the fields of associated data.
func isID(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !('A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
