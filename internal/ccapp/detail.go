package ccapp

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"bipPass/internal/vault"
)

// detailState is what the detail holds between frames: which of its secrets
// are showing. Every one of them is off between frames, so a secret that was
// revealed is put away again unless the button is being held down right now.
type detailState struct {
	// showing is the key of the one secret being revealed, empty when none is.
	// It is a key rather than a flag per secret so that its empty value is
	// "nothing is showing": a zero int would be the first field, and a detail
	// that opens with somebody's field already legible is a bug nobody would
	// see coming.
	showing string
	// copied is what the last copy said, so a user who cannot see the
	// clipboard knows the copy happened.
	copied string
}

// detailView is the one open item. It is a screen of its own over the list:
// the list is not built while it shows, so the keys go where the detail puts
// them and nothing behind it takes them.
func (a *App) detailView(c *ui.Context) {
	e, ok := a.v.Entry(a.main.open)
	if !ok {
		// Gone: purged, or the vault was closed under it. Back to the list,
		// which shows what there is now.
		a.main.open = ""
		return
	}
	ui.Column(c).Fill().Gap(12).Children(func() {
		ui.Row(c).Gap(8).Children(func() {
			if ui.Button(c, "返回").Clicked() {
				a.main.open = ""
				// The list was not on screen while the detail was, so it has
				// no focus now: it takes it again on the way back, which is
				// what keeps the arrows working without a click first.
				a.main.listed = false
			}
			ui.Text(c, title(e)).FontSize(20).Bold().Grow(1)
		})
		ui.Text(c, fmt.Sprintf("%s · %s", typeLabel(e.Item.DisplayType()), vaultName(a.v, e.VaultID))).TextColor(c.Theme().TextMuted)
		it := e.Item
		if it.ReadOnly() {
			// A payload from a build that does not exist yet. Its data is
			// shown in full — hiding the user's own data would be the loss —
			// and it is said to be read-only, because a build that does not
			// know the format must not rewrite it and lose what it cannot
			// read.
			ui.Text(c, "只读：这条来自更新版本的 bipPass，只能看，不能改。").TextColor(c.Theme().TextMuted)
		}
		if it.Username != "" {
			a.detailRow(c, "用户名", it.Username, false, "复制用户名")
		}
		if it.Password != "" {
			a.detailSecret(c, "密码", it.Password, keyPassword, "复制密码")
		}
		if it.OTP != nil {
			a.detailOTP(c, it.OTP)
		}
		for i, u := range it.URLs {
			a.detailURL(c, u, i)
		}
		if it.Notes != "" {
			a.detailRow(c, "备注", it.Notes, false, "")
		}
		for i, f := range it.Fields {
			if f.Secret {
				a.detailSecret(c, f.Label, f.Value, fmt.Sprintf("字段%d", i), copyLabel(f.Label))
				continue
			}
			// A field that is not secret is the user's own words, like the
			// username and the notes: shown, and copyable.
			a.detailRow(c, f.Label, f.Value, false, copyLabel(f.Label))
		}
		if len(it.History) > 0 {
			a.detailHistory(c, it.History)
		}
		// What the item itself can be done to comes after what it holds: the
		// buttons that change it are the least likely reason somebody opened
		// it, and 彻底删除 is nowhere near a row's own copy button.
		a.itemActions(c, e)
		if a.detail.copied != "" {
			ui.Text(c, a.detail.copied).TextColor(c.Theme().TextMuted)
		}
		if a.err != "" {
			ui.Text(c, a.err).TextColor(c.Theme().TextMuted)
		}
	})
}

// detailRow is one of an item's plain values: legible, and copyable when there
// is a copy button for it. The username and the notes are the user's own words
// and there is nothing to hide, so they are simply shown.
func (a *App) detailRow(c *ui.Context, label, value string, secret bool, copy string) {
	ui.Row(c).Fill().Gap(8).Children(func() {
		ui.Text(c, label).TextColor(c.Theme().TextMuted)
		shown := value
		if secret {
			shown = bullets(value)
		}
		ui.Text(c, shown).SingleLine().Grow(1)
		if copy != "" {
			a.copyButton(c, copy, value)
		}
	})
}

// keyPassword is the reveal key of the item's own password, which no field's
// index can collide with.
const keyPassword = "密码"

// detailSecret is one value that is not shown unless its reveal is held: the
// password, and any field the item marks secret.
//
// The reveal is a hold, not a toggle. A toggle leaves a secret on screen until
// something takes it off, and a screen nobody is looking at is exactly when
// that matters; a hold shows it for as long as the pointer is down and no
// longer.
func (a *App) detailSecret(c *ui.Context, label, value, key, copy string) {
	held := false
	ui.Row(c).Fill().Gap(8).Children(func() {
		ui.Text(c, label).TextColor(c.Theme().TextMuted)
		shown := bullets(value)
		if a.detail.showing == key {
			shown = value
		}
		ui.Text(c, shown).SingleLine().Grow(1)
		// The reveal names its row: a detail can hold a password, several
		// secret fields and an old password each behind a 显示, and a button
		// that says only 显示 tells neither a user nor a screen reader which
		// one it is.
		reveal := ui.Button(c, "显示"+label)
		if reveal.Pressed() {
			held = true
		}
		a.copyButton(c, copy, value)
	})
	// The pointer is read from the button while the row is built, so what it
	// learned is written back here: the row is legible for as long as the
	// button is held, and the frame after it is let go is masked again.
	if held {
		a.detail.showing = key
	} else if a.detail.showing == key {
		a.detail.showing = ""
	}
}

// detailOTP is the two-factor code: the RFC 6238 code for the current step,
// the seconds it has left, and a copy button for it.
//
// The clock is the view's own, which the window's frames drive: the code is
// asked for again every frame, so it turns over when the step does without
// anything counting seconds. A frame is asked for when the step ends, which is
// what keeps a window sitting idle on a detail correct.
func (a *App) detailOTP(c *ui.Context, o *vault.OTP) {
	now := c.Now()
	code, err := o.Code(now)
	if ok := err == nil; !ok {
		// A secret this build cannot make a code from. It says so rather than
		// showing a blank where a code should be.
		a.err = "这个两步验证密钥用不了"
		return
	}
	ui.Row(c).Fill().Gap(8).Children(func() {
		ui.Text(c, "验证码").TextColor(c.Theme().TextMuted)
		ui.Text(c, code).SingleLine().Grow(1)
		ui.Text(c, fmt.Sprintf("%d 秒", int(o.Remaining(now).Seconds()))).TextColor(c.Theme().TextMuted)
		a.copyButton(c, "复制验证码", code)
	})
	c.After(time.Until(now.Truncate(30 * time.Second).Add(30 * time.Second)))
}

// detailURL is one of an item's urls. A url is data, and data can carry
// anything: a javascript: or data: url is code a browser runs in this app's
// name, so it is shown as text and never handed to the browser. A host with
// no scheme is a host someone typed, and it gets https.
func (a *App) detailURL(c *ui.Context, u string, i int) {
	opened := openURL(u)
	ui.Row(c).Fill().Gap(8).Children(func() {
		ui.Text(c, "网址").TextColor(c.Theme().TextMuted)
		if opened == "" {
			ui.Text(c, u).SingleLine().Grow(1)
			ui.Text(c, "打不开").TextColor(c.Theme().TextMuted)
			return
		}
		ui.Link(c, u, opened).SingleLine().Grow(1)
	})
}

// detailHistory is the old passwords. They are masked like the current one is
// — an old password opens the same doors — and each can be copied, because
// the reason to look at one is usually to use it somewhere.
func (a *App) detailHistory(c *ui.Context, history []vault.PasswordChange) {
	ui.Column(c).Gap(4).Children(func() {
		ui.Text(c, "用过的密码").TextColor(c.Theme().TextMuted)
		for _, h := range history {
			// An old password is masked and never revealed. It opens the same
			// doors the current one does, and the reason to look at one is to
			// copy it, which the button beside it does.
			ui.Row(c).Fill().Gap(8).Children(func() {
				ui.Text(c, bullets(h.Password)).SingleLine().Grow(1)
				ui.Button(c, "复制旧密码").OnClick(func() {
					a.copy(c, h.Password)
				})
			})
		}
	})
}

// copyButton copies value and says so. It is a button rather than a bare
// action so the thing being copied is named: a row of three identical icons
// copies whichever one the pointer happens to land on.
func (a *App) copyButton(c *ui.Context, label, value string) {
	if ui.Button(c, label).Clicked() {
		a.copy(c, value)
	}
}

// copy puts text on the clipboard and says what it was. The clipboard is the
// system's, so what was copied is out of this app's hands the moment it is
// there: the thirty-second clear, and the comparison before it, are what the
// app does about that.
func (a *App) copy(c *ui.Context, value string) {
	a.copySecret(c, value)
	a.detail.copied = "已复制"
}

// copyLabel is the copy button of one of an item's own fields. A field with no
// name cannot have a button of its own, so it gets none.
func copyLabel(label string) string {
	if label == "" {
		return ""
	}
	return "复制" + label
}

// openURL is the url a click on an item's url opens, or "" when it must not be
// opened at all.
//
// Only http and https are opened. A javascript:, data: or file: url is code or
// a local file rather than a page, and a browser asked for one runs it with
// the user's own cookies and session in it — the one place where a vault's
// data can turn into an action taken in the user's name. Anything else is
// shown as text so the user can see what their own item holds.
func openURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	parsed, err := url.Parse(u)
	if err != nil {
		return ""
	}
	switch parsed.Scheme {
	case "http", "https":
		return parsed.String()
	case "":
		// A bare host, the way people type one into a note: url.Parse puts it
		// all in Path and leaves the Host empty, so it is parsed again with a
		// scheme in front of it. It is a page, so it is opened, over https:
		// there is no plain http site worth sending a login to.
		withScheme, err := url.Parse("https://" + u)
		if err != nil || withScheme.Host == "" {
			return ""
		}
		return withScheme.String()
	}
	return ""
}

// bullets is a masked secret: one bullet per character, so a password's length
// is still a hint to whoever typed it and nothing more.
func bullets(s string) string {
	return strings.Repeat("•", len([]rune(s)))
}

// title is what a row and a detail are headed with. An item with no title is
// headed with its type rather than with nothing at all.
func title(e vault.Entry) string {
	if e.Item.Title != "" {
		return e.Item.Title
	}
	return typeLabel(e.Item.DisplayType())
}

// vaultName is a vault's name, or its id for one this build did not open.
func vaultName(v *vault.Vault, id string) string {
	for _, vi := range v.Vaults() {
		if vi.ID == id {
			return vi.Name
		}
	}
	return id
}
