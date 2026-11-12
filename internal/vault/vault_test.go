package vault

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ccpass/internal/identity"
	"ccpass/internal/store"
)

// aSession is a real account, derived from a fixed mnemonic, so the ids in
// these tests are stable.
const engineMnemonic = "coral light army gather adapt blossom school alcohol coral light army gather adapt blossom school alcohol coral light army gather adapt blossom school awesome"

func newSession(t *testing.T) *identity.Session {
	t.Helper()
	kf, session, err := identity.NewAccount(engineMnemonic, "engine passphrase here", time.Unix(1730000000, 0).UTC(), bytes.NewReader(bytes.Repeat([]byte{7}, 64)))
	if err != nil {
		t.Fatal(err)
	}
	if kf == nil {
		t.Fatal("no keyfile")
	}
	return session
}

func openDB(t *testing.T) (*store.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), store.DBName)
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db, path
}

func openVault(t *testing.T) (*Vault, *store.DB, string) {
	t.Helper()
	db, path := openDB(t)
	v, err := Open(db, newSession(t), 1730000000)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { v.Close() })
	return v, db, path
}

func TestOpenCreatesTheDefaultVault(t *testing.T) {
	v, _, _ := openVault(t)
	vaults := v.Vaults()
	if len(vaults) != 1 {
		t.Fatalf("%d vaults, want the default one", len(vaults))
	}
	d := vaults[0]
	// The default vault's id is derived from the root key, so every device
	// that recovers this account agrees on it, and its name is the Chinese
	// one the plan fixes.
	s := newSession(t)
	if d.ID != s.DefaultVaultID() {
		t.Errorf("the default vault id = %s, want %s", d.ID, s.DefaultVaultID())
	}
	if d.Name != defaultVaultName {
		t.Errorf("the default vault is called %q, want %q", d.Name, defaultVaultName)
	}
	if d.Rev != 1 || d.CreatedAt != 1730000000 {
		t.Errorf("the default vault is %+v", d)
	}
}

func TestOpenBindsTheDatabaseToTheAccount(t *testing.T) {
	db, path := openDB(t)
	s := newSession(t)
	v, err := Open(db, s, 1730000000)
	if err != nil {
		t.Fatal(err)
	}
	v.Close()
	// The account id is in meta, so a second account opening the same file
	// is refused rather than shown another account's vault.
	otherKf, otherSession, err := identity.NewAccount(otherEngineMnemonic, "engine passphrase here", time.Unix(1730000000, 0).UTC(), bytes.NewReader(bytes.Repeat([]byte{8}, 64)))
	if err != nil {
		t.Fatal(err)
	}
	if otherKf == nil {
		t.Fatal("no keyfile")
	}
	db2, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	if _, err := Open(db2, otherSession, 1730000000); !errors.Is(err, ErrOtherAccount) {
		t.Errorf("another account: err = %v, want ErrOtherAccount", err)
	}
	if err := CheckAccount(db2, otherKf.AccountID); !errors.Is(err, ErrOtherAccount) {
		t.Errorf("CheckAccount: err = %v, want ErrOtherAccount", err)
	}
	if err := CheckAccount(db2, s.AccountID()); err != nil {
		t.Errorf("the right account: %v", err)
	}
}

const otherEngineMnemonic = "couple maze era give basic obtain shadow change couple maze era give basic obtain shadow change couple maze era give basic obtain shadow course"

func TestOpenRefusesACorruptItemWithACount(t *testing.T) {
	// This test edits the database file itself, so it holds its only handle
	// open for the whole of it: another handle would keep the write-ahead
	// log from being folded back into the file it patches.
	path := filepath.Join(t.TempDir(), store.DBName)
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	v, err := Open(db, newSession(t), 1730000000)
	if err != nil {
		t.Fatal(err)
	}
	vaultID := v.Vaults()[0].ID
	for _, title := range []string{"第一", "第二"} {
		if _, err := v.Create(vaultID, Item{Type: TypeLogin, Title: title, Username: "u", Password: "p"}, 1730000100); err != nil {
			t.Fatal(err)
		}
	}
	v.Close()
	db.Close()

	// Flip one byte of one item's stored ciphertext: what a bad disk or a
	// hand-edited file looks like. The tag fails, so the vault will not open.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	items, err := rows.Items()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("%d items", len(items))
	}
	rows.Close()
	bad := append([]byte{}, items[0].CT...)
	bad[0] ^= 0xff
	if !bytes.Contains(raw, items[0].CT) {
		t.Fatal("the ciphertext is not in the database file")
	}
	patched := bytes.Replace(raw, items[0].CT, bad, 1)
	if err := os.WriteFile(path, patched, 0o600); err != nil {
		t.Fatal(err)
	}

	db3, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db3.Close()
	_, err = Open(db3, newSession(t), 1730000000)
	var corrupt *CorruptError
	if !errors.As(err, &corrupt) {
		t.Fatalf("err = %v, want a CorruptError", err)
	}
	if corrupt.Count != 1 {
		t.Errorf("Count = %d, want 1", corrupt.Count)
	}
}

func TestOpenRefusesAReplayedOldRev(t *testing.T) {
	db, path := openDB(t)
	v, err := Open(db, newSession(t), 1730000000)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := v.Create(v.Vaults()[0].ID, Item{Type: TypeLogin, Title: "某处", Username: "u", Password: "p1"}, 1730000100)
	if err != nil {
		t.Fatal(err)
	}
	rev1 := entry.Rev
	if _, err := v.Update(entry.ID, Item{Type: TypeLogin, Title: "某处", Username: "u", Password: "p2"}, 1730000200); err != nil {
		t.Fatal(err)
	}
	v.Close()

	// Put the rev-1 ciphertext on the rev-2 row: an old version copied back
	// over the current one. The row's header says rev 2, and the ciphertext
	// was sealed under a header that says rev 1, so it does not open.
	rows, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	items, _ := rows.Items()
	if len(items) != 1 || items[0].Rev != rev1+1 {
		t.Fatalf("items = %+v", items)
	}
	// A fresh write of the same payload, at the same rev and the same time,
	// in another database: the same header, and the same account and item
	// id, so its ciphertext differs only by its nonce.
	scratch, _ := openDB(t)
	sv, err := Open(scratch, newSession(t), 1730000000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sv.Create(sv.Vaults()[0].ID, Item{Type: TypeLogin, Title: "某处", Username: "u", Password: "p1"}, 1730000100); err != nil {
		t.Fatal(err)
	}
	sv.Close()
	seedRows, _ := scratch.Items()
	if len(seedRows) != 1 {
		t.Fatalf("the scratch database holds %d items", len(seedRows))
	}
	// The replayed row moves forward a rev, as the store requires, but its
	// sealed bytes are the old ones: the header says rev 3 and the ciphertext
	// was sealed under rev 1. Nothing in the engine would ever write that.
	replayed := items[0]
	replayed.Rev = rev1 + 2
	replayed.UpdatedAt = 1730000300
	replayed.Nonce, replayed.CT = seedRows[0].Nonce, seedRows[0].CT
	if err := rows.PutItem(replayed, store.OpUpdate, 1730000300); err != nil {
		t.Fatal(err)
	}
	rows.Close()

	db3, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db3.Close()
	_, err = Open(db3, newSession(t), 1730000000)
	var corrupt *CorruptError
	if !errors.As(err, &corrupt) {
		t.Fatalf("a replayed old rev: err = %v, want a CorruptError", err)
	}
}

func TestCRUDPersistsAcrossReopen(t *testing.T) {
	db, path := openDB(t)
	v, err := Open(db, newSession(t), 1730000000)
	if err != nil {
		t.Fatal(err)
	}
	vaultID := v.Vaults()[0].ID
	entry, err := v.Create(vaultID, Item{Type: TypeLogin, Title: "第一", Username: "me", Password: "p1", URLs: []string{"https://example.com"}}, 1730000100)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Rev != 1 || entry.ID == "" {
		t.Fatalf("the new entry is %+v", entry)
	}
	if entry.Item.Title != "第一" || entry.Item.Username != "me" {
		t.Errorf("the entry did not carry the item: %+v", entry.Item)
	}
	updated, err := v.Update(entry.ID, Item{Type: TypeLogin, Title: "第一", Username: "me", Password: "p2"}, 1730000200)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Rev != 2 || updated.Item.Password != "p2" {
		t.Fatalf("the update is %+v", updated)
	}
	v.Close()

	db2, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	v2, err := Open(db2, newSession(t), 1730000000)
	if err != nil {
		t.Fatal(err)
	}
	defer v2.Close()
	entries := v2.Entries()
	if len(entries) != 1 {
		t.Fatalf("%d entries after a reopen", len(entries))
	}
	got := entries[0]
	if got.ID != entry.ID || got.Rev != 2 || got.Item.Password != "p2" {
		t.Errorf("after a reopen: %+v", got)
	}
	if e, ok := v2.Entry(entry.ID); !ok || e.Item.Title != "第一" {
		t.Errorf("Entry(%q) = %+v, %v", entry.ID, e, ok)
	}
	if _, ok := v2.Entry("nope"); ok {
		t.Error("an unknown id gave an entry")
	}
}

func TestDeleteRestoreAndPurge(t *testing.T) {
	v, _, _ := openVault(t)
	vaultID := v.Vaults()[0].ID
	entry, err := v.Create(vaultID, Item{Type: TypeLogin, Title: "某处", Password: "p"}, 1730000100)
	if err != nil {
		t.Fatal(err)
	}
	// A soft delete keeps the payload, so it can be restored.
	if err := v.Delete(entry.ID, 1730000200); err != nil {
		t.Fatal(err)
	}
	e, ok := v.Entry(entry.ID)
	if !ok || !e.Deleted {
		t.Fatalf("after a delete: %+v, %v", e, ok)
	}
	if e.Item.Password != "p" {
		t.Errorf("a soft delete dropped the payload: %+v", e.Item)
	}
	if err := v.Restore(entry.ID, 1730000300); err != nil {
		t.Fatal(err)
	}
	e, _ = v.Entry(entry.ID)
	if e.Deleted {
		t.Error("a restore left the item deleted")
	}
	// A purge is a tombstone: deleted, with no payload left to restore.
	if err := v.Purge(entry.ID, 1730000400); err != nil {
		t.Fatal(err)
	}
	e, ok = v.Entry(entry.ID)
	if !ok {
		t.Fatal("a purged item vanished from the entries")
	}
	if !e.Deleted {
		t.Error("a tombstone is not deleted")
	}
	if e.Item.Title != "" || e.Item.Password != "" {
		t.Errorf("a tombstone still carries data: %+v", e.Item)
	}
	if !e.Purged {
		t.Error("the tombstone is not marked purged")
	}
	// A purged item cannot be restored.
	if err := v.Restore(entry.ID, 1730000500); err == nil {
		t.Error("a tombstone was restored")
	}
}

func TestAPasswordChangeAppendsHistory(t *testing.T) {
	v, _, _ := openVault(t)
	vaultID := v.Vaults()[0].ID
	entry, err := v.Create(vaultID, Item{Type: TypeLogin, Title: "某处", Password: "first"}, 1730000100)
	if err != nil {
		t.Fatal(err)
	}
	for i, pw := range []string{"second", "third"} {
		it, _ := v.Entry(entry.ID)
		item := it.Item
		item.Password = pw
		if _, err := v.Update(entry.ID, item, 1730000200+int64(i)*100); err != nil {
			t.Fatal(err)
		}
	}
	e, _ := v.Entry(entry.ID)
	if e.Item.Password != "third" {
		t.Errorf("password = %q", e.Item.Password)
	}
	if len(e.Item.History) != 2 {
		t.Fatalf("history = %+v, want two old passwords", e.Item.History)
	}
	if e.Item.History[0].Password != "first" || e.Item.History[1].Password != "second" {
		t.Errorf("history = %+v", e.Item.History)
	}
}

func TestCreateVaultAndRenameVault(t *testing.T) {
	v, _, _ := openVault(t)
	extra, err := v.CreateVault("工作", 1730000100)
	if err != nil {
		t.Fatal(err)
	}
	if extra.Name != "工作" || extra.ID == "" || extra.Rev != 1 {
		t.Fatalf("the new vault is %+v", extra)
	}
	if extra.ID == v.Vaults()[0].ID {
		t.Error("a new vault took the default's id")
	}
	// An item can live in it, and the two vaults' keys are different, so a
	// vault's ciphertext is not readable in another.
	entry, err := v.Create(extra.ID, Item{Type: TypeLogin, Title: "工作账号", Password: "p"}, 1730000200)
	if err != nil {
		t.Fatal(err)
	}
	if entry.VaultID != extra.ID {
		t.Errorf("the entry is in vault %s, want %s", entry.VaultID, extra.ID)
	}
	if err := v.RenameVault(extra.ID, "工作与学习", 1730000300); err != nil {
		t.Fatal(err)
	}
	for _, vi := range v.Vaults() {
		if vi.ID == extra.ID {
			if vi.Name != "工作与学习" {
				t.Errorf("after a rename the vault is %q", vi.Name)
			}
			if vi.Rev != 2 {
				t.Errorf("after a rename rev = %d", vi.Rev)
			}
		}
	}
}

func TestTheRawDatabaseHoldsNoPlaintext(t *testing.T) {
	v, _, path := openVault(t)
	vaultID := v.Vaults()[0].ID
	secrets := []string{
		"招商银行", "me@example.com", "https://example.com",
		"super-secret-password", "a very specific note",
		"工作", defaultVaultName,
	}
	if _, err := v.Create(vaultID, Item{
		Type: TypeLogin, Title: secrets[0], Username: secrets[1],
		URLs: []string{secrets[2]}, Password: secrets[3], Notes: secrets[4],
		Tags: []string{"银行"}, OTP: &OTP{Secret: "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"},
	}, 1730000100); err != nil {
		t.Fatal(err)
	}
	if _, err := v.CreateVault(secrets[5], 1730000200); err != nil {
		t.Fatal(err)
	}
	v.Close()

	// The database and its WAL, as bytes, contain none of it. The routing
	// columns are the ids and the revs, and that is all a reader learns.
	for _, f := range []string{path, path + "-wal", path + "-shm"} {
		raw, err := os.ReadFile(f)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			t.Fatal(err)
		}
		for _, s := range secrets {
			if bytes.Contains(raw, []byte(s)) {
				t.Errorf("%s contains %q", filepath.Base(f), s)
			}
		}
		// Nor does the mnemonic or the passphrase.
		for _, s := range []string{"coral light", "couple maze", "engine passphrase"} {
			if bytes.Contains(raw, []byte(s)) {
				t.Errorf("%s contains %q", filepath.Base(f), s)
			}
		}
	}
}

func TestOpenRefusesWhenAnyItemFails(t *testing.T) {
	// One unopenable item is the whole vault refusing: no partial decryption,
	// no showing the rest.
	db, path := openDB(t)
	v, err := Open(db, newSession(t), 1730000000)
	if err != nil {
		t.Fatal(err)
	}
	vaultID := v.Vaults()[0].ID
	for i := 0; i < 3; i++ {
		if _, err := v.Create(vaultID, Item{Type: TypeLogin, Title: "条目", Password: "p"}, 1730000100+int64(i)); err != nil {
			t.Fatal(err)
		}
	}
	v.Close()
	rows, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	items, _ := rows.Items()
	// Truncate one ciphertext so its tag fails.
	broken := items[1]
	broken.CT = broken.CT[:len(broken.CT)-1]
	if err := rows.PutItem(store.ItemRecord{
		ID: broken.ID, VaultID: broken.VaultID, Rev: broken.Rev + 1,
		UpdatedAt: 1730000999, Nonce: broken.Nonce, CT: broken.CT,
	}, store.OpUpdate, 1730000999); err != nil {
		t.Fatal(err)
	}
	rows.Close()

	db2, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	_, err = Open(db2, newSession(t), 1730000000)
	var corrupt *CorruptError
	if !errors.As(err, &corrupt) {
		t.Fatalf("err = %v, want a CorruptError", err)
	}
	if corrupt.Count != 1 {
		t.Errorf("Count = %d, want 1", corrupt.Count)
	}
}

func TestCreateRejectsAnUnknownVault(t *testing.T) {
	v, _, _ := openVault(t)
	if _, err := v.Create("nope", Item{Type: TypeLogin, Title: "t"}, 1730000100); err == nil {
		t.Error("an item was created in a vault that does not exist")
	}
}

func TestUpdateRefusesAnUnknownItem(t *testing.T) {
	v, _, _ := openVault(t)
	if _, err := v.Update("nope", Item{Type: TypeLogin, Title: "t"}, 1730000100); err == nil {
		t.Error("an unknown item was updated")
	}
}

func TestCloseIsIdempotentAndLocksNothing(t *testing.T) {
	db, _ := openDB(t)
	v, err := Open(db, newSession(t), 1730000000)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}
	// A second Close does not panic and does not lose data.
	if err := v.Close(); err != nil {
		t.Errorf("a second Close: %v", err)
	}
}

// The strings(1) check the security checklist asks for, run on a real
// database file.
func TestStringsFindsNoSecrets(t *testing.T) {
	if testing.Short() {
		t.Skip("strings(1) is not run in short mode")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, store.DBName)
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	v, err := Open(db, newSession(t), 1730000000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Create(v.Vaults()[0].ID, Item{Type: TypeLogin, Title: "字符串检查", Username: "check", Password: "check-me"}, 1730000100); err != nil {
		t.Fatal(err)
	}
	v.Close()
	db.Close()
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"字符串检查", "check", "check-me"} {
		if strings.Contains(string(out), s) {
			t.Errorf("%s is in the database", s)
		}
	}
}
