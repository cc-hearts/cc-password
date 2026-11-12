package ccapp

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"bipPass/internal/vault"
	"bipPass/internal/vaultcrypto"
)

// detailVault opens the app on the detail of one login item: a password, a
// two-factor secret, a url, notes, a secret field and an old password, so the
// detail has every kind of thing on it. The tester's clock is pinned, so the
// two-factor code is a fixed number rather than whatever minute the test runs
// in.
func detailVault(t *testing.T) (*App, *ui.Tester, vault.Entry) {
	t.Helper()
	a, tt := newMain(t)
	personal := a.main.vaultID
	// A real Base32 secret, so the code is the RFC 6238 value rather than a
	// stub's: JBSWY3DPEHPK3PXP is the RFC 4226 test key.
	it := vault.Item{
		Type:     vault.TypeLogin,
		Title:    "招商银行",
		Username: "carl",
		Password: "s3cret-passphrase",
		OTP:      &vault.OTP{Secret: "JBSWY3DPEHPK3PXP", Digits: 6, Period: 30, Algorithm: "SHA1"},
		URLs:     []string{"https://www.cmbchina.com/"},
		Notes:    "工资卡",
		Fields: []vault.Field{
			{Label: "取款密码", Value: "112233", Secret: true},
			{Label: "开户行", Value: "深圳分行"},
		},
		History: []vault.PasswordChange{{Password: "an-old-password", ChangedAt: fixedUnix - 86400}},
	}
	e, err := a.v.Create(personal, it, fixedUnix)
	if err != nil {
		t.Fatal(err)
	}
	a.main.open = e.ID
	tt.Frame()
	return a, tt, e
}

// wantCode is the RFC 6238 code for the test secret at time t, computed by the
// crypto package itself rather than written down: a test that hardcodes the
// digits proves nothing about which clock was used.
func wantCode(t *testing.T, at time.Time) string {
	t.Helper()
	o := vaultcrypto.OTP{Secret: "JBSWY3DPEHPK3PXP", Digits: 6, Period: 30, Algorithm: "SHA1"}
	code, err := o.Code(at)
	if err != nil {
		t.Fatal(err)
	}
	return code
}

func TestTheDetailHidesThePasswordUntilTheRevealIsHeld(t *testing.T) {
	_, tt, e := detailVault(t)
	// The password is on the detail, and it is not legible: the row shows a
	// run of bullets rather than the secret, and nothing else on screen holds
	// it either.
	for _, s := range tt.Texts() {
		if strings.Contains(s, e.Item.Password) {
			t.Errorf("the password is on screen: %q", s)
		}
	}
	if !tt.HasText("••") {
		t.Errorf("the password row shows no bullets: %v", tt.Texts())
	}
	// Holding the reveal button shows it, and letting go puts it away again.
	x, y := center(t, tt, "显示密码")
	tt.Press(x, y)
	tt.Frame()
	if !tt.HasText(e.Item.Password) {
		t.Errorf("holding 显示密码 did not reveal the password: %v", tt.Texts())
	}
	tt.Release(x, y)
	tt.Frame()
	for _, s := range tt.Texts() {
		if strings.Contains(s, e.Item.Password) {
			t.Errorf("the password is on screen after letting go: %q", s)
		}
	}
}

func TestTheDetailCopiesThePasswordToTheClipboard(t *testing.T) {
	_, tt, e := detailVault(t)
	if err := tt.Click("复制密码"); err != nil {
		t.Fatalf("clicking 复制密码: %v", err)
	}
	tt.Frame()
	if got := tt.Clipboard(); got != e.Item.Password {
		t.Errorf("the clipboard holds %q, want the password", got)
	}
	// And the secret never appeared on screen on its way there.
	for _, s := range tt.Texts() {
		if strings.Contains(s, e.Item.Password) {
			t.Errorf("copying showed the password: %q", s)
		}
	}
}

func TestTheDetailCopiesTheUsernameAndTheOTPCode(t *testing.T) {
	_, tt, e := detailVault(t)
	if err := tt.Click("复制用户名"); err != nil {
		t.Fatalf("clicking 复制用户名: %v", err)
	}
	if got := tt.Clipboard(); got != e.Item.Username {
		t.Errorf("the clipboard holds %q, want the username", got)
	}
	if err := tt.Click("复制验证码"); err != nil {
		t.Fatalf("clicking 复制验证码: %v", err)
	}
	want := wantCode(t, tt.Now())
	if got := tt.Clipboard(); got != want {
		t.Errorf("the clipboard holds %q, want the code %q", got, want)
	}
}

func TestTheDetailShowsTheOTPCodeWithTheSecondsLeft(t *testing.T) {
	_, tt, _ := detailVault(t)
	// The tester's clock stands still, so the code is the RFC value at this
	// moment and stays it until the clock is moved.
	code := wantCode(t, tt.Now())
	if !tt.HasText(code) {
		t.Fatalf("the code is not on screen: %v", tt.Texts())
	}
	// The seconds left are said in words, so a user can see whether the code
	// they are about to type still has time on it.
	if !tt.HasText("秒") {
		t.Errorf("the code's remaining time is not said: %v", tt.Texts())
	}
}

func TestTheDetailRefreshesTheOTPCodeWhenTheClockPassesTheStep(t *testing.T) {
	_, tt, _ := detailVault(t)
	// A period of 30 seconds, so the code turns over half a minute later: the
	// view asks for the code again on every frame rather than counting down
	// itself, and a frame is asked for when the step ends.
	before := wantCode(t, tt.Now())
	tt.Advance(31 * time.Second)
	tt.Frame()
	after := wantCode(t, tt.Now())
	if before == after {
		t.Fatalf("the code did not change: %q", before)
	}
	if !tt.HasText(after) {
		t.Errorf("the new code %q is not on screen: %v", after, tt.Texts())
	}
	// The clock the view used is the tester's, not the machine's: a frame
	// built a second later still shows the same code.
	tt.Frame()
	if !tt.HasText(after) {
		t.Errorf("the code moved on without the clock: %v", tt.Texts())
	}
}

func TestTheDetailMasksASecretFieldUntilItsRevealIsHeld(t *testing.T) {
	_, tt, e := detailVault(t)
	// A field marked secret is masked like the password is, and the plain one
	// beside it is legible.
	if !tt.HasText("深圳分行") {
		t.Errorf("the plain field is not shown: %v", tt.Texts())
	}
	for _, s := range tt.Texts() {
		if strings.Contains(s, "112233") {
			t.Errorf("a secret field is on screen: %q", s)
		}
	}
	// Its own reveal, held: the label of the field is what the hold acts on.
	x, y := center(t, tt, "显示取款密码")
	tt.Press(x, y)
	tt.Frame()
	if !tt.HasText(e.Item.Fields[0].Value) {
		t.Errorf("holding 显示 did not reveal the secret field: %v", tt.Texts())
	}
}

func TestTheDetailMasksThePasswordHistoryAndCopiesIt(t *testing.T) {
	_, tt, e := detailVault(t)
	old := e.Item.History[0].Password
	for _, s := range tt.Texts() {
		if strings.Contains(s, old) {
			t.Errorf("an old password is on screen: %q", s)
		}
	}
	if err := tt.Click("复制旧密码"); err != nil {
		t.Fatalf("clicking 复制旧密码: %v", err)
	}
	if got := tt.Clipboard(); got != old {
		t.Errorf("the clipboard holds %q, want the old password", got)
	}
}

func TestTheDetailOpensAURLWhenItIsClicked(t *testing.T) {
	_, tt, e := detailVault(t)
	u := e.Item.URLs[0]
	if err := tt.Click(u); err != nil {
		t.Fatalf("clicking %s: %v", u, err)
	}
	tt.Frame()
	opened := tt.OpenedURLs()
	if len(opened) != 1 || opened[0] != u {
		t.Errorf("opened %v, want [%s]", opened, u)
	}
}

func TestTheDetailGivesABareHostHTTPS(t *testing.T) {
	a, tt := newMain(t)
	e, err := a.v.Create(a.main.vaultID, vault.Item{
		Type:  vault.TypeLogin,
		Title: "裸域名",
		URLs:  []string{"www.cmbchina.com"},
	}, fixedUnix)
	if err != nil {
		t.Fatal(err)
	}
	a.main.open = e.ID
	tt.Frame()
	if err := tt.Click("www.cmbchina.com"); err != nil {
		t.Fatalf("clicking the bare host: %v", err)
	}
	tt.Frame()
	opened := tt.OpenedURLs()
	if len(opened) != 1 || opened[0] != "https://www.cmbchina.com" {
		t.Errorf("opened %v, want [https://www.cmbchina.com]", opened)
	}
}

func TestTheDetailDoesNotOpenADangerousURL(t *testing.T) {
	a, tt := newMain(t)
	// A url is data, and data can carry anything. javascript: and data: are
	// opened by a browser as code rather than as a page, so the detail shows
	// them as text and refuses to hand them to the browser.
	for _, bad := range []string{
		"javascript:alert(1)",
		"data:text/html,<script>alert(1)</script>",
		"file:///etc/passwd",
	} {
		_, err := a.v.Create(a.main.vaultID, vault.Item{
			Type:  vault.TypeLogin,
			Title: "危险链接 " + bad,
			URLs:  []string{bad},
		}, fixedUnix)
		if err != nil {
			t.Fatal(err)
		}
	}
	tt.Frame()
	// Whatever is on screen, the browser was never asked for any of them.
	tt.Click("javascript:alert(1)")
	tt.Click("data:text/html,<script>alert(1)</script>")
	tt.Click("file:///etc/passwd")
	tt.Frame()
	if len(tt.OpenedURLs()) != 0 {
		t.Errorf("opened %v, want none", tt.OpenedURLs())
	}
}

func TestTheDetailShowsANewerSchemaReadOnly(t *testing.T) {
	a, tt := newMain(t)
	// An item written by a later build has a schema this one does not know.
	// It is shown, in full, and said to be read-only: the data is the user's
	// and hiding it would be a loss, but a build that does not know the
	// format must not rewrite it.
	e, err := a.v.Create(a.main.vaultID, vault.Item{
		Type:     vault.TypeLogin,
		Title:    "来自未来的条目",
		Username: "carl",
		Schema:   vault.SchemaVersion + 1,
	}, fixedUnix)
	if err != nil {
		t.Fatal(err)
	}
	a.main.open = e.ID
	tt.Frame()
	if !tt.HasText("来自未来的条目") {
		t.Errorf("the item is not shown: %v", tt.Texts())
	}
	if !tt.HasText("只读") {
		t.Errorf("a newer schema is not said to be read-only: %v", tt.Texts())
	}
}

func TestTheDetailSaysNothingWhenTheItemIsGone(t *testing.T) {
	a, tt, e := detailVault(t)
	// The item was purged elsewhere, or the vault went. The detail closes
	// rather than showing a half of one.
	if err := a.v.Purge(e.ID, fixedUnix); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("复制密码") {
		t.Errorf("the detail is still showing a purged item: %v", tt.Texts())
	}
}

// center is the middle of the element showing or labeled s, which a press and
// a release act on. A hold is a press, a frame, and a release: the reveal
// buttons show their secret only while the pointer is down.
func center(tb testing.TB, tt *ui.Tester, s string) (x, y float32) {
	tb.Helper()
	r, ok := tt.Find(s)
	if !ok {
		tb.Fatalf("ui: no element shows %q", s)
	}
	return r.X + r.W/2, r.Y + r.H/2
}
