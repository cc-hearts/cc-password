package identity

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"bipPass/internal/vaultcrypto"
)

// ErrUnlock is the one error of every failed unlock: a wrong passphrase, a
// keyfile that was edited, a wrapped root that no longer matches its account,
// parameters this build refuses. It never says which, and never repeats the
// passphrase, so a caller cannot tell an attacker how close they were.
var ErrUnlock = errors.New("identity: could not open the keyfile")

// NewAccount makes an account from a mnemonic and a passphrase and returns the
// keyfile that describes it plus the session that holds its root key.
//
// The mnemonic is the whole identity: the same words give the same account on
// every device. The passphrase only guards this file, so losing it means
// writing a new keyfile from the mnemonic, not losing the account. r supplies
// the Argon2 salt and the wrap nonce; crypto/rand.Reader is the real source.
func NewAccount(mnemonic, passphrase string, now time.Time, r io.Reader) (*Keyfile, *Session, error) {
	if err := CheckPassphrase(passphrase); err != nil {
		return nil, nil, err
	}
	root, accountID, vaultID, err := deriveAccount(mnemonic)
	if err != nil {
		return nil, nil, err
	}
	w, err := vaultcrypto.WrapRoot(root, passphrase, accountID, vaultcrypto.DefaultArgon2, r)
	if err != nil {
		return nil, nil, err
	}
	kf := &Keyfile{
		Version:   KeyfileVersion,
		AccountID: accountID,
		VaultID:   vaultID,
		CreatedAt: now.UTC().Unix(),
	}
	kf.setWrapped(w)
	return kf, &Session{root: root, accountID: accountID, vaultID: vaultID}, nil
}

// Unlock opens a keyfile with a passphrase and returns the session that holds
// the root key it wrapped.
//
// The account id and the default vault id are re-derived from the root key and
// compared with the file, so a keyfile whose identity was edited is refused
// even though the wrapped root still opens under its own passphrase: the file
// would otherwise hand out another account's keys.
func Unlock(kf *Keyfile, passphrase string) (*Session, error) {
	if kf.Version != KeyfileVersion {
		return nil, fmt.Errorf("%w: %d, want %d", ErrKeyfileVersion, kf.Version, KeyfileVersion)
	}
	w, err := kf.wrapped()
	if err != nil {
		return nil, ErrUnlock
	}
	// A wrong passphrase, parameters this build refuses, and a file that no
	// longer matches its own account are one error: the caller has nothing
	// to act on with any of them, and an attacker learns nothing from which
	// one they got.
	root, err := vaultcrypto.UnwrapRoot(w, passphrase, kf.AccountID)
	if err != nil {
		return nil, ErrUnlock
	}
	accountID, vaultID, err := identitiesOf(root)
	if err != nil {
		return nil, ErrUnlock
	}
	if accountID != kf.AccountID || vaultID != kf.VaultID {
		return nil, ErrUnlock
	}
	return &Session{root: root, accountID: accountID, vaultID: vaultID}, nil
}

// deriveAccount runs the mnemonic through the key tree and returns the root
// key with the two public identifiers that hang off it.
func deriveAccount(mnemonic string) (root []byte, accountID, vaultID string, err error) {
	if err := vaultcrypto.ValidateMnemonic(mnemonic); err != nil {
		return nil, "", "", err
	}
	root = vaultcrypto.RootKey(vaultcrypto.MnemonicSeed(mnemonic, ""))
	accountID, vaultID, err = identitiesOf(root)
	if err != nil {
		return nil, "", "", err
	}
	return root, accountID, vaultID, nil
}

// identitiesOf derives the account id and the default vault id from a root
// key. Both are public, so they can be recomputed at any time and compared
// with what a file claims.
func identitiesOf(root []byte) (accountID, vaultID string, err error) {
	if len(root) != vaultcrypto.KeySize {
		return "", "", fmt.Errorf("identity: root key is %d bytes, want %d", len(root), vaultcrypto.KeySize)
	}
	pub, ok := vaultcrypto.IdentityKey(root).Public().(ed25519.PublicKey)
	if !ok {
		return "", "", fmt.Errorf("identity: the identity key has no public half")
	}
	return vaultcrypto.AccountID(pub), vaultcrypto.DefaultVaultID(root), nil
}

// Session is an open account: the root key in memory and nothing else. It
// hands out vault keys and forgets them, and Lock wipes the root key so the
// process holds no secret after the user locks it.
type Session struct {
	mu        sync.Mutex
	root      []byte
	accountID string
	vaultID   string
	locked    bool
}

// AccountID is the account's public id, the one the server knows it by.
func (s *Session) AccountID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.accountID
}

// DefaultVaultID is the id of the account's default vault, derived from the
// root key so every device agrees on it.
func (s *Session) DefaultVaultID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.vaultID
}

// VaultKey is the data key of a vault. It is derived from the root key each
// time, so a locked session hands out nothing rather than a stale key.
func (s *Session) VaultKey(vaultID string) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locked {
		return nil
	}
	return vaultcrypto.VaultKey(s.root, vaultID)
}

// Locked reports whether the session has been locked.
func (s *Session) Locked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.locked
}

// Lock wipes the root key. After it, every vault key this session would have
// handed out is gone, and the process holds no secret.
func (s *Session) Lock() {
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.root)
	s.root = nil
	s.locked = true
}

// DeriveAccount is the account id a mnemonic derives to, without wrapping a
// root key or touching a file. It is what a screen asks before it overwrites
// anything: recovering a mnemonic into a directory whose data belongs to
// another account has to be refused, and this is the cheapest way to tell.
func DeriveAccount(mnemonic string) (root []byte, accountID, vaultID string, err error) {
	return deriveAccount(mnemonic)
}
