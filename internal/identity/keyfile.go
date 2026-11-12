// Package identity is the local half of an account: the keyfile on disk, the
// passphrase that opens it, and the root key that lives only in memory while
// it is open.
//
// Nothing here ever asks the server for anything. A keyfile is a file on the
// user's own machine holding one wrapped root key, and the only thing that can
// open it is the passphrase the user typed. The mnemonic, the passphrase and
// the root key are never written to it.
package identity

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"ccpass/internal/vaultcrypto"
)

// KeyfileVersion is the only version this build reads or writes.
const KeyfileVersion = 1

// keyfileMode is the permission a keyfile is written with: the owner can read
// and write it, and nobody else can even see that it exists.
const keyfileMode os.FileMode = 0o600

// dirMode is the permission of a directory created for a keyfile.
const dirMode os.FileMode = 0o700

// ErrKeyfileVersion is the error of a keyfile this build does not know how to
// read. It is its own error, not ErrUnlock: a version mismatch says nothing
// about the passphrase, and the caller has to be able to offer a migration
// rather than "try again".
var ErrKeyfileVersion = errors.New("identity: keyfile version is not supported")

// ErrPassphraseShort is the error of a passphrase too short to wrap a root
// key with.
var ErrPassphraseShort = errors.New("identity: passphrase is too short")

// MinPassphraseRunes is the shortest passphrase accepted. It is counted in
// runes, not bytes, so a Chinese passphrase of eight characters is as long as
// an English one of eight characters.
const MinPassphraseRunes = 8

// argon2Block is the keyfile's argon2 object. The field names are the format:
// a port that spells them differently cannot read the file, so they are
// written out here rather than inherited from vaultcrypto's struct, which
// has no JSON of its own.
type argon2Block struct {
	MKiB uint32 `json:"mKiB"`
	T    uint32 `json:"t"`
	P    uint8  `json:"p"`
	Salt string `json:"salt"`
}

func (b argon2Block) params() vaultcrypto.Argon2Params {
	return vaultcrypto.Argon2Params{MKiB: b.MKiB, T: b.T, P: b.P}
}

// Keyfile is what a keyfile.json holds. Every field is either a public
// identifier or a wrapped key; the root key itself is never one of them.
type Keyfile struct {
	// Version names the format of the rest of the file.
	Version int `json:"version"`

	// AccountID is the base64url Ed25519 public key of the account. It is
	// the identity on the server and the associated data of the wrap, so
	// changing it makes the wrapped root unopenable.
	AccountID string `json:"accountId"`

	// VaultID is the id of the account's default vault, derived from the
	// root key so every device that recovers this account agrees on it.
	VaultID string `json:"vaultId"`

	// Argon2 holds the parameters the root key was wrapped with, and the
	// salt it was derived from. They are saved so a stronger default needs
	// no migration, and so opening a keyfile spends the memory it was made
	// with.
	Argon2 argon2Block `json:"argon2"`

	// WrapNonce is the 24-byte XChaCha20-Poly1305 nonce, base64url.
	WrapNonce string `json:"wrapNonce"`

	// WrappedRoot is the root key sealed under the passphrase's key,
	// base64url.
	WrappedRoot string `json:"wrappedRoot"`

	// CreatedAt is when the keyfile was written, in Unix seconds. It is
	// only ever shown to the user: it is not authenticated, so nothing may
	// depend on it.
	CreatedAt int64 `json:"createdAt"`
}

// CheckPassphrase reports whether a passphrase is long enough to wrap a root
// key. The check is on runes, so it means the same thing in any script.
func CheckPassphrase(passphrase string) error {
	if n := len([]rune(passphrase)); n < MinPassphraseRunes {
		return fmt.Errorf("%w: %d characters, want at least %d", ErrPassphraseShort, n, MinPassphraseRunes)
	}
	return nil
}

// LoadKeyfile reads a keyfile. A file this build cannot parse, or whose
// version it does not support, is refused before any Argon2 runs; the only
// thing that reaches the passphrase is a keyfile that looks like version 1.
func LoadKeyfile(path string) (*Keyfile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var kf Keyfile
	if err := json.Unmarshal(raw, &kf); err != nil {
		return nil, fmt.Errorf("identity: reading the keyfile: %w", err)
	}
	if kf.Version != KeyfileVersion {
		return nil, fmt.Errorf("%w: %d, want %d", ErrKeyfileVersion, kf.Version, KeyfileVersion)
	}
	if err := kf.complete(); err != nil {
		return nil, err
	}
	return &kf, nil
}

// complete reports whether every field a version 1 keyfile needs is there. A
// file from something else parses as JSON and then fails here, on load,
// rather than reaching the passphrase and failing as a wrong one.
func (kf *Keyfile) complete() error {
	for _, f := range []struct{ name, value string }{
		{"accountId", kf.AccountID},
		{"vaultId", kf.VaultID},
		{"argon2.salt", kf.Argon2.Salt},
		{"wrapNonce", kf.WrapNonce},
		{"wrappedRoot", kf.WrappedRoot},
	} {
		if f.value == "" {
			return fmt.Errorf("identity: the keyfile has no %s", f.name)
		}
	}
	return nil
}

// SaveKeyfile writes the keyfile at path, atomically and with the owner-only
// permission. The bytes go to a temporary file beside the destination, are
// flushed to the disk, and are then renamed over it, so a crash or a full disk
// leaves either the old keyfile or the new one, never half of either.
func SaveKeyfile(path string, kf *Keyfile) error {
	raw, err := json.Marshal(kf)
	if err != nil {
		return fmt.Errorf("identity: writing the keyfile: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".keyfile-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	// The temporary file holds a wrapped root key, so it gets the same
	// permission as the destination and is removed on every failure path.
	remove := true
	defer func() {
		if remove {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(keyfileMode); err != nil {
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		// The rename has to reach the disk too, or a power loss can leave the
		// directory entry pointing at the old contents.
		d, err := os.Open(dir)
		if err != nil {
			return err
		}
		if err := d.Sync(); err != nil {
			d.Close()
			return err
		}
		if err := d.Close(); err != nil {
			return err
		}
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	remove = false
	return os.Chmod(path, keyfileMode)
}

// setWrapped puts a wrapped root key into the keyfile's fields.
func (kf *Keyfile) setWrapped(w vaultcrypto.Wrapped) {
	kf.Argon2 = argon2Block{
		MKiB: w.Params.MKiB,
		T:    w.Params.T,
		P:    w.Params.P,
		Salt: vaultcrypto.EncodeB64(w.Salt),
	}
	kf.WrapNonce = vaultcrypto.EncodeB64(w.Nonce)
	kf.WrappedRoot = vaultcrypto.EncodeB64(w.CT)
}

// wrapped is the wrapped root key the file holds, for UnwrapRoot. A field
// that is not canonical base64url is a corrupted file, and fails as ErrUnlock
// like every other way of not being able to open it.
func (kf *Keyfile) wrapped() (vaultcrypto.Wrapped, error) {
	salt, err := vaultcrypto.DecodeB64(kf.Argon2.Salt)
	if err != nil {
		return vaultcrypto.Wrapped{}, err
	}
	nonce, err := vaultcrypto.DecodeB64(kf.WrapNonce)
	if err != nil {
		return vaultcrypto.Wrapped{}, err
	}
	ct, err := vaultcrypto.DecodeB64(kf.WrappedRoot)
	if err != nil {
		return vaultcrypto.Wrapped{}, err
	}
	return vaultcrypto.Wrapped{Params: kf.Argon2.params(), Salt: salt, Nonce: nonce, CT: ct}, nil
}
