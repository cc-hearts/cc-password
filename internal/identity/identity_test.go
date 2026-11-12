package identity

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"ccpass/internal/vaultcrypto"
)

// testMnemonic is the BIP39 mnemonic of zero entropy, spelled out by the
// vectors file rather than by hand.
func testMnemonic(t *testing.T) string {
	t.Helper()
	m, err := vaultcrypto.GenerateMnemonic(strings.NewReader(strings.Repeat("\x00", 32)))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

const testPassphrase = "correct horse battery staple"

// otherMnemonic is a second, different mnemonic: the account a keyfile edited
// into another identity would belong to.
func otherMnemonic(t *testing.T) string {
	t.Helper()
	m, err := vaultcrypto.GenerateMnemonic(strings.NewReader(strings.Repeat("\x01", 32)))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func tempPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "keyfile.json")
}

// aReader is enough deterministic bytes for a keyfile: 16 B of salt and 24 B
// of nonce, twice over for the re-wrap after a passphrase change.
func aReader() *strings.Reader {
	b := make([]byte, 128)
	for i := range b {
		b[i] = byte(i)
	}
	return strings.NewReader(string(b))
}

func TestNewAccountWritesAKeyfileThatUnlocks(t *testing.T) {
	path := tempPath(t)
	now := time.Unix(1730000000, 0).UTC()
	kf, session, err := NewAccount(testMnemonic(t), testPassphrase, now, aReader())
	if err != nil {
		t.Fatal(err)
	}
	if session == nil {
		t.Fatal("no session")
	}
	if err := SaveKeyfile(path, kf); err != nil {
		t.Fatal(err)
	}
	// The file on disk is exactly the JSON of the keyfile, at 0600.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var back Keyfile
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("the keyfile is not JSON: %v", err)
	}
	if back.Version != 1 {
		t.Errorf("version = %d", back.Version)
	}
	if back.AccountID != session.AccountID() {
		t.Errorf("accountId = %s, session says %s", back.AccountID, session.AccountID())
	}
	if back.VaultID != session.DefaultVaultID() {
		t.Errorf("vaultId = %s, session says %s", back.VaultID, session.DefaultVaultID())
	}
	if back.CreatedAt != now.Unix() {
		t.Errorf("createdAt = %d, want %d", back.CreatedAt, now.Unix())
	}
	if back.Argon2.MKiB != vaultcrypto.DefaultArgon2.MKiB ||
		back.Argon2.T != vaultcrypto.DefaultArgon2.T ||
		back.Argon2.P != vaultcrypto.DefaultArgon2.P {
		t.Errorf("argon2 = %+v, want the defaults", back.Argon2)
	}
	// The Argon2 block keeps the plan's field names, not Go's.
	if !strings.Contains(string(raw), `"mKiB"`) ||
		!strings.Contains(string(raw), `"t"`) ||
		!strings.Contains(string(raw), `"p"`) {
		t.Error("the argon2 block is not spelled the plan's way")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Errorf("mode = %#o, want 0600", mode)
		}
	}
	// No temporary file is left behind.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("%d files in the directory, want 1", len(entries))
	}

	// Reloading the file and unlocking with the right passphrase gives the
	// same account and vault.
	loaded, err := LoadKeyfile(path)
	if err != nil {
		t.Fatal(err)
	}
	unlocked, err := Unlock(loaded, testPassphrase)
	if err != nil {
		t.Fatal(err)
	}
	if unlocked.AccountID() != session.AccountID() {
		t.Errorf("recovered account = %s, want %s", unlocked.AccountID(), session.AccountID())
	}
	if unlocked.DefaultVaultID() != session.DefaultVaultID() {
		t.Errorf("recovered vault = %s, want %s", unlocked.DefaultVaultID(), session.DefaultVaultID())
	}
	if string(unlocked.VaultKey(session.DefaultVaultID())) != string(session.VaultKey(session.DefaultVaultID())) {
		t.Error("the vault key differs after a reload")
	}
}

func TestUnlockWithAWrongPassphrase(t *testing.T) {
	path := tempPath(t)
	kf, _, err := NewAccount(testMnemonic(t), testPassphrase, time.Unix(1730000000, 0).UTC(), aReader())
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveKeyfile(path, kf); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadKeyfile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, pw := range []string{
		"",
		testPassphrase + "x",
		strings.ToUpper(testPassphrase),
		" " + testPassphrase,
		testPassphrase + " ",
	} {
		if _, err := Unlock(loaded, pw); !errors.Is(err, ErrUnlock) {
			t.Errorf("Unlock(%q): err = %v, want ErrUnlock", pw, err)
		}
	}
}

// tamper returns a copy of the keyfile with one field changed, the way an
// edited file on disk would.
func tamper(t *testing.T, kf *Keyfile, f func(*Keyfile)) *Keyfile {
	t.Helper()
	cp := *kf
	f(&cp)
	return &cp
}

func TestUnlockRefusesATamperedKeyfile(t *testing.T) {
	kf, session, err := NewAccount(testMnemonic(t), testPassphrase, time.Unix(1730000000, 0).UTC(), aReader())
	if err != nil {
		t.Fatal(err)
	}
	// The account id and the default vault id are re-derived on unlock, so
	// changing either is detected even though neither is authenticated by
	// the AAD.
	otherAccount, _, err := NewAccount(otherMnemonic(t), testPassphrase, time.Unix(1730000000, 0).UTC(), aReader())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		f    func(*Keyfile)
	}{
		{"wrapped root", func(k *Keyfile) {
			k.WrappedRoot = flipLast(k.WrappedRoot)
		}},
		{"nonce", func(k *Keyfile) {
			k.WrapNonce = flipLast(k.WrapNonce)
		}},
		{"salt", func(k *Keyfile) {
			k.Argon2.Salt = flipLast(k.Argon2.Salt)
		}},
		{"account id", func(k *Keyfile) {
			k.AccountID = otherAccount.AccountID
		}},
		{"vault id", func(k *Keyfile) {
			k.VaultID = otherAccount.VaultID
		}},
	} {
		bad := tamper(t, kf, c.f)
		if _, err := Unlock(bad, testPassphrase); !errors.Is(err, ErrUnlock) {
			t.Errorf("%s: err = %v, want ErrUnlock", c.name, err)
		}
	}
	// The untampered keyfile still opens.
	if _, err := Unlock(kf, testPassphrase); err != nil {
		t.Fatalf("the original keyfile stopped opening: %v", err)
	}
	// The session the good keyfile gave still works.
	if session == nil {
		t.Fatal("no session")
	}
}

func flipLast(s string) string {
	if s == "" {
		return "A"
	}
	last := s[len(s)-1]
	if last == 'A' {
		return s[:len(s)-1] + "B"
	}
	return s[:len(s)-1] + "A"
}

func TestUnlockRefusesParametersOutOfRange(t *testing.T) {
	kf, _, err := NewAccount(testMnemonic(t), testPassphrase, time.Unix(1730000000, 0).UTC(), aReader())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		f    func(*argon2Block)
	}{
		{"memory below the default", func(p *argon2Block) { p.MKiB = 1024 }},
		{"passes below the default", func(p *argon2Block) { p.T = 1 }},
		{"lanes below the default", func(p *argon2Block) { p.P = 0 }},
		{"memory past the cap", func(p *argon2Block) { p.MKiB = 5 << 20 }},
		{"passes past the cap", func(p *argon2Block) { p.T = 128 }},
		{"lanes past the cap", func(p *argon2Block) { p.P = 32 }},
		{"a blank salt", func(p *argon2Block) { p.Salt = "" }},
		{"a salt that is not base64url", func(p *argon2Block) { p.Salt = "not base64url!" }},
	} {
		bad := tamper(t, kf, func(k *Keyfile) { c.f(&k.Argon2) })
		if _, err := Unlock(bad, testPassphrase); !errors.Is(err, ErrUnlock) {
			t.Errorf("%s: err = %v, want ErrUnlock", c.name, err)
		}
	}
	// A keyfile with no argon2 block at all is refused rather than run at
	// the defaults.
	empty := tamper(t, kf, func(k *Keyfile) { k.Argon2 = argon2Block{} })
	if _, err := Unlock(empty, testPassphrase); !errors.Is(err, ErrUnlock) {
		t.Errorf("empty parameters: err = %v, want ErrUnlock", err)
	}
}

func TestUnlockRefusesAnUnsupportedVersion(t *testing.T) {
	path := tempPath(t)
	kf, _, err := NewAccount(testMnemonic(t), testPassphrase, time.Unix(1730000000, 0).UTC(), aReader())
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []int{0, 2, 99, -1} {
		bad := tamper(t, kf, func(k *Keyfile) { k.Version = v })
		if _, err := Unlock(bad, testPassphrase); !errors.Is(err, ErrKeyfileVersion) {
			t.Errorf("version %d: err = %v, want ErrKeyfileVersion", v, err)
		}
	}
	// A file on disk with an unsupported version is refused on load, before
	// any Argon2 runs.
	bad := tamper(t, kf, func(k *Keyfile) { k.Version = 7 })
	if err := SaveKeyfile(path, bad); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadKeyfile(path); !errors.Is(err, ErrKeyfileVersion) {
		t.Errorf("load: err = %v, want ErrKeyfileVersion", err)
	}
}

func TestCheckPassphrase(t *testing.T) {
	for _, c := range []struct {
		in   string
		want bool
	}{
		{"", false},
		{"a", false},
		{"1234567", false},
		{"12345678", true},
		{"correct horse battery staple", true},
		{"\u4e2d\u6587\u53e3\u4ee4\u5bc6\u7801\u957f\u5ea6", true}, // 8 CJK runes
	} {
		err := CheckPassphrase(c.in)
		if c.want && err != nil {
			t.Errorf("CheckPassphrase(%q): %v", c.in, err)
		}
		if !c.want && !errors.Is(err, ErrPassphraseShort) {
			t.Errorf("CheckPassphrase(%q): err = %v, want ErrPassphraseShort", c.in, err)
		}
	}
	// The minimum is in runes, not bytes: eight CJK characters are
	// twenty-four bytes but eight runes, and are enough.
	if err := CheckPassphrase("\u4e2d\u6587\u53e3\u4ee4\u5bc6\u7801\u957f\u5ea6"); err != nil {
		t.Errorf("eight CJK runes: %v", err)
	}
	if err := CheckPassphrase("\u4e2d\u6587\u53e3"); !errors.Is(err, ErrPassphraseShort) {
		t.Errorf("three CJK runes: err = %v, want ErrPassphraseShort", err)
	}
	if err := CheckPassphrase(strings.Repeat("\u4e2d", 7)); !errors.Is(err, ErrPassphraseShort) {
		t.Errorf("seven CJK runes: err = %v, want ErrPassphraseShort", err)
	}
}

func TestNewAccountRefusesAShortPassphrase(t *testing.T) {
	for _, pw := range []string{"", "short", "1234567"} {
		if _, _, err := NewAccount(testMnemonic(t), pw, time.Unix(1730000000, 0).UTC(), aReader()); !errors.Is(err, ErrPassphraseShort) {
			t.Errorf("NewAccount(%q): err = %v, want ErrPassphraseShort", pw, err)
		}
	}
}

func TestNewAccountRefusesABadMnemonic(t *testing.T) {
	for _, m := range []string{
		"",
		"not a mnemonic at all",
		"abandon abandon abandon",
		strings.Replace(testMnemonic(t), "art", "abandon", 1),
	} {
		if _, _, err := NewAccount(m, testPassphrase, time.Unix(1730000000, 0).UTC(), aReader()); err == nil {
			t.Errorf("NewAccount accepted a bad mnemonic")
		}
	}
	// A messy spelling of the same mnemonic is the same account.
	tidy := testMnemonic(t)
	messy := "  " + strings.ReplaceAll(strings.ToUpper(tidy), " ", "   ") + "  \n"
	kf, session, err := NewAccount(messy, testPassphrase, time.Unix(1730000000, 0).UTC(), aReader())
	if err != nil {
		t.Fatalf("a messy mnemonic was refused: %v", err)
	}
	clean, _, err := NewAccount(tidy, testPassphrase, time.Unix(1730000000, 0).UTC(), aReader())
	if err != nil {
		t.Fatal(err)
	}
	if kf.AccountID != clean.AccountID || kf.VaultID != clean.VaultID {
		t.Errorf("a messy spelling gave a different account: %s vs %s", kf.AccountID, clean.AccountID)
	}
	if session == nil {
		t.Fatal("no session")
	}
}

// The same mnemonic and passphrase on two devices give the same account and
// the same default vault, so sync does not fork.
func TestRecoveryGivesTheSameAccount(t *testing.T) {
	m := testMnemonic(t)
	first, firstSession, err := NewAccount(m, testPassphrase, time.Unix(1730000000, 0).UTC(), aReader())
	if err != nil {
		t.Fatal(err)
	}
	second, secondSession, err := NewAccount(m, testPassphrase, time.Unix(1760000000, 0).UTC(), aReader())
	if err != nil {
		t.Fatal(err)
	}
	if first.AccountID != second.AccountID {
		t.Errorf("account ids differ: %s vs %s", first.AccountID, second.AccountID)
	}
	if first.VaultID != second.VaultID {
		t.Errorf("default vault ids differ: %s vs %s", first.VaultID, second.VaultID)
	}
	if firstSession.AccountID() != secondSession.AccountID() ||
		firstSession.DefaultVaultID() != secondSession.DefaultVaultID() {
		t.Error("the sessions disagree")
	}
	// A different passphrase is a different wrapped root but the same
	// account: the passphrase only guards the file, not the identity.
	third, _, err := NewAccount(m, testPassphrase+"other", time.Unix(1730000000, 0).UTC(), aReader())
	if err != nil {
		t.Fatal(err)
	}
	if third.AccountID != first.AccountID {
		t.Errorf("a different passphrase changed the account: %s vs %s", third.AccountID, first.AccountID)
	}
	if third.WrappedRoot == first.WrappedRoot {
		t.Error("a different passphrase produced the same wrapped root")
	}
}

func TestSessionLockZeroesTheRoot(t *testing.T) {
	_, session, err := NewAccount(testMnemonic(t), testPassphrase, time.Unix(1730000000, 0).UTC(), aReader())
	if err != nil {
		t.Fatal(err)
	}
	if session.Locked() {
		t.Fatal("a fresh session is locked")
	}
	before := session.VaultKey(session.DefaultVaultID())
	if len(before) != vaultcrypto.KeySize {
		t.Fatalf("vault key is %d bytes", len(before))
	}
	session.Lock()
	if !session.Locked() {
		t.Error("Lock did not lock the session")
	}
	after := session.VaultKey(session.DefaultVaultID())
	if len(after) != 0 {
		t.Errorf("a locked session still hands out a %d-byte key", len(after))
	}
	// Locking twice is harmless.
	session.Lock()
}

func TestErrorTextCarriesNoPassphrase(t *testing.T) {
	path := tempPath(t)
	kf, _, err := NewAccount(testMnemonic(t), testPassphrase, time.Unix(1730000000, 0).UTC(), aReader())
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveKeyfile(path, kf); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadKeyfile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Unlock(loaded, "a wrong passphrase entirely")
	if err == nil {
		t.Fatal("a wrong passphrase unlocked the keyfile")
	}
	if strings.Contains(err.Error(), "wrong passphrase") ||
		strings.Contains(err.Error(), testPassphrase) ||
		strings.Contains(err.Error(), kf.WrappedRoot) {
		t.Errorf("the error text carries the input or the secret: %v", err)
	}
	// Nor does the keyfile text.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), testPassphrase) {
		t.Error("the passphrase is in the keyfile")
	}
	for _, word := range strings.Fields(testMnemonic(t)) {
		if strings.Contains(string(raw), word) {
			t.Errorf("the mnemonic is in the keyfile: %q", word)
		}
	}
}

func TestSaveKeyfileIsAtomic(t *testing.T) {
	path := tempPath(t)
	kf, _, err := NewAccount(testMnemonic(t), testPassphrase, time.Unix(1730000000, 0).UTC(), aReader())
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveKeyfile(path, kf); err != nil {
		t.Fatal(err)
	}
	// A second save over the same path leaves exactly one file, with the new
	// contents.
	later := tamper(t, kf, func(k *Keyfile) { k.CreatedAt = 1760000000 })
	if err := SaveKeyfile(path, later); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadKeyfile(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.CreatedAt != 1760000000 {
		t.Errorf("createdAt = %d, want the second write", loaded.CreatedAt)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("%d files after two saves, want 1", len(entries))
	}
	// A save into a directory that does not exist creates it, owner-only,
	// because the keyfile has to be able to live under a fresh CCPASS_HOME.
	fresh := filepath.Join(t.TempDir(), "ccpass", "keyfile.json")
	if err := SaveKeyfile(fresh, kf); err != nil {
		t.Fatalf("a save into a fresh directory failed: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("the fresh keyfile is not there: %v", err)
	}
	// A path that cannot be a file at all fails rather than half-writing.
	if err := SaveKeyfile(string([]byte{0}), kf); err == nil {
		t.Error("a save to an unusable path succeeded")
	}
}

func TestLoadKeyfileRefusesGarbage(t *testing.T) {
	path := tempPath(t)
	for _, in := range []string{
		"",
		"{}",
		"not json",
		"{\"version\":1}",
		"[]",
	} {
		if err := os.WriteFile(path, []byte(in), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadKeyfile(path); err == nil {
			t.Errorf("LoadKeyfile accepted %q", in)
		}
	}
	if _, err := LoadKeyfile(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Error("a missing keyfile loaded")
	}
}

func TestKeyfileJSONFieldNames(t *testing.T) {
	// The field names are the format: a port that spells them differently
	// cannot read the file.
	path := tempPath(t)
	kf, _, err := NewAccount(testMnemonic(t), testPassphrase, time.Unix(1730000000, 0).UTC(), aReader())
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveKeyfile(path, kf); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{
		"\"version\"", "\"accountId\"", "\"vaultId\"", "\"argon2\"",
		"\"mKiB\"", "\"t\"", "\"p\"", "\"salt\"",
		"\"wrapNonce\"", "\"wrappedRoot\"", "\"createdAt\"",
	} {
		if !strings.Contains(string(raw), field) {
			t.Errorf("the keyfile has no %s field", field)
		}
	}
}
