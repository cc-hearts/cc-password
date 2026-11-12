package ccapp

import (
	"github.com/egoist/mygo/ui"

	"ccpass/internal/identity"
)

// startView is the everyday screen: a passphrase where there is a keyfile, and
// the way to a new account where there is not.
func (a *App) startView(c *ui.Context) {
	ui.Column(c).Gap(12).Children(func() {
		ui.Text(c, "ccpass").FontSize(20).Bold()
		if a.keyfile != nil {
			ui.Field(c, "口令", func() {
				ui.TextInput(c, &a.unlock.passphrase).Password().Fill()
			})
			if a.busy != "" {
				ui.Text(c, a.busy).TextColor(c.Theme().TextMuted)
			} else {
				ui.Text(c, a.err).TextColor(c.Theme().TextMuted)
			}
			ui.Row(c).Gap(8).Children(func() {
				if ui.PrimaryButton(c, "解锁").Disabled(a.busy != "").Clicked() {
					a.unlockApp()
				}
				ui.Button(c, "恢复").Disabled(a.busy != "").OnClick(func() {
					a.screen, a.err = screenRecover, ""
					a.protect(true)
				})
			})
			return
		}
		ui.Text(c, "把 24 个词收好，它们就是你的账号。").TextColor(c.Theme().TextMuted)
		ui.Row(c).Gap(8).Children(func() {
			if ui.PrimaryButton(c, "新建账号").Clicked() {
				a.startCreate()
			}
			ui.Button(c, "恢复").OnClick(func() {
				a.screen, a.err = screenRecover, ""
				a.protect(true)
			})
		})
	})
}

// unlock opens the keyfile with the passphrase that was typed.
func (a *App) unlockApp() {
	if a.busy != "" {
		return
	}
	if err := identity.CheckPassphrase(a.unlock.passphrase); err != nil {
		// A passphrase that is too short is its own message: it is not an
		// authentication failure, and telling the user how short it is helps
		// them type the one they meant.
		a.err = "口令太短，至少 8 个字符"
		return
	}
	// What the work needs is read here, on the main thread, before it starts:
	// the keyfile and the passphrase belong to the screen, and the work runs
	// with what it was handed rather than reaching into a screen being drawn.
	keyfile, passphrase, home := a.keyfile, a.unlock.passphrase, a.home
	a.busy, a.err = "正在解锁", ""
	a.run(func() {
		out := outcome{}
		out.session, out.err = identity.Unlock(keyfile, passphrase)
		if out.err == nil {
			out.db, out.v, out.err = openVault(home, out.session)
		}
		a.pending = out
	}, a.finish)
}
