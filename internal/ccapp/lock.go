package ccapp

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
)

// idleKey is the store's meta key holding the auto-lock timeout in minutes. The
// meta table holds no secret — it is routing data the sync also sees — so a
// setting that survives a restart costs nothing the plan protects.
const idleKey = "idle_minutes"

// The auto-lock choices, in minutes. Five is the default, sixty is the most
// anybody is offered, and there is no way to turn the timeout off at all: a
// password manager with no lock is a password manager whose key sits in memory
// for as long as the machine is on.
const (
	defaultIdle = 5 * time.Minute
	minIdle     = 1
	maxIdle     = 60
)

// lockState is what the auto-lock and the clipboard hold between frames.
type lockState struct {
	// idleAt is when the vault locks itself if nothing happens first. It is
	// moved forward by every keypress, click and character typed — the user
	// being there is what keeps the vault open, and nothing else is.
	idleAt time.Time
	// idleFor is the timeout idleAt was set with, so a change to the stored
	// setting is noticed even when it was written behind the app's back. A
	// timer armed for five minutes and then told to be one has to be armed
	// again, or the vault stays open for five.
	idleFor time.Duration
	// armed is whether the idle timer is waiting for a frame. Asking for one
	// every frame would keep the window redrawing for nothing, so the frame is
	// asked for once, when the timer is set.
	armed bool
	// clipboard is what this app last put on the system clipboard, and when.
	// The system's clipboard is the system's: what was copied is out of this
	// app's hands the moment it is there, so the only thing that can be done
	// about it is to clear it again later — and only if it is still the app's
	// own copy rather than something the user put there since.
	clipboard   string
	clipboardAt time.Time
	// clipboardPending is set when the vault locked while a copy was still on
	// the clipboard, and cleared by the next frame that has a Context to clear
	// it with. Lock has no Context of its own — the screen lock and the suspend
	// call it from the platform's own thread — so the view does the clearing.
	clipboardPending bool
}

// idleTimeout is how long the vault stays open with nothing happening. It is
// the stored setting, and the default when there is none — or when the stored
// one is a number nobody chose, which is what an older build or another app
// writing the table would produce.
func (a *App) idleTimeout() time.Duration {
	if a.db == nil {
		return defaultIdle
	}
	v, err := a.db.Meta(idleKey)
	if err != nil {
		return defaultIdle
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < minIdle || n > maxIdle {
		return defaultIdle
	}
	return time.Duration(n) * time.Minute
}

// idleChoices are the timeouts the settings screen offers.
var idleChoices = []int{1, 5, 15, 30, 60}

// touch records that the user is there, and moves the idle timer forward from
// the frame's clock. It runs from the keys, the clicks and the typing the view
// sees, which is everything a user does — and a frame being drawn is not one of
// those things.
func (a *App) touch(now time.Time) {
	if a.v == nil {
		return
	}
	a.lock.idleFor = a.idleTimeout()
	a.lock.idleAt = now.Add(a.lock.idleFor)
	a.lock.armed = false
}

// idleLock is the auto-lock: it locks when the timer has run out, and asks for
// the frame that will do it.
//
// It is called from the view, which is the only place the clock and the screen
// are the same thread's. The timer is the view's own `After`, so a window that
// is closed asks for nothing and a tester's clock drives it.
func (a *App) idleLock(c *ui.Context) {
	if a.v == nil {
		return
	}
	// A deadline that was set with a different timeout is set again: the
	// setting can change under the app — another window, an older build's idea
	// of a number — and a timer armed for the old one would keep the vault open
	// for as long as that.
	if a.lock.idleAt.IsZero() || a.lock.idleFor != a.idleTimeout() {
		a.touch(c.Now())
	}
	if now := c.Now(); !now.Before(a.lock.idleAt) {
		a.Lock()
		return
	}
	if !a.lock.armed {
		// One frame at the deadline, and another if something happens before
		// it: a timer that asked for a frame every second would keep the
		// window redrawing for a countdown nobody is looking at.
		c.After(a.lock.idleAt.Sub(c.Now()))
		a.lock.armed = true
	}
}

// escape is what Esc does, from the top down: it closes what is open, and only
// when there is nothing left to close does it lock.
//
// One Esc that locked would be an Esc that lost the item somebody was reading,
// or the search they were narrowing a list with. So the first Esc takes one
// layer off and says nothing, and the second — with the screen back to the list
// — is the lock.
//
// While the form is open it does nothing at all: a half-typed item is work that
// has not been saved, and Esc in a form is the field's own anyway. The form has
// its own 取消, which is a choice rather than a keypress.
func (a *App) escape(c *ui.Context) {
	switch {
	case a.edit.open:
		// The form's fields take Esc themselves; a window Esc here would
		// discard what was typed. Leave it to 取消.
	case a.gen.open:
		a.closeGen()
	case a.settings.open:
		a.closeSettings()
	case a.detail.showing != "" || a.main.open != "":
		a.main.open = ""
		a.main.listed = false
	case a.main.query != "":
		a.main.query = ""
	default:
		a.Lock()
		return
	}
	a.touch(c.Now())
}

// copySecret puts text on the system clipboard and remembers what this app put
// there, so it can be cleared again half a minute later — and only then, and
// only if it is still this app's copy.
//
// The clipboard is the system's: a password on it is out of this app's hands,
// and the plan's answer is the thirty-second clear with the comparison first.
// Clearing without it would destroy whatever the user copied after, which is
// the one thing the clear is meant to leave alone.
func (a *App) copySecret(c *ui.Context, text string) {
	c.WriteClipboard(text)
	a.lock.clipboard, a.lock.clipboardAt = text, c.Now()
}

// clipboardClear is the clipboard's thirty seconds. It clears what this app
// copied if — and only if — the clipboard still holds exactly that.
func (a *App) clipboardClear(c *ui.Context) {
	if a.lock.clipboardPending {
		// The vault locked with a copy still out there. It is cleared whatever
		// is on the clipboard now, because what is there is what this app put
		// there: the lock happened before anything else could have been copied.
		a.lock.clipboardPending = false
		c.WriteClipboard("")
		return
	}
	if a.lock.clipboard == "" {
		return
	}
	if c.ReadClipboard() != a.lock.clipboard {
		// Something else is there now: the user copied something after, or
		// another app did. It is not this app's to clear.
		a.lock.clipboard = ""
		return
	}
	if now := c.Now(); now.Before(a.lock.clipboardAt.Add(30 * time.Second)) {
		c.After(a.lock.clipboardAt.Add(30 * time.Second).Sub(now))
		return
	}
	c.WriteClipboard("")
	a.lock.clipboard = ""
}
