package ccapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"ccpass/internal/identity"
	"ccpass/internal/store"
	"ccpass/internal/vault"
)

// appMnemonic is a real 24-word mnemonic, so these tests exercise the actual
// derivation rather than a stub.
const appMnemonic = "coral light army gather adapt blossom school alcohol coral light army gather adapt blossom school alcohol coral light army gather adapt blossom school awesome"

// otherAppMnemonic is a different account entirely.
const otherAppMnemonic = "couple maze era give basic obtain shadow change couple maze era give basic obtain shadow change couple maze era give basic obtain shadow course"

// fixedUnix is the time these tests run at, so the ids and the revs they
// produce are the same every run.
const fixedUnix int64 = 1730000000

func fixedNow() time.Time { return time.Unix(fixedUnix, 0).UTC() }

// appReader is 128 deterministic bytes: enough for a keyfile's salt and nonce,
// and the same every run.
func appReader() *strings.Reader {
	b := make([]byte, 128)
	for i := range b {
		b[i] = byte(i)
	}
	return strings.NewReader(string(b))
}

// testHome is a home directory holding a keyfile, and the passphrase that
// unlocks it.
func testHome(t *testing.T) (home, passphrase string) {
	t.Helper()
	home = t.TempDir()
	passphrase = "correct horse battery staple"
	kf, _, err := identity.NewAccount(appMnemonic, passphrase, fixedNow(), appReader())
	if err != nil {
		t.Fatal(err)
	}
	if err := identity.SaveKeyfile(filepath.Join(home, store.KeyfileName), kf); err != nil {
		t.Fatal(err)
	}
	return home, passphrase
}

// emptyHome is a home directory with nothing in it.
func emptyHome(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// newTester builds the app's view on a headless tester.
func newTester(t *testing.T, a *App) *ui.Tester {
	t.Helper()
	return ui.NewTester(a.View, 900, 600)
}

// field clicks a field's label, which focuses the control inside it, and types
// s into it.
func field(t *testing.T, tt *ui.Tester, label, s string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatalf("clicking the %s field: %v", label, err)
	}
	tt.Type(s)
}

func TestTheStartScreenAsksForThePassphraseWhenAKeyfileIsThere(t *testing.T) {
	home, _ := testHome(t)
	a, err := New(Options{Home: home})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Lock()
	tt := newTester(t, a)
	// A keyfile is there, so the screen is the everyday one: a passphrase and
	// nothing else. The mnemonic belongs to the recovery screen, which is a
	// separate place.
	if !tt.HasText("口令") {
		t.Errorf("the start screen does not ask for a passphrase: %v", tt.Texts())
	}
	if tt.HasText("助记词") {
		t.Error("a keyfile is there, so the start screen is not the recovery screen")
	}
	if a.Unlocked() {
		t.Error("the app started unlocked")
	}
}

func TestTheStartScreenOffersToCreateAnAccountWhenThereIsNoKeyfile(t *testing.T) {
	a, err := New(Options{Home: emptyHome(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Lock()
	tt := newTester(t, a)
	if !tt.HasText("新建") {
		t.Errorf("a fresh install does not offer a new account: %v", tt.Texts())
	}
	if tt.HasText("口令") {
		t.Errorf("a fresh install asks for a passphrase: %v", tt.Texts())
	}
}

func TestUnlockWithTheRightPassphraseShowsTheMainView(t *testing.T) {
	home, passphrase := testHome(t)
	a, err := New(Options{Home: home})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Lock()
	tt := newTester(t, a)
	field(t, tt, "口令", passphrase)
	if err := tt.Click("解锁"); err != nil {
		t.Fatal(err)
	}
	if !a.Unlocked() {
		t.Fatal("the right passphrase did not unlock")
	}
	// The main view, and not the unlock screen any more.
	tt.Frame()
	if !tt.HasText("锁定") {
		t.Errorf("the unlocked view has no lock button: %v", tt.Texts())
	}
	if tt.HasText("口令") {
		t.Errorf("the unlocked view still shows the passphrase field: %v", tt.Texts())
	}
}

func TestUnlockWithAWrongPassphraseSaysOneThing(t *testing.T) {
	// Every wrong passphrase is the same message. Nothing tells anyone
	// whether the words were wrong or the file was, so a guess at either is
	// no better off than before.
	for _, pw := range []string{
		"correct horse battery staplx",
		"correct horse battery stapl",
		"correct horse battery staple ",
	} {
		home, _ := testHome(t)
		a, err := New(Options{Home: home})
		if err != nil {
			t.Fatal(err)
		}
		tt := newTester(t, a)
		field(t, tt, "口令", pw)
		if err := tt.Click("解锁"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if !tt.HasText("口令不对，或密钥文件已损坏") {
			t.Fatalf("%q gave: %v", pw, tt.Texts())
		}
		if a.Unlocked() {
			t.Fatal("a wrong passphrase unlocked the app")
		}
		// And the field is empty again, so the wrong words are not left there
		// to be read.
		field(t, tt, "口令", "correct horse battery staple")
		if err := tt.Click("解锁"); err != nil {
			t.Fatal(err)
		}
		if !a.Unlocked() {
			t.Fatalf("after a wrong passphrase the app would not unlock: %v", tt.Texts())
		}
		a.Lock()
	}
}

func TestUnlockRefusesAShortPassphraseWithItsOwnMessage(t *testing.T) {
	home, _ := testHome(t)
	a, err := New(Options{Home: home})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Lock()
	tt := newTester(t, a)
	field(t, tt, "口令", "短")
	if err := tt.Click("解锁"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("口令太短，至少 8 个字符") {
		t.Errorf("a short passphrase gave: %v", tt.Texts())
	}
	if a.Unlocked() {
		t.Error("a short passphrase unlocked the app")
	}
}

func TestThePassphraseFieldIsMasked(t *testing.T) {
	home, passphrase := testHome(t)
	a, err := New(Options{Home: home})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Lock()
	tt := newTester(t, a)
	field(t, tt, "口令", passphrase)
	tt.Frame()
	// The passphrase itself is nowhere on screen: the field shows bullets,
	// and no text node holds what was typed.
	for _, s := range tt.Texts() {
		if strings.Contains(s, passphrase) {
			t.Errorf("the passphrase is on screen: %q", s)
		}
	}
}

func TestLockClearsTheSessionAndReturnsToTheStartScreen(t *testing.T) {
	home, passphrase := testHome(t)
	a, err := New(Options{Home: home})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Lock()
	tt := newTester(t, a)
	field(t, tt, "口令", passphrase)
	if err := tt.Click("解锁"); err != nil {
		t.Fatal(err)
	}
	if !a.Unlocked() {
		t.Fatal("unlock failed")
	}
	a.Lock()
	tt.Frame()
	if a.Unlocked() {
		t.Error("Lock left the app unlocked")
	}
	if !tt.HasText("口令") {
		t.Errorf("after a lock the start screen is not back: %v", tt.Texts())
	}
}

func TestRecoveryOfTheSameAccountUnlocksAndClearsTheFields(t *testing.T) {
	home := emptyHome(t)
	// A recovery has only the mnemonic, so it goes in one field.
	a, err := New(Options{Home: home, Rand: appReader()})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Lock()
	tt := newTester(t, a)
	if err := tt.Click("恢复"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("助记词") {
		t.Fatalf("no recovery screen: %v", tt.Texts())
	}
	// A pasted mnemonic arrives with the newlines a paste carries, and they
	// are folded to spaces: the field holds one line.
	field(t, tt, "助记词", strings.ReplaceAll(appMnemonic, " ", "\n"))
	tt.Frame()
	if !tt.HasText("24") {
		t.Errorf("the word count did not follow the paste: %v", tt.Texts())
	}
	field(t, tt, "新口令", "recovery passphrase here")
	field(t, tt, "再输一次", "recovery passphrase here")
	if err := tt.Click("开始恢复"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.Unlocked() {
		t.Fatalf("recovery did not unlock: %v", tt.Texts())
	}
	// The keyfile is written, and the mnemonic and passphrase fields are
	// empty: nothing secret is left on screen to be shoulder-surfed.
	if _, err := os.Stat(filepath.Join(home, store.KeyfileName)); err != nil {
		t.Errorf("recovery wrote no keyfile: %v", err)
	}
	for _, s := range tt.Texts() {
		if strings.Contains(s, appMnemonic) || strings.Contains(s, "recovery passphrase") {
			t.Errorf("a secret is still on screen: %q", s)
		}
	}
}

func TestRecoveryCountsTheWordsAsTheyAreTyped(t *testing.T) {
	a, err := New(Options{Home: emptyHome(t), Rand: appReader()})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Lock()
	tt := newTester(t, a)
	if err := tt.Click("恢复"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	field(t, tt, "助记词", "coral light army")
	tt.Frame()
	if !tt.HasText("3") {
		t.Errorf("three words gave: %v", tt.Texts())
	}
	// The count is of words, not of characters, and it rises as they come.
	// The field keeps the focus the click gave it, so this types into the
	// words that are already there rather than over them.
	tt.Type(" gather adapt")
	tt.Frame()
	if !tt.HasText("5") {
		t.Errorf("five words gave: %v", tt.Texts())
	}
}

func TestRecoveryRefusesTheWrongNumberOfWords(t *testing.T) {
	a, err := New(Options{Home: emptyHome(t), Rand: appReader()})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Lock()
	tt := newTester(t, a)
	if err := tt.Click("恢复"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	words := strings.Fields(appMnemonic)
	field(t, tt, "助记词", strings.Join(words[:23], " "))
	field(t, tt, "新口令", "recovery passphrase here")
	field(t, tt, "再输一次", "recovery passphrase here")
	if err := tt.Click("开始恢复"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	want := "助记词应为 24 个词，现在是 23 个"
	if !tt.HasText(want) {
		t.Errorf("23 words gave: %v", tt.Texts())
	}
	if a.Unlocked() {
		t.Error("23 words unlocked the app")
	}
}

func TestRecoveryRefusesAWordThatIsNotInTheWordlist(t *testing.T) {
	a, err := New(Options{Home: emptyHome(t), Rand: appReader()})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Lock()
	tt := newTester(t, a)
	if err := tt.Click("恢复"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	words := strings.Fields(appMnemonic)
	words[6] = "notaword"
	field(t, tt, "助记词", strings.Join(words, " "))
	field(t, tt, "新口令", "recovery passphrase here")
	field(t, tt, "再输一次", "recovery passphrase here")
	if err := tt.Click("开始恢复"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	want := "第 7 个词不在词表里"
	if !tt.HasText(want) {
		t.Errorf("a bad word gave: %v", tt.Texts())
	}
}

func TestRecoveryRefusesABadChecksum(t *testing.T) {
	a, err := New(Options{Home: emptyHome(t), Rand: appReader()})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Lock()
	tt := newTester(t, a)
	if err := tt.Click("恢复"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// Swap two real words: the right count, every word in the wordlist, and
	// a checksum that no longer matches.
	words := strings.Fields(appMnemonic)
	words[0], words[1] = words[1], words[0]
	field(t, tt, "助记词", strings.Join(words, " "))
	field(t, tt, "新口令", "recovery passphrase here")
	field(t, tt, "再输一次", "recovery passphrase here")
	if err := tt.Click("开始恢复"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	want := "助记词校验和不对，请检查是否抄错"
	if !tt.HasText(want) {
		t.Errorf("a bad checksum gave: %v", tt.Texts())
	}
}

func TestRecoveryRefusesTwoDifferentPassphrases(t *testing.T) {
	a, err := New(Options{Home: emptyHome(t), Rand: appReader()})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Lock()
	tt := newTester(t, a)
	if err := tt.Click("恢复"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	field(t, tt, "助记词", appMnemonic)
	field(t, tt, "新口令", "recovery passphrase here")
	field(t, tt, "再输一次", "recovery passphrase herE")
	if err := tt.Click("开始恢复"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	want := "两次输入的口令不一致"
	if !tt.HasText(want) {
		t.Errorf("a mismatch gave: %v", tt.Texts())
	}
	if a.Unlocked() {
		t.Error("a mismatched passphrase unlocked the app")
	}
}

func TestRecoveryRefusesAnotherAccountsMnemonicWhenThereIsLocalData(t *testing.T) {
	home, _ := testHome(t)
	a, err := New(Options{Home: home, Rand: appReader()})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Lock()
	// A keyfile is there, so the app opens it rather than making an account.
	// Recovering a different mnemonic into it would leave a database that
	// belongs to nobody, so it is refused, and the files are named.
	tt := newTester(t, a)
	if err := tt.Click("恢复"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	field(t, tt, "助记词", otherAppMnemonic)
	field(t, tt, "新口令", "recovery passphrase here")
	field(t, tt, "再输一次", "recovery passphrase here")
	if err := tt.Click("开始恢复"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.Unlocked() {
		t.Fatal("another account's mnemonic unlocked this one's data")
	}
	for _, s := range []string{store.KeyfileName, store.DBName} {
		if !tt.HasText(s) {
			t.Errorf("the refusal does not name %s: %v", s, tt.Texts())
		}
	}
	// And the keyfile that was there is untouched.
	if _, err := os.Stat(filepath.Join(home, store.KeyfileName)); err != nil {
		t.Errorf("the keyfile is gone: %v", err)
	}
}

func TestRecoveryOfAnExistingAccountKeepsItsVaults(t *testing.T) {
	home, passphrase := testHome(t)
	// Open it once, so the database and the default vault are there, then
	// recover the same mnemonic into it: the same account, the same vault.
	db, err := store.Open(filepath.Join(home, store.DBName))
	if err != nil {
		t.Fatal(err)
	}
	kf, err := identity.LoadKeyfile(filepath.Join(home, store.KeyfileName))
	if err != nil {
		t.Fatal(err)
	}
	session, err := identity.Unlock(kf, passphrase)
	if err != nil {
		t.Fatal(err)
	}
	v, err := vault.Open(db, session, fixedUnix)
	if err != nil {
		t.Fatal(err)
	}
	want := v.Vaults()[0].ID
	v.Close()
	db.Close()

	a, err := New(Options{Home: home, Rand: appReader()})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Lock()
	tt := newTester(t, a)
	if err := tt.Click("恢复"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	field(t, tt, "助记词", appMnemonic)
	field(t, tt, "新口令", passphrase)
	field(t, tt, "再输一次", passphrase)
	if err := tt.Click("开始恢复"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.Unlocked() {
		t.Fatalf("recovery failed: %v", tt.Texts())
	}
	got := a.Vaults()
	if len(got) != 1 || got[0] != want {
		t.Errorf("after a recovery the vaults are %v, want [%s]", got, want)
	}
}

func TestTheAsyncRunnerKeepsTheUIResponsive(t *testing.T) {
	// Unlocking runs Argon2id, which takes a second or so. The work goes
	// through the injected runner, and the interface is told when it is done.
	var work, done func()
	a, err := New(Options{
		Home: homeOf(t),
		Run:  func(w, d func()) { work, done = w, d },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Lock()
	tt := newTester(t, a)
	field(t, tt, "口令", "correct horse battery staple")
	if err := tt.Click("解锁"); err != nil {
		t.Fatal(err)
	}
	if work == nil {
		t.Fatal("the unlock work did not go through the runner")
	}
	// The app is not unlocked yet: the work has not come back.
	if a.Unlocked() {
		t.Error("the app unlocked before the work finished")
	}
	tt.Frame()
	if !tt.HasText("正在解锁") {
		t.Errorf("the app does not say it is unlocking: %v", tt.Texts())
	}
	work()
	done()
	tt.Frame()
	if !a.Unlocked() {
		t.Errorf("the app did not unlock when the work came back: %v", tt.Texts())
	}
}

func TestContentProtectionIsTurnedOnForTheMnemonicAndOffAfter(t *testing.T) {
	var protect []bool
	a, err := New(Options{
		Home:    emptyHome(t),
		Rand:    appReader(),
		Protect: func(on bool) { protect = append(protect, on) },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Lock()
	tt := newTester(t, a)
	if err := tt.Click("恢复"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(protect) == 0 || !protect[0] {
		t.Fatalf("the mnemonic field did not turn content protection on: %v", protect)
	}
	field(t, tt, "助记词", appMnemonic)
	field(t, tt, "新口令", "recovery passphrase here")
	field(t, tt, "再输一次", "recovery passphrase here")
	if err := tt.Click("开始恢复"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(protect) < 2 || protect[len(protect)-1] {
		t.Errorf("content protection was not turned back off: %v", protect)
	}
}

func TestNewRefusesAHomeItCannotUse(t *testing.T) {
	// A file where the home directory should be.
	f := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(Options{Home: f}); err == nil {
		t.Error("a file was accepted as the home directory")
	}
}

// homeOf is a home holding a keyfile, for the tests that do not care which
// passphrase opens it.
func homeOf(t *testing.T) string {
	t.Helper()
	home, _ := testHome(t)
	return home
}

func TestTheWorkOffTheMainThreadTouchesNoScreensState(t *testing.T) {
	// The unlock runs Argon2id and the vault's own decryption, so it goes off
	// the main thread. While it runs, a frame is still being built: the work
	// must hand its result back and let the main thread apply it, rather than
	// reaching into the screens it is drawing from under them.
	var work, done func()
	a, err := New(Options{
		Home: homeOf(t),
		Run:  func(w, d func()) { work, done = w, d },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Lock()
	tt := newTester(t, a)
	field(t, tt, "口令", "correct horse battery staple")
	if err := tt.Click("解锁"); err != nil {
		t.Fatal(err)
	}
	// Frames keep being built while the work runs, and none of them sees a
	// half-applied change: the app is still locked, and the screen still asks
	// for the passphrase rather than showing the vault.
	for i := 0; i < 3; i++ {
		tt.Frame()
		if a.Unlocked() {
			t.Fatalf("frame %d: the app unlocked before the work came back", i)
		}
		if !tt.HasText("口令") {
			t.Fatalf("frame %d: the start screen is gone: %v", i, tt.Texts())
		}
	}
	work()
	// The result is not applied by the work itself.
	tt.Frame()
	if a.Unlocked() {
		t.Error("the work applied its own result")
	}
	done()
	tt.Frame()
	if !a.Unlocked() {
		t.Errorf("the main thread did not take the result: %v", tt.Texts())
	}
	if !tt.HasText("锁定") {
		t.Errorf("the main view is not showing: %v", tt.Texts())
	}
}

func TestAFailureToRecoverKeepsTheMnemonicAndSaysOneThing(t *testing.T) {
	// A recovery that fails after the words were typed — the disk is full, the
	// file cannot be written — leaves the words where they are. They are the
	// account, and making someone type all 24 of them again because a write
	// failed is the kind of thing that loses people their vault.
	home := emptyHome(t)
	blocked := filepath.Join(home, store.KeyfileName)
	if err := os.Mkdir(blocked, 0o500); err != nil {
		t.Fatal(err)
	}
	a, err := New(Options{Home: home, Rand: appReader()})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Lock()
	tt := newTester(t, a)
	if err := tt.Click("恢复"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	field(t, tt, "助记词", appMnemonic)
	field(t, tt, "新口令", "recovery passphrase here")
	field(t, tt, "再输一次", "recovery passphrase here")
	if err := tt.Click("开始恢复"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.Unlocked() {
		t.Fatal("a recovery that could not write its keyfile unlocked the app")
	}
	// Still on the recovery screen, with the words still there, and one thing
	// said about what went wrong.
	if !tt.HasText("恢复账号") {
		t.Errorf("the recovery screen is gone: %v", tt.Texts())
	}
	if !tt.HasText("24") {
		t.Errorf("the mnemonic was cleared: %v", tt.Texts())
	}
	// "口令" alone would match the 新口令 label above it, so this asks for the
	// start screen's own 解锁.
	if tt.HasText("解锁") {
		t.Errorf("a failed recovery fell back to the start screen: %v", tt.Texts())
	}
	if len(tt.Texts()) == 0 {
		t.Error("nothing on screen")
	}
}
