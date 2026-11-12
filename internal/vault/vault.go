// Package vault is the encrypted vault engine: it opens every item in the
// store or refuses to open any of them, and it is the only thing that turns
// sealed blobs back into titles, usernames and passwords.
//
// It holds the session's root key only through the vault keys it derives, and
// it writes the envelope the vectors file pins: the routing columns in the
// clear, everything else behind XChaCha20-Poly1305 bound to the account, the
// vault, the item and its rev.
package vault

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"

	"bipPass/internal/identity"
	"bipPass/internal/store"
	"bipPass/internal/vaultcrypto"
)

// defaultVaultName is the name of the vault every account starts with.
const defaultVaultName = "个人"

// metaAccountID is the meta key that binds a database to one account.
const metaAccountID = "accountId"

// ErrOtherAccount is the error of a database that belongs to a different
// account. It is not a decryption failure: the file is fine, it is simply not
// this account's, and the user has to be told that rather than "wrong
// passphrase", because no passphrase will open it.
var ErrOtherAccount = errors.New("vault: this database belongs to another account")

// CorruptError is the error of a vault that will not open because some of its
// records failed to decrypt. The count is how many, so the user can be told
// what they are looking at instead of a bare failure.
type CorruptError struct {
	Count int
}

func (e *CorruptError) Error() string {
	if e.Count == 1 {
		return "vault: 1 item could not be decrypted"
	}
	return fmt.Sprintf("vault: %d items could not be decrypted", e.Count)
}

// Entry is one item as the engine holds it: its routing columns and its
// decrypted payload.
type Entry struct {
	ID        string
	VaultID   string
	Rev       int64
	UpdatedAt int64
	Deleted   bool
	Purged    bool
	Item      Item
}

// VaultInfo is a vault's routing columns and its decrypted name.
type VaultInfo struct {
	ID        string
	Name      string
	Rev       int64
	UpdatedAt int64
	CreatedAt int64
}

// Vault is an open vault: every item decrypted into memory and every vault
// name available. Nothing is written back until a caller asks it to be.
type Vault struct {
	db      *store.DB
	session *identity.Session

	accountID  string
	entries    map[string]Entry
	order      []string
	vaults     map[string]VaultInfo
	vaultKeys  map[string][]byte
	vaultOrder []string
	closed     bool
}

// CheckAccount reports whether the database belongs to this account. A
// database with no account bound yet is this account's to claim.
func CheckAccount(db *store.DB, accountID string) error {
	bound, err := db.Meta(metaAccountID)
	if err != nil {
		// Nothing is bound: the first account to open it owns it.
		return nil
	}
	if bound != accountID {
		return ErrOtherAccount
	}
	return nil
}

// Open decrypts every item in the store, or refuses to open the vault at all.
//
// The account is bound to the database on the first open, so a second account
// opening the same file is told it is not theirs rather than being shown a
// vault it cannot read. The default vault is created if the account has none,
// at the id derived from the root key so every device agrees on it.
//
// If any item fails to decrypt — a flipped byte, a truncated blob, an old rev
// copied back — the whole vault refuses with a CorruptError and a count.
// Nothing is partially displayed: authentication failure fails as a whole.
func Open(db *store.DB, s *identity.Session, now int64) (*Vault, error) {
	if s == nil || s.Locked() {
		return nil, fmt.Errorf("vault: the session is locked")
	}
	if err := CheckAccount(db, s.AccountID()); err != nil {
		return nil, err
	}
	v := &Vault{
		db:        db,
		session:   s,
		accountID: s.AccountID(),
		entries:   map[string]Entry{},
		vaults:    map[string]VaultInfo{},
		vaultKeys: map[string][]byte{},
	}
	if err := db.SetMeta(metaAccountID, v.accountID); err != nil {
		return nil, err
	}
	vaultRows, err := db.Vaults()
	if err != nil {
		return nil, err
	}
	// Every vault key comes from the root key, so a vault this account does
	// not own cannot be read even if its row is in the file.
	for _, row := range vaultRows {
		key := s.VaultKey(row.ID)
		if key == nil {
			return nil, fmt.Errorf("vault: the session is locked")
		}
		name, err := openVaultMeta(v.accountID, key, row)
		if err != nil {
			return nil, fmt.Errorf("vault: reading the vault %s: %w", row.ID, err)
		}
		v.vaults[row.ID] = VaultInfo{
			ID: row.ID, Name: name, Rev: row.Rev,
			UpdatedAt: row.UpdatedAt, CreatedAt: row.CreatedAt,
		}
		v.vaultKeys[row.ID] = key
		v.vaultOrder = append(v.vaultOrder, row.ID)
	}
	if len(v.vaults) == 0 {
		// The account's first vault is the default one, at the derived id.
		if _, err := v.putVault(s.DefaultVaultID(), defaultVaultName, 1, now, now); err != nil {
			return nil, err
		}
	}
	rows, err := db.Items()
	if err != nil {
		return nil, err
	}
	var corrupt int
	for _, row := range rows {
		key, ok := v.vaultKeys[row.VaultID]
		if !ok {
			// An item in a vault that is not there: its key cannot be
			// derived, so it is unreadable rather than skipped.
			corrupt++
			continue
		}
		item, err := openItem(v.accountID, key, row)
		if err != nil {
			corrupt++
			continue
		}
		v.entries[row.ID] = Entry{
			ID: row.ID, VaultID: row.VaultID, Rev: row.Rev,
			UpdatedAt: row.UpdatedAt, Deleted: row.Deleted, Purged: len(row.CT) == 0,
			Item: item,
		}
		v.order = append(v.order, row.ID)
	}
	if corrupt > 0 {
		return nil, &CorruptError{Count: corrupt}
	}
	return v, nil
}

// openVaultMeta decrypts a vault's name.
func openVaultMeta(accountID string, key []byte, row store.VaultRecord) (string, error) {
	payload, err := vaultcrypto.OpenVaultMeta(key, vaultcrypto.VaultMetaHeader{
		AccountID: accountID, VaultID: row.ID, Rev: row.Rev, UpdatedAt: row.UpdatedAt,
	}, row.Nonce, row.CT)
	if err != nil {
		return "", err
	}
	var meta struct {
		Schema int    `json:"schema"`
		Name   string `json:"name"`
	}
	if err := json.Unmarshal(payload, &meta); err != nil {
		return "", fmt.Errorf("vault: reading the vault's name: %w", err)
	}
	return meta.Name, nil
}

// openItem decrypts one item's payload. A tombstone has no ciphertext at all,
// and is an item with nothing in it.
func openItem(accountID string, key []byte, row store.ItemRecord) (Item, error) {
	if len(row.CT) == 0 {
		return Item{Schema: SchemaVersion}, nil
	}
	payload, err := vaultcrypto.OpenItem(key, vaultcrypto.ItemHeader{
		AccountID: accountID, VaultID: row.VaultID, ItemID: row.ID,
		Rev: row.Rev, UpdatedAt: row.UpdatedAt, Deleted: row.Deleted,
	}, row.Nonce, row.CT)
	if err != nil {
		return Item{}, err
	}
	return UnmarshalItem(payload)
}

// Entries is every open item, in id order.
func (v *Vault) Entries() []Entry {
	out := make([]Entry, 0, len(v.order))
	for _, id := range v.order {
		out = append(out, v.entries[id])
	}
	return out
}

// Entry is one item, and whether it is there at all.
func (v *Vault) Entry(id string) (Entry, bool) {
	e, ok := v.entries[id]
	return e, ok
}

// Vaults is every vault, in id order.
func (v *Vault) Vaults() []VaultInfo {
	out := make([]VaultInfo, 0, len(v.vaultOrder))
	for _, id := range v.vaultOrder {
		out = append(out, v.vaults[id])
	}
	return out
}

// Create adds an item to a vault. Its rev starts at 1, and its payload is
// sealed under that vault's key.
func (v *Vault) Create(vaultID string, it Item, now int64) (Entry, error) {
	if v.closed {
		return Entry{}, fmt.Errorf("vault: the vault is closed")
	}
	key, ok := v.vaultKeys[vaultID]
	if !ok {
		return Entry{}, fmt.Errorf("vault: there is no vault %s", vaultID)
	}
	id, err := vaultcrypto.NewID(rand.Reader)
	if err != nil {
		return Entry{}, err
	}
	it.VaultID = vaultID
	if it.CreatedAt == 0 {
		it.CreatedAt = now
	}
	it.UpdatedAt = now
	row, err := v.sealItem(key, id, 1, it, false)
	if err != nil {
		return Entry{}, err
	}
	if err := v.db.PutItem(row, store.OpCreate, now); err != nil {
		return Entry{}, err
	}
	e := Entry{ID: id, VaultID: vaultID, Rev: 1, UpdatedAt: now, Item: it}
	v.entries[id] = e
	v.order = append(v.order, id)
	return e, nil
}

// Update replaces an item's payload at the next rev. The password it replaced
// moves into the item's history, so the user can see what they used to use.
func (v *Vault) Update(id string, it Item, now int64) (Entry, error) {
	if v.closed {
		return Entry{}, fmt.Errorf("vault: the vault is closed")
	}
	e, ok := v.entries[id]
	if !ok {
		return Entry{}, fmt.Errorf("vault: there is no item %s", id)
	}
	if e.Purged {
		return Entry{}, fmt.Errorf("vault: a permanently deleted item cannot be edited")
	}
	if e.Deleted {
		return Entry{}, fmt.Errorf("vault: this item is deleted; restore it before editing it")
	}
	if e.Item.ReadOnly() {
		return Entry{}, fmt.Errorf("vault: this item's format is newer than this build")
	}
	key := v.vaultKeys[e.VaultID]
	rev := e.Rev + 1
	// The history is the engine's to keep: a caller sends the new password,
	// and the one it replaced moves in behind the others.
	if it.Password != "" && it.Password != e.Item.Password {
		it.History = append(it.History, PasswordChange{Password: e.Item.Password, ChangedAt: e.UpdatedAt})
	}
	it.VaultID = e.VaultID
	it.CreatedAt = e.Item.CreatedAt
	it.UpdatedAt = now
	row, err := v.sealItem(key, id, rev, it, false)
	if err != nil {
		return Entry{}, err
	}
	if err := v.db.PutItem(row, store.OpUpdate, now); err != nil {
		return Entry{}, err
	}
	out := Entry{ID: id, VaultID: e.VaultID, Rev: rev, UpdatedAt: now, Item: it}
	v.entries[id] = out
	return out, nil
}

// Delete is a soft delete: the payload stays, so the item can be restored.
func (v *Vault) Delete(id string, now int64) error {
	if v.closed {
		return fmt.Errorf("vault: the vault is closed")
	}
	e, ok := v.entries[id]
	if !ok {
		return fmt.Errorf("vault: there is no item %s", id)
	}
	if e.Deleted {
		return nil
	}
	key := v.vaultKeys[e.VaultID]
	row, err := v.sealItem(key, id, e.Rev+1, e.Item, true)
	if err != nil {
		return err
	}
	if err := v.db.PutItem(row, store.OpDelete, now); err != nil {
		return err
	}
	e.Rev, e.UpdatedAt, e.Deleted = e.Rev+1, now, true
	v.entries[id] = e
	return nil
}

// Restore brings a soft-deleted item back. A purged item has no payload left,
// so it cannot come back.
func (v *Vault) Restore(id string, now int64) error {
	if v.closed {
		return fmt.Errorf("vault: the vault is closed")
	}
	e, ok := v.entries[id]
	if !ok {
		return fmt.Errorf("vault: there is no item %s", id)
	}
	if !e.Deleted {
		return nil
	}
	if e.Purged {
		return fmt.Errorf("vault: this item was permanently deleted")
	}
	key := v.vaultKeys[e.VaultID]
	row, err := v.sealItem(key, id, e.Rev+1, e.Item, false)
	if err != nil {
		return err
	}
	if err := v.db.PutItem(row, store.OpRestore, now); err != nil {
		return err
	}
	e.Rev, e.UpdatedAt, e.Deleted = e.Rev+1, now, false
	v.entries[id] = e
	return nil
}

// Purge is a tombstone: the item is deleted and its ciphertext is emptied, so
// there is nothing left to restore. The routing row stays, so a sync does not
// bring the item back from another device.
func (v *Vault) Purge(id string, now int64) error {
	if v.closed {
		return fmt.Errorf("vault: the vault is closed")
	}
	e, ok := v.entries[id]
	if !ok {
		return fmt.Errorf("vault: there is no item %s", id)
	}
	if e.Purged {
		return nil
	}
	// The tombstone carries no ciphertext at all: nothing is left to seal,
	// and the store's rev rule is what keeps it from being replayed.
	row := store.ItemRecord{
		ID: id, VaultID: e.VaultID, Rev: e.Rev + 1, UpdatedAt: now, Deleted: true,
	}
	if err := v.db.PutItem(row, store.OpPurge, now); err != nil {
		return err
	}
	e.Rev, e.UpdatedAt, e.Deleted, e.Purged = e.Rev+1, now, true, true
	e.Item = Item{Schema: SchemaVersion}
	v.entries[id] = e
	return nil
}

// CreateVault makes a vault with a new random id and a sealed name.
func (v *Vault) CreateVault(name string, now int64) (VaultInfo, error) {
	if v.closed {
		return VaultInfo{}, fmt.Errorf("vault: the vault is closed")
	}
	id, err := vaultcrypto.NewID(rand.Reader)
	if err != nil {
		return VaultInfo{}, err
	}
	return v.putVault(id, name, 1, now, now)
}

// RenameVault gives a vault a new name at its next rev.
func (v *Vault) RenameVault(id, name string, now int64) error {
	if v.closed {
		return fmt.Errorf("vault: the vault is closed")
	}
	info, ok := v.vaults[id]
	if !ok {
		return fmt.Errorf("vault: there is no vault %s", id)
	}
	_, err := v.putVault(id, name, info.Rev+1, now, info.CreatedAt)
	return err
}

// putVault seals a vault's name and stores it.
func (v *Vault) putVault(id, name string, rev int64, updatedAt, createdAt int64) (VaultInfo, error) {
	key := v.session.VaultKey(id)
	if key == nil {
		return VaultInfo{}, fmt.Errorf("vault: the session is locked")
	}
	payload, err := json.Marshal(struct {
		Schema int    `json:"schema"`
		Name   string `json:"name"`
	}{Schema: SchemaVersion, Name: name})
	if err != nil {
		return VaultInfo{}, fmt.Errorf("vault: writing the vault's name: %w", err)
	}
	nonce, ct, err := vaultcrypto.SealVaultMeta(key, vaultcrypto.VaultMetaHeader{
		AccountID: v.accountID, VaultID: id, Rev: rev, UpdatedAt: updatedAt,
	}, payload, rand.Reader)
	if err != nil {
		return VaultInfo{}, err
	}
	row := store.VaultRecord{ID: id, Rev: rev, UpdatedAt: updatedAt, CreatedAt: createdAt, Nonce: nonce, CT: ct}
	if err := v.db.PutVault(row); err != nil {
		return VaultInfo{}, err
	}
	info := VaultInfo{ID: id, Name: name, Rev: rev, UpdatedAt: updatedAt, CreatedAt: createdAt}
	if _, seen := v.vaults[id]; !seen {
		v.vaultOrder = append(v.vaultOrder, id)
	}
	v.vaults[id] = info
	v.vaultKeys[id] = key
	return info, nil
}

// sealItem seals an item's payload at a rev, with every routing column bound
// into the associated data so a replayed or edited row does not open.
func (v *Vault) sealItem(key []byte, id string, rev int64, it Item, deleted bool) (store.ItemRecord, error) {
	payload, err := MarshalItem(it)
	if err != nil {
		return store.ItemRecord{}, err
	}
	nonce, ct, err := vaultcrypto.SealItem(key, vaultcrypto.ItemHeader{
		AccountID: v.accountID, VaultID: it.VaultID, ItemID: id,
		Rev: rev, UpdatedAt: it.UpdatedAt, Deleted: deleted,
	}, payload, rand.Reader)
	if err != nil {
		return store.ItemRecord{}, err
	}
	return store.ItemRecord{
		ID: id, VaultID: it.VaultID, Rev: rev, UpdatedAt: it.UpdatedAt,
		Deleted: deleted, Nonce: nonce, CT: ct,
	}, nil
}

// Close drops the vault's in-memory keys. The database stays open: closing it
// is the caller's, and they may want to reopen the vault.
func (v *Vault) Close() error {
	if v.closed {
		return nil
	}
	v.closed = true
	for id, key := range v.vaultKeys {
		clear(key)
		delete(v.vaultKeys, id)
	}
	v.entries = nil
	v.order = nil
	return nil
}
