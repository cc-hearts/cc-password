// Package store is the local ciphertext store: a SQLite database on the
// user's own machine holding items, vaults and a merge log, and nothing that
// can be read without the root key.
//
// It knows about routing — which item belongs to which vault, what rev it is
// at, whether it is deleted — and nothing else. Every title, url, username and
// password is inside a sealed blob this package hands out without opening.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite" // the pure-Go driver: no cgo, one binary
)

// DBName is the file the store lives in, next to the keyfile.
const DBName = "vault.db"

// KeyfileName is the file the account's wrapped root key lives in.
const KeyfileName = "keyfile.json"

// HomeEnv is the environment variable that moves the whole store, for tests
// and for a user who wants it on a removable drive.
const HomeEnv = "CCPASS_HOME"

// appDir is the directory the store lives in under the user's config
// directory.
const appDir = "ccpass"

// The permissions a store is created with. The database holds ciphertext, but
// it also holds the shape of the user's data and the fact that they use this
// app, so it is no more readable than the keyfile beside it.
const (
	dbMode  os.FileMode = 0o600
	dirMode os.FileMode = 0o700
)

// The five operations the oplog records. Every write to an item is one of
// them, and the log is what a merge reads to decide what changed.
const (
	OpCreate  = "create"
	OpUpdate  = "update"
	OpDelete  = "delete"
	OpRestore = "restore"
	OpPurge   = "purge"
)

// ErrConflict is the error of a write that does not continue the row's rev.
// It is the store refusing to lose an edit: a rev that skips, repeats or goes
// backwards is a second device writing what this one has not seen yet.
var ErrConflict = errors.New("store: the item has changed on another device")

// schema is the local database, created on open. The plaintext columns are
// the routing metadata the server also sees, and nothing more.
const schema = `
CREATE TABLE IF NOT EXISTS meta (
  key TEXT PRIMARY KEY, value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS vaults (
  id TEXT PRIMARY KEY, rev INTEGER NOT NULL, updated_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL, nonce BLOB NOT NULL, ct BLOB NOT NULL
);
CREATE TABLE IF NOT EXISTS items (
  id TEXT PRIMARY KEY, vault_id TEXT NOT NULL,
  rev INTEGER NOT NULL, updated_at INTEGER NOT NULL, deleted INTEGER NOT NULL,
  nonce BLOB, ct BLOB
);
CREATE TABLE IF NOT EXISTS oplog (
  seq INTEGER PRIMARY KEY AUTOINCREMENT,
  item_id TEXT NOT NULL, rev INTEGER NOT NULL, op TEXT NOT NULL, at INTEGER NOT NULL
);
`

// ItemRecord is a stored item: the routing columns in the clear and the
// sealed payload this package never opens.
type ItemRecord struct {
	ID        string
	VaultID   string
	Rev       int64
	UpdatedAt int64
	Deleted   bool
	Nonce     []byte
	CT        []byte
}

// VaultRecord is a stored vault: its id, its revs, and its sealed name.
type VaultRecord struct {
	ID        string
	Rev       int64
	UpdatedAt int64
	CreatedAt int64
	Nonce     []byte
	CT        []byte
}

// OplogEntry is one recorded change, in the order it was written.
type OplogEntry struct {
	Seq    int64
	ItemID string
	Rev    int64
	Op     string
	At     int64
}

// DB is an open store. It is safe for one writer at a time, which is what a
// single-user local database needs.
type DB struct {
	db *sql.DB
}

// Home is the directory the keyfile and the database live in: CCPASS_HOME
// when it is set, otherwise the user's config directory. The directory is
// created owner-only the first time something is written into it.
func Home() (string, error) {
	if h := os.Getenv(HomeEnv); h != "" {
		return h, nil
	}
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("store: finding the config directory: %w", err)
	}
	return filepath.Join(cfg, appDir), nil
}

// Path is where the database lives: the keyfile beside it, in Home.
func Path() (string, error) {
	h, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, DBName), nil
}

// Open opens the store at path, creating it with the schema above if it is
// new. The file is written 0600 in a 0700 directory, and the database runs in
// WAL mode so a reader never blocks the writer.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		return nil, err
	}
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)" +
		"&_pragma=foreign_keys(1)&_pragma=synchronous(FULL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: opening %s: %w", path, err)
	}
	// One connection: the store is local and single-user, and a pool of
	// them would let a transaction's writes land out of order.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: opening %s: %w", path, err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: creating the schema: %w", err)
	}
	if err := os.Chmod(path, dbMode); err != nil {
		db.Close()
		return nil, err
	}
	return &DB{db: db}, nil
}

// Close closes the store. A second call is harmless, so a caller can close in
// a defer and again on an error path.
func (d *DB) Close() error {
	if err := d.db.Close(); err != nil {
		return fmt.Errorf("store: closing: %w", err)
	}
	return nil
}

// validOp reports whether op is one of the five the oplog records.
func validOp(op string) bool {
	switch op {
	case OpCreate, OpUpdate, OpDelete, OpRestore, OpPurge:
		return true
	}
	return false
}

// PutItem writes an item and its oplog entry in one transaction, so a crash
// can never leave an item whose change was never recorded.
//
// The rev must be exactly the row's current rev plus one, or 1 for a new
// item: anything else is ErrConflict, and neither the item nor the log entry
// is written. A rev that skips or repeats is another device's edit arriving
// out of order, and overwriting it would silently drop one of the two.
func (d *DB) PutItem(rec ItemRecord, op string, at int64) error {
	if rec.ID == "" || rec.VaultID == "" {
		return fmt.Errorf("store: an item needs an id and a vault id")
	}
	if !validOp(op) {
		return fmt.Errorf("store: %q is not an operation", op)
	}
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("store: beginning a transaction: %w", err)
	}
	defer tx.Rollback()
	var current int64
	err = tx.QueryRow(`SELECT rev FROM items WHERE id = ?`, rec.ID).Scan(&current)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if rec.Rev != 1 {
			return fmt.Errorf("%w: a new item starts at rev 1, not %d", ErrConflict, rec.Rev)
		}
		if _, err := tx.Exec(
			`INSERT INTO items (id, vault_id, rev, updated_at, deleted, nonce, ct)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			rec.ID, rec.VaultID, rec.Rev, rec.UpdatedAt, boolToInt(rec.Deleted), rec.Nonce, rec.CT,
		); err != nil {
			return fmt.Errorf("store: writing the item: %w", err)
		}
	case err != nil:
		return fmt.Errorf("store: reading the item: %w", err)
	default:
		if rec.Rev != current+1 {
			return fmt.Errorf("%w: rev %d is not %d + 1", ErrConflict, rec.Rev, current)
		}
		if _, err := tx.Exec(
			`UPDATE items SET vault_id = ?, rev = ?, updated_at = ?, deleted = ?,
			 nonce = ?, ct = ? WHERE id = ?`,
			rec.VaultID, rec.Rev, rec.UpdatedAt, boolToInt(rec.Deleted),
			rec.Nonce, rec.CT, rec.ID,
		); err != nil {
			return fmt.Errorf("store: writing the item: %w", err)
		}
	}
	if _, err := tx.Exec(
		`INSERT INTO oplog (item_id, rev, op, at) VALUES (?, ?, ?, ?)`,
		rec.ID, rec.Rev, op, at,
	); err != nil {
		return fmt.Errorf("store: writing the oplog: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: committing: %w", err)
	}
	return nil
}

// PutItemDeleted removes an item outright, with no oplog entry: a purge is the
// user asking for the ciphertext to be gone, not tombstoned.
func (d *DB) PutItemDeleted(id string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("store: beginning a transaction: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM items WHERE id = ?`, id); err != nil {
		return fmt.Errorf("store: removing the item: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM oplog WHERE item_id = ?`, id); err != nil {
		return fmt.Errorf("store: removing the oplog: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: committing: %w", err)
	}
	return nil
}

// Items is every stored item, in id order. The ciphertext is handed out as it
// is stored: opening it is the vault engine's job, not the store's. A
// tombstone's ct is empty, so the column is not NOT NULL: a purge is the
// user asking for the ciphertext to be gone.
func (d *DB) Items() ([]ItemRecord, error) {
	rows, err := d.db.Query(
		`SELECT id, vault_id, rev, updated_at, deleted, nonce, ct FROM items ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("store: reading the items: %w", err)
	}
	defer rows.Close()
	var out []ItemRecord
	for rows.Next() {
		var r ItemRecord
		var deleted int
		if err := rows.Scan(&r.ID, &r.VaultID, &r.Rev, &r.UpdatedAt, &deleted, &r.Nonce, &r.CT); err != nil {
			return nil, fmt.Errorf("store: reading an item: %w", err)
		}
		r.Deleted = deleted != 0
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: reading the items: %w", err)
	}
	return out, nil
}

// PutVault writes a vault's sealed name. Its rev is the caller's to keep:
// the vault engine is the one that knows what a vault's next rev is.
func (d *DB) PutVault(rec VaultRecord) error {
	if rec.ID == "" {
		return fmt.Errorf("store: a vault needs an id")
	}
	if _, err := d.db.Exec(
		`INSERT INTO vaults (id, rev, updated_at, created_at, nonce, ct)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET rev = excluded.rev, updated_at = excluded.updated_at,
		   nonce = excluded.nonce, ct = excluded.ct`,
		rec.ID, rec.Rev, rec.UpdatedAt, rec.CreatedAt, rec.Nonce, rec.CT,
	); err != nil {
		return fmt.Errorf("store: writing the vault: %w", err)
	}
	return nil
}

// Vaults is every stored vault, in id order.
func (d *DB) Vaults() ([]VaultRecord, error) {
	rows, err := d.db.Query(
		`SELECT id, rev, updated_at, created_at, nonce, ct FROM vaults ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("store: reading the vaults: %w", err)
	}
	defer rows.Close()
	var out []VaultRecord
	for rows.Next() {
		var r VaultRecord
		if err := rows.Scan(&r.ID, &r.Rev, &r.UpdatedAt, &r.CreatedAt, &r.Nonce, &r.CT); err != nil {
			return nil, fmt.Errorf("store: reading a vault: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: reading the vaults: %w", err)
	}
	return out, nil
}

// Meta reads a non-secret preference. An unset key is an error the caller is
// expected to treat as "not there".
func (d *DB) Meta(key string) (string, error) {
	var v string
	err := d.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("store: %s is not set", key)
		}
		return "", fmt.Errorf("store: reading %s: %w", key, err)
	}
	return v, nil
}

// SetMeta writes a non-secret preference. Nothing that unlocks anything ever
// goes here: the store holds no passphrase, no mnemonic and no key.
func (d *DB) SetMeta(key, value string) error {
	if _, err := d.db.Exec(
		`INSERT INTO meta (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	); err != nil {
		return fmt.Errorf("store: writing %s: %w", key, err)
	}
	return nil
}

// Oplog is every recorded change, oldest first: what a merge reads to find
// what has changed since the last sync.
func (d *DB) Oplog() ([]OplogEntry, error) {
	rows, err := d.db.Query(
		`SELECT seq, item_id, rev, op, at FROM oplog ORDER BY seq`)
	if err != nil {
		return nil, fmt.Errorf("store: reading the oplog: %w", err)
	}
	defer rows.Close()
	var out []OplogEntry
	for rows.Next() {
		var e OplogEntry
		if err := rows.Scan(&e.Seq, &e.ItemID, &e.Rev, &e.Op, &e.At); err != nil {
			return nil, fmt.Errorf("store: reading an oplog entry: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: reading the oplog: %w", err)
	}
	return out, nil
}

// Exec runs a statement that has no result, for the caller's own schema work.
func (d *DB) Exec(query string, args ...any) error {
	if _, err := d.db.Exec(query, args...); err != nil {
		return fmt.Errorf("store: %s: %w", firstWords(query), err)
	}
	return nil
}

// QueryRow runs a query expected to return one row.
func (d *DB) QueryRow(query string, args ...any) *sql.Row {
	return d.db.QueryRow(query, args...)
}

// firstWords is the start of a statement, for an error message.
func firstWords(q string) string {
	q = strings.Join(strings.Fields(q), " ")
	if len(q) > 40 {
		return q[:40]
	}
	return q
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
