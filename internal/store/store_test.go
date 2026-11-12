package store

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func tempDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), DBName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestHomeHonoursCCPASSHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(HomeEnv, dir)
	got, err := Home()
	if err != nil {
		t.Fatal(err)
	}
	if got != dir {
		t.Errorf("Home() = %q, want %q", got, dir)
	}
	// The default path is under the user's config directory, and is not the
	// current directory.
	t.Setenv(HomeEnv, "")
	got, err = Home()
	if err != nil {
		t.Fatal(err)
	}
	if got == "" || got == "." {
		t.Errorf("the default home is %q", got)
	}
	if !strings.Contains(got, "bipPass") {
		t.Errorf("the default home %q does not name bipPass", got)
	}
}

func TestOpenCreatesTheFileWithOwnerOnlyPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DBName)
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the database is not there: %v", err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Errorf("mode = %#o, want 0600", mode)
		}
		// The keyfile's sibling directory is created owner-only too.
		sub := filepath.Join(dir, "fresh", DBName)
		db2, err := Open(sub)
		if err != nil {
			t.Fatal(err)
		}
		db2.Close()
		di, err := os.Stat(filepath.Dir(sub))
		if err != nil {
			t.Fatal(err)
		}
		if mode := di.Mode().Perm(); mode != 0o700 {
			t.Errorf("directory mode = %#o, want 0700", mode)
		}
	}
}

func TestOpenRunsInWALMode(t *testing.T) {
	db := tempDB(t)
	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(mode, "wal") {
		t.Errorf("journal_mode = %q, want wal", mode)
	}
	// A reader does not block the writer's transaction, which is what WAL
	// is for.
	var foreign int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&foreign); err != nil {
		t.Fatal(err)
	}
	_ = foreign
}

func TestPutItemAssignsRevsAndPersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DBName)
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	first := ItemRecord{ID: "item1", VaultID: "vault1", Rev: 1, UpdatedAt: 1700000000, Nonce: []byte("n"), CT: []byte("c")}
	if err := db.PutItem(first, OpCreate, 1700000000); err != nil {
		t.Fatal(err)
	}
	items, err := db.Items()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Rev != 1 {
		t.Fatalf("items = %+v", items)
	}
	// The oplog holds the operation.
	entries, err := db.Oplog()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Op != OpCreate || entries[0].ItemID != "item1" {
		t.Fatalf("oplog = %+v", entries)
	}

	// An update at rev 2 succeeds.
	second := first
	second.Rev, second.UpdatedAt, second.CT = 2, 1700000100, []byte("c2")
	if err := db.PutItem(second, OpUpdate, 1700000100); err != nil {
		t.Fatal(err)
	}
	items, _ = db.Items()
	if len(items) != 1 || items[0].Rev != 2 || string(items[0].CT) != "c2" {
		t.Fatalf("after an update: %+v", items)
	}
	entries, _ = db.Oplog()
	if len(entries) != 2 {
		t.Fatalf("oplog has %d entries, want 2", len(entries))
	}

	// Reopening the same path keeps both the item and the oplog.
	db.Close()
	db2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	items, err = db2.Items()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Rev != 2 || string(items[0].CT) != "c2" {
		t.Fatalf("after a reopen: %+v", items)
	}
	entries, _ = db2.Oplog()
	if len(entries) != 2 {
		t.Fatalf("after a reopen the oplog has %d entries", len(entries))
	}
}

func TestPutItemRefusesARevThatIsNotExistingPlusOne(t *testing.T) {
	db := tempDB(t)
	if err := db.PutItem(ItemRecord{ID: "i", VaultID: "v", Rev: 1, UpdatedAt: 1, Nonce: []byte("n"), CT: []byte("c")}, OpCreate, 1); err != nil {
		t.Fatal(err)
	}
	for _, rev := range []int64{0, 1, 3, 100, -1} {
		rec := ItemRecord{ID: "i", VaultID: "v", Rev: rev, UpdatedAt: 2, Nonce: []byte("n"), CT: []byte("c")}
		if err := db.PutItem(rec, OpUpdate, 2); !errors.Is(err, ErrConflict) {
			t.Errorf("rev %d: err = %v, want ErrConflict", rev, err)
		}
	}
	// The item is unchanged by the refused writes.
	items, _ := db.Items()
	if items[0].Rev != 1 || string(items[0].CT) != "c" {
		t.Errorf("a refused write changed the item: %+v", items[0])
	}
	// A new item must start at 1, not at 0 and not at 2.
	for _, rev := range []int64{0, 2} {
		rec := ItemRecord{ID: "new", VaultID: "v", Rev: rev, UpdatedAt: 2, Nonce: []byte("n"), CT: []byte("c")}
		if err := db.PutItem(rec, OpCreate, 2); !errors.Is(err, ErrConflict) {
			t.Errorf("a new item at rev %d: err = %v, want ErrConflict", rev, err)
		}
	}
	if _, err := db.Items(); err != nil {
		t.Fatal(err)
	}
	rows, _ := db.Items()
	for _, r := range rows {
		if r.ID == "new" {
			t.Error("a refused new item was written")
		}
	}
}

func TestPutItemRollsBackOnAnInvalidOp(t *testing.T) {
	db := tempDB(t)
	rec := ItemRecord{ID: "i", VaultID: "v", Rev: 1, UpdatedAt: 1, Nonce: []byte("n"), CT: []byte("c")}
	if err := db.PutItem(rec, "not-an-op", 1); err == nil {
		t.Fatal("an invalid op was accepted")
	}
	// The item and the oplog are written in one transaction, so neither is
	// there: no item without its log entry.
	items, err := db.Items()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("the item survived a rolled-back write: %+v", items)
	}
	entries, err := db.Oplog()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("the oplog entry survived a rolled-back write: %+v", entries)
	}
}

func TestPutVaultAndVaults(t *testing.T) {
	db := tempDB(t)
	rec := VaultRecord{ID: "v1", Rev: 1, UpdatedAt: 1700000000, CreatedAt: 1700000000, Nonce: []byte("n"), CT: []byte("c")}
	if err := db.PutVault(rec); err != nil {
		t.Fatal(err)
	}
	vaults, err := db.Vaults()
	if err != nil {
		t.Fatal(err)
	}
	if len(vaults) != 1 || vaults[0].ID != "v1" || vaults[0].Rev != 1 {
		t.Fatalf("vaults = %+v", vaults)
	}
	// A rename is a new rev on the same row.
	rec.Rev, rec.UpdatedAt, rec.CT = 2, 1700000100, []byte("c2")
	if err := db.PutVault(rec); err != nil {
		t.Fatal(err)
	}
	vaults, _ = db.Vaults()
	if len(vaults) != 1 || vaults[0].Rev != 2 || string(vaults[0].CT) != "c2" {
		t.Fatalf("after a rename: %+v", vaults)
	}
	if vaults[0].CreatedAt != 1700000000 {
		t.Errorf("createdAt = %d, want it kept", vaults[0].CreatedAt)
	}
	// A second vault is a second row.
	if err := db.PutVault(VaultRecord{ID: "v2", Rev: 1, UpdatedAt: 1700000200, CreatedAt: 1700000200, Nonce: []byte("n"), CT: []byte("c")}); err != nil {
		t.Fatal(err)
	}
	vaults, _ = db.Vaults()
	if len(vaults) != 2 {
		t.Fatalf("%d vaults", len(vaults))
	}
}

func TestMetaRoundTrip(t *testing.T) {
	db := tempDB(t)
	// An unset key is not an error: the caller is asking whether a
	// preference is there.
	if _, err := db.Meta("nope"); err == nil {
		t.Error("an unset key returned a value")
	}
	for _, kv := range [][2]string{
		{"accountId", "abc"},
		{"autoLockMinutes", "5"},
		{"a key with spaces", "a value with = and | and \"quotes\""},
	} {
		if err := db.SetMeta(kv[0], kv[1]); err != nil {
			t.Fatalf("SetMeta(%q): %v", kv[0], err)
		}
		got, err := db.Meta(kv[0])
		if err != nil {
			t.Fatalf("Meta(%q): %v", kv[0], err)
		}
		if got != kv[1] {
			t.Errorf("Meta(%q) = %q, want %q", kv[0], got, kv[1])
		}
	}
	// A second set overwrites.
	if err := db.SetMeta("autoLockMinutes", "15"); err != nil {
		t.Fatal(err)
	}
	if got, _ := db.Meta("autoLockMinutes"); got != "15" {
		t.Errorf("after a second set = %q, want 15", got)
	}
}

func TestCloseAndReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DBName)
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SetMeta("k", "v"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	// A second Close is harmless.
	if err := db.Close(); err != nil {
		t.Errorf("a second Close: %v", err)
	}
	db2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	if got, err := db2.Meta("k"); err != nil || got != "v" {
		t.Errorf("after a reopen: %q, %v", got, err)
	}
}

func TestOpenRefusesAnUnusablePath(t *testing.T) {
	// A directory cannot be a database.
	dir := t.TempDir()
	if _, err := Open(dir); err == nil {
		t.Error("a directory opened as a database")
	}
}

func TestKeyfileNameAndDBName(t *testing.T) {
	if KeyfileName != "keyfile.json" {
		t.Errorf("KeyfileName = %q", KeyfileName)
	}
	if DBName != "vault.db" {
		t.Errorf("DBName = %q", DBName)
	}
}

func TestOpsAreTheFiveThePlanNames(t *testing.T) {
	for _, op := range []string{OpCreate, OpUpdate, OpDelete, OpRestore, OpPurge} {
		if op == "" {
			t.Error("an empty op constant")
		}
	}
	// The five are distinct, so the oplog cannot be read ambiguously.
	seen := map[string]bool{}
	for _, op := range []string{OpCreate, OpUpdate, OpDelete, OpRestore, OpPurge} {
		if seen[op] {
			t.Errorf("%s is not distinct", op)
		}
		seen[op] = true
	}
}

func TestItemsCarriesDeletedAndRoutingColumns(t *testing.T) {
	db := tempDB(t)
	// A tombstone is a row like any other: deleted, with a payload the
	// engine decides what to do with.
	if err := db.PutItem(ItemRecord{ID: "gone", VaultID: "v", Rev: 1, UpdatedAt: 1, Deleted: true, Nonce: []byte("n"), CT: nil}, OpDelete, 1); err != nil {
		t.Fatal(err)
	}
	items, err := db.Items()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || !items[0].Deleted || len(items[0].CT) != 0 {
		t.Fatalf("the tombstone is %+v", items)
	}
	// A restore is the same row back at the next rev, not a new one.
	if err := db.PutItem(ItemRecord{ID: "gone", VaultID: "v", Rev: 2, UpdatedAt: 2, Nonce: []byte("n"), CT: []byte("c")}, OpRestore, 2); err != nil {
		t.Fatal(err)
	}
	items, _ = db.Items()
	if len(items) != 1 || items[0].Deleted || string(items[0].CT) != "c" {
		t.Fatalf("after a restore: %+v", items)
	}
	entries, _ := db.Oplog()
	if len(entries) != 2 || entries[1].Op != OpRestore || entries[1].Rev != 2 {
		t.Fatalf("oplog = %+v", entries)
	}
}

func TestOplogSequenceIncreases(t *testing.T) {
	db := tempDB(t)
	for i := int64(1); i <= 5; i++ {
		rec := ItemRecord{ID: "i", VaultID: "v", Rev: i, UpdatedAt: i, Nonce: []byte("n"), CT: []byte("c")}
		if err := db.PutItem(rec, OpUpdate, i); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := db.Oplog()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 5 {
		t.Fatalf("%d entries", len(entries))
	}
	for i, e := range entries {
		if e.Seq != int64(i+1) {
			t.Errorf("entry %d has seq %d", i, e.Seq)
		}
		if e.Rev != int64(i+1) {
			t.Errorf("entry %d has rev %d", i, e.Rev)
		}
	}
}
