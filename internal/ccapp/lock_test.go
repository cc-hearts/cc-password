package ccapp

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// lockMain opens the app on the main view, unlocked, the way the app is after
// a passphrase has been typed.
func lockMain(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	a, tt := newMain(t)
	return a, tt
}

// idleMinutes is the auto-lock timeout the tests set, in minutes. The real
// default is five; the tests set their own so the clock does not have to be
// advanced five minutes at a time.
const idleMinutes = 1

// setIdle writes the auto-lock timeout into the store, the way the settings
// screen does.
func setIdle(t *testing.T, a *App, minutes int) {
	t.Helper()
	if err := a.db.SetMeta(idleKey, itoa(minutes)); err != nil {
		t.Fatal(err)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestTheVaultLocksItselfAfterTheIdleTimeout(t *testing.T) {
	a, tt := lockMain(t)
	setIdle(t, a, idleMinutes)
	tt.Frame()
	if !a.Unlocked() {
		t.Fatal("the app started locked")
	}
	// One minute of nothing: the root key is gone, and the screen is the
	// unlock one. A vault left open on a desk nobody is sitting at is a vault
	// whose key is in memory for as long as the machine is on.
	tt.Advance(time.Duration(idleMinutes)*time.Minute + time.Second)
	tt.Frame()
	if a.Unlocked() {
		t.Error("the vault is still open after the idle timeout")
	}
	if !tt.HasText("口令") {
		// A first-run home is the other thing a lock can show, and the
		// tester's is one: the vault was opened straight into the main view,
		// so there is no keyfile to be asked for.
		if !tt.HasText("新建账号") {
			t.Errorf("the app did not go back to unlocking: %v", tt.Texts())
		}
	}
	// The data is gone with it: an app that kept the vault handle behind a
	// locked screen would be one a bug could still read from.
	if a.v != nil {
		t.Error("the vault handle is still there")
	}
	if a.main.vaultID != "" {
		t.Errorf("the main screen still holds its vault: %q", a.main.vaultID)
	}
}

func TestTheIdleTimeoutIsTheDefaultOfFiveMinutes(t *testing.T) {
	a, tt := lockMain(t)
	tt.Frame()
	// Nothing has been set, so the default applies: five minutes, not a minute
	// and not never.
	if got := a.idleTimeout(); got != 5*time.Minute {
		t.Errorf("the default idle timeout is %v", got)
	}
	// And it is not never, however long the clock runs.
	tt.Advance(10 * time.Minute)
	tt.Frame()
	if a.Unlocked() {
		t.Error("an unset timeout never locked")
	}
}

func TestActivityResetsTheIdleTimeout(t *testing.T) {
	a, tt := lockMain(t)
	setIdle(t, a, idleMinutes)
	tt.Frame()
	// Half a minute of nothing, then somebody types in the search field. The
	// clock starts again from there: a user reading a long note is a user who
	// is there, and a vault that locked while they read would be a vault that
	// made them unlock to press nothing.
	tt.Advance(30 * time.Second)
	tt.Frame()
	field(t, tt, "搜索", "招")
	tt.Frame()
	tt.Advance(40 * time.Second)
	tt.Frame()
	if !a.Unlocked() {
		t.Fatal("the vault locked while it was being used")
	}
	tt.Advance(30 * time.Second)
	tt.Frame()
	if a.Unlocked() {
		t.Error("the timeout did not restart from the activity")
	}
}

func TestTheIdleSettingIsStoredAndApplied(t *testing.T) {
	a, tt := lockMain(t)
	// Ten minutes is chosen, written and read back. It is stored in the store's
	// own meta table, which holds no secret, so a setting that survives a
	// restart costs nothing the plan protects.
	if err := tt.Click("设置"); err != nil {
		t.Fatalf("clicking 设置: %v", err)
	}
	tt.Frame()
	if !tt.HasText("自动锁定") {
		t.Fatalf("the settings screen did not open: %v", tt.Texts())
	}
	// The choices are a picker, and "永不" is not among them: a password
	// manager with no way to stop it locking is a password manager that locks
	// when somebody is not there.
	if tt.HasText("永不") {
		t.Error("the settings offer never locking")
	}
	for _, want := range []string{"1 分钟", "5 分钟", "30 分钟", "60 分钟"} {
		if !tt.HasText(want) {
			t.Errorf("the choices do not include %s: %v", want, tt.Texts())
		}
	}
	if err := tt.Click("30 分钟"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := a.idleTimeout(); got != 30*time.Minute {
		t.Errorf("the timeout is %v after choosing 30 分钟", got)
	}
	// And it is still there after the vault is closed and opened again: a
	// setting that only lasted as long as the session would be one everybody
	// set every morning.
	got, err := a.db.Meta(idleKey)
	if err != nil {
		t.Fatalf("the setting was not stored: %v", err)
	}
	if got != "30" {
		t.Errorf("the store holds %q", got)
	}
}

func TestAnIdleSettingOutsideTheChoicesIsIgnored(t *testing.T) {
	a, tt := lockMain(t)
	// A store somebody else wrote, or an older build's idea of a number. The
	// app falls back to the default rather than locking every second, and
	// rather than locking never.
	if err := a.db.SetMeta(idleKey, "0"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := a.idleTimeout(); got != 5*time.Minute {
		t.Errorf("a timeout of 0 gave %v", got)
	}
	if err := a.db.SetMeta(idleKey, "9999"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := a.idleTimeout(); got != 5*time.Minute {
		t.Errorf("a timeout of 9999 gave %v", got)
	}
	if err := a.db.SetMeta(idleKey, "not a number"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := a.idleTimeout(); got != 5*time.Minute {
		t.Errorf("a timeout that is not a number gave %v", got)
	}
}

func TestTheClipboardIsClearedAfterThirtySeconds(t *testing.T) {
	_, tt := lockMain(t)
	openItem(t, tt, "招商银行")
	if err := tt.Click("复制密码"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := tt.Clipboard(); got == "" {
		t.Fatal("the copy did not reach the clipboard")
	}
	// Twenty-nine seconds is not thirty: a password cleared a second early is
	// a password somebody pastes into a login form that has already lost it.
	tt.Advance(29 * time.Second)
	tt.Frame()
	if got := tt.Clipboard(); got == "" {
		t.Error("the clipboard was cleared early")
	}
	tt.Advance(2 * time.Second)
	tt.Frame()
	if got := tt.Clipboard(); got != "" {
		t.Errorf("the clipboard still holds %q", got)
	}
}

func TestTheClipboardIsKeptWhenSomethingElseWasCopied(t *testing.T) {
	a, tt := lockMain(t)
	openItem(t, tt, "招商银行")
	if err := tt.Click("复制密码"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	first := tt.Clipboard()
	// Somebody else puts something on the clipboard: another app, or this one
	// copying a username. Clearing it would destroy their copy, not the
	// password, and the plan's rule is to compare before clearing.
	tt.SetClipboard("somebody else's text")
	tt.Advance(31 * time.Second)
	tt.Frame()
	if got := tt.Clipboard(); got != "somebody else's text" {
		t.Errorf("the clipboard holds %q, want somebody else's text", got)
	}
	// And the password is gone from the app's own idea of what is there: the
	// next copy arms the timer again rather than leaving a stale one running.
	if a.lock.clipboard != "" {
		t.Errorf("the app still holds %q", a.lock.clipboard)
	}
	_ = first
}

func TestLockingClearsTheClipboard(t *testing.T) {
	a, tt := lockMain(t)
	openItem(t, tt, "招商银行")
	if err := tt.Click("复制密码"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := tt.Clipboard(); got == "" {
		t.Fatal("nothing was copied")
	}
	a.Lock()
	tt.Frame()
	// A locked vault with a password on the clipboard is a password the vault
	// was protecting sitting in plain text in the system's own hands.
	if got := tt.Clipboard(); got != "" {
		t.Errorf("the clipboard still holds %q after a lock", got)
	}
}

func TestEscClosesTheOpenItemBeforeItLocks(t *testing.T) {
	a, tt := lockMain(t)
	setIdle(t, a, idleMinutes)
	openItem(t, tt, "招商银行")
	tt.Frame()
	// The first Esc closes what is open, and only then — with nothing left to
	// close — does the next one lock. A single Esc that locked would be an Esc
	// that lost the item somebody was reading.
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if !a.Unlocked() {
		t.Fatal("the first Esc locked the vault")
	}
	if a.main.open != "" {
		t.Error("the first Esc did not close the item")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.Unlocked() {
		t.Error("the second Esc did not lock")
	}
}

func TestEscClearsTheSearchBeforeItLocks(t *testing.T) {
	a, tt := lockMain(t)
	setIdle(t, a, idleMinutes)
	field(t, tt, "搜索", "招")
	tt.Frame()
	if a.main.query == "" {
		t.Fatal("the search field was not filled in")
	}
	// Somebody's search is their own work: Esc takes it away rather than
	// locking the vault out from under a list they were reading.
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.main.query != "" {
		t.Errorf("the search was not cleared: %q", a.main.query)
	}
	if !a.Unlocked() {
		t.Error("the first Esc locked the vault")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.Unlocked() {
		t.Error("the second Esc did not lock")
	}
}

func TestEscIsIgnoredWhileAnItemIsBeingEdited(t *testing.T) {
	a, tt := lockMain(t)
	setIdle(t, a, idleMinutes)
	if err := tt.Click("新建条目"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	field(t, tt, "名称", "写了一半的条目")
	// Esc while a form is open would throw away what has been typed, and the
	// form has its own 取消 for that. The plan says editing does not respond
	// to Esc for exactly this reason.
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if !a.edit.open {
		t.Error("Esc closed the form")
	}
	if !a.Unlocked() {
		t.Error("Esc locked the vault while a form was open")
	}
	if a.edit.title != "写了一半的条目" {
		t.Errorf("the form lost what was typed: %q", a.edit.title)
	}
}

func TestLockingDropsTheRootKey(t *testing.T) {
	a, tt := lockMain(t)
	if a.session == nil {
		t.Fatal("there is no session to lock away")
	}
	a.Lock()
	tt.Frame()
	// The session is the root key's handle, and it is dropped: a locked vault
	// is one nothing in this process can read again until the passphrase is
	// typed once more.
	if a.session != nil {
		t.Error("the session is still there")
	}
	if a.Unlocked() {
		t.Error("the vault is still open")
	}
}

func TestTheGeneratorClosesWhenTheVaultLocks(t *testing.T) {
	a, tt := lockMain(t)
	if err := tt.Click("生成器"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.gen.password == "" {
		t.Fatal("the generator made no password")
	}
	// An idle timeout with the generator open is an idle timeout with a
	// password on screen. It goes the way the rest of the screen does.
	tt.Advance(10 * time.Minute)
	tt.Frame()
	if a.gen.open {
		t.Error("the generator is still open")
	}
	if a.gen.password != "" {
		t.Errorf("the generated password is still in memory: %q", a.gen.password)
	}
}

func TestTheSettingsScreenClosesOnEsc(t *testing.T) {
	a, tt := lockMain(t)
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.settings.open {
		t.Fatal("the settings screen did not open")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.settings.open {
		t.Error("Esc did not close the settings")
	}
	if !a.Unlocked() {
		t.Error("the first Esc locked the vault")
	}
}
