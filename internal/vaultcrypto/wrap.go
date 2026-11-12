package vaultcrypto

import (
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
	"golang.org/x/text/unicode/norm"
)

// SaltSize is the size of the Argon2id salt of a wrapped root key.
const SaltSize = 16

// Argon2Params are the Argon2id costs of a key-encryption key: memory in
// KiB, passes and lanes. A keyfile keeps the parameters it was wrapped
// with, so that stronger defaults need no migration.
type Argon2Params struct {
	MKiB uint32
	T    uint32
	P    uint8
}

// DefaultArgon2 are the parameters new keyfiles use: 64 MiB, 3 passes,
// 1 lane. They are also the weakest that Check accepts.
var DefaultArgon2 = Argon2Params{MKiB: 65536, T: 3, P: 1}

// The most that Check accepts: a keyfile asking for more is an attack on
// the memory or the patience of the machine opening it.
const (
	maxArgon2MKiB = 4 << 20 // 4 GiB
	maxArgon2T    = 64
	maxArgon2P    = 16
)

// ErrArgon2Params is the error of parameters out of range.
var ErrArgon2Params = errors.New("vaultcrypto: argon2 parameters out of range")

// Check reports whether p is no weaker than DefaultArgon2 and no larger
// than a machine can run.
func (p Argon2Params) Check() error {
	if p.MKiB < DefaultArgon2.MKiB || p.MKiB > maxArgon2MKiB ||
		p.T < DefaultArgon2.T || p.T > maxArgon2T ||
		p.P < DefaultArgon2.P || p.P > maxArgon2P {
		return fmt.Errorf("%w: m=%d KiB t=%d p=%d", ErrArgon2Params, p.MKiB, p.T, p.P)
	}
	return nil
}

// DeriveKEK derives a 32-byte key-encryption key from a passphrase with
// Argon2id, after putting the passphrase in NFKD. It does not Check p:
// WrapRoot and UnwrapRoot do.
func DeriveKEK(passphrase string, salt []byte, p Argon2Params) []byte {
	pw := []byte(norm.NFKD.String(passphrase))
	defer clear(pw)
	return argon2.IDKey(pw, salt, p.T, p.MKiB, p.P, KeySize)
}

// Wrapped is a root key encrypted under a passphrase.
type Wrapped struct {
	Params Argon2Params
	Salt   []byte
	Nonce  []byte
	CT     []byte
}

func wrapAAD(accountID string) []byte {
	return []byte("ccpass/v1/wrap|" + accountID)
}

// WrapRoot encrypts the root key under a passphrase: XChaCha20-Poly1305
// with the Argon2id key of the passphrase, bound to the account by the
// associated data "ccpass/v1/wrap|" + accountID. It reads the 16-byte salt
// and then the 24-byte nonce from r.
func WrapRoot(root []byte, passphrase, accountID string, p Argon2Params, r io.Reader) (Wrapped, error) {
	if err := p.Check(); err != nil {
		return Wrapped{}, err
	}
	if len(root) != KeySize {
		return Wrapped{}, fmt.Errorf("vaultcrypto: root key is %d bytes, want %d", len(root), KeySize)
	}
	salt := make([]byte, SaltSize)
	if _, err := io.ReadFull(r, salt); err != nil {
		return Wrapped{}, fmt.Errorf("vaultcrypto: reading a salt: %w", err)
	}
	kek := DeriveKEK(passphrase, salt, p)
	defer clear(kek)
	nonce, ct, err := Seal(kek, root, wrapAAD(accountID), r)
	if err != nil {
		return Wrapped{}, err
	}
	return Wrapped{Params: p, Salt: salt, Nonce: nonce, CT: ct}, nil
}

// UnwrapRoot decrypts a wrapped root key. Parameters out of range fail
// with ErrArgon2Params before Argon2 runs; every other failure is ErrAuth.
func UnwrapRoot(w Wrapped, passphrase, accountID string) ([]byte, error) {
	if err := w.Params.Check(); err != nil {
		return nil, err
	}
	if len(w.Salt) != SaltSize {
		return nil, ErrAuth
	}
	kek := DeriveKEK(passphrase, w.Salt, w.Params)
	defer clear(kek)
	root, err := Open(kek, w.Nonce, w.CT, wrapAAD(accountID))
	if err != nil || len(root) != KeySize {
		return nil, ErrAuth
	}
	return root, nil
}
