package ccapp

import (
	"fmt"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
)

// settingsState is what the settings screen holds between frames. There is one
// setting in v1, so there is nothing else here: the screen is where it lives.
type settingsState struct {
	// open is whether the settings screen is showing.
	open bool
}

// settingsView is the settings: what the auto-lock is set to. It is a screen of
// its own over the main one, so nothing behind it takes a key while it is up.
func (a *App) settingsView(c *ui.Context) {
	ui.Column(c).Fill().Gap(12).Children(func() {
		ui.Row(c).Gap(8).Children(func() {
			if ui.Button(c, "返回").Clicked() {
				a.closeSettings()
			}
			ui.Text(c, "设置").FontSize(20).Bold().Grow(1)
		})
		ui.Text(c, "自动锁定").Bold()
		// The choices are named, and 永不 is not among them. A password manager
		// with no lock is a password manager whose key sits in memory for as
		// long as the machine is on, and somebody who wants it open for longer
		// is offered an hour.
		ui.Text(c, "一段时间没有操作就自动锁定，锁定后要用口令重新打开。").TextColor(c.Theme().TextMuted)
		ui.Column(c).Gap(8).Children(func() {
			for _, m := range idleChoices {
				b := ui.Button(c, idleLabel(m))
				if a.idleTimeout() == time.Duration(m)*time.Minute {
					b.Background(c.Theme().Surface)
				}
				if b.Clicked() {
					a.setIdle(c, m)
				}
			}
		})
		if a.err != "" {
			ui.Text(c, a.err).TextColor(c.Theme().TextMuted)
		}
	})
}

// idleLabel is what a timeout of m minutes is called on screen.
func idleLabel(m int) string {
	return fmt.Sprintf("%d 分钟", m)
}

// setIdle stores a chosen timeout. It is the store's own meta table, which
// holds no secret, so a setting that survives a restart costs nothing the plan
// protects.
func (a *App) setIdle(c *ui.Context, minutes int) {
	if a.db == nil {
		return
	}
	if err := a.db.SetMeta(idleKey, strconv.Itoa(minutes)); err != nil {
		a.err = "设置没存上，可能是磁盘满了。"
		return
	}
	a.err = ""
	// The idle timer is the view's own, and this is called from a frame that
	// has one: the next frame sees the new timeout and arms the timer again
	// from now. A wall-clock call here would move a deadline the frame clock
	// does not share.
	a.touch(c.Now())
}

// openSettings shows the settings screen.
func (a *App) openSettings() { a.settings.open = true; a.err = "" }

// closeSettings leaves it.
func (a *App) closeSettings() { a.settings = settingsState{} }
