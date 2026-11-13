package ccapp

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// unlockApp is the app as it starts, on the unlock screen, with a keyfile to
// unlock.
func unlockApp(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	home, _ := testHome(t)
	a, err := New(Options{Home: home})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Lock)
	return a, newTester(t, a)
}

// createApp is a fresh install on the first step of the create flow.
func createApp(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	return newCreate(t)
}

// Work that takes a second — Argon2id, and the vault's own decryption — is the
// app standing still. The screen should say so the way a screen says it: a
// spinner that turns while it runs, and words for whoever cannot see it turn.
//
// A line of grey text is not that. It is a sentence that appears, does not
// move, and then leaves, and it is indistinguishable from every other grey
// sentence on the screen — including the errors.

// TestTheUnlockShowsItIsWorking: unlocking runs Argon2id on the words, which
// takes about a second on purpose. While it runs there is a spinner on screen
// and the words 正在解锁 read out, so somebody who cannot see the spinner knows
// the machine is not broken.
func TestTheUnlockShowsItIsWorking(t *testing.T) {
	a, tt := unlockApp(t)
	a.busy = "正在解锁"
	tt.Frame()
	if !tt.HasText("正在解锁") {
		t.Errorf("the unlock does not say it is working: %v", tt.Texts())
	}
	// Said out loud as well as drawn: a spinner is a picture, and a picture is
	// nothing at all to a screen reader.
	got := tt.Announcements()
	if len(got) == 0 {
		t.Errorf("the working was not read out: %v", got)
	} else if got[len(got)-1] != "正在解锁" {
		t.Errorf("read out %q, want 正在解锁", got[len(got)-1])
	}
}

// TestTheCreateShowsItIsWorking: making the vault is the same work, and the
// create screen says so the same way.
func TestTheCreateShowsItIsWorking(t *testing.T) {
	a, tt := createApp(t)
	a.busy = "正在创建"
	tt.Frame()
	if !tt.HasText("正在创建") {
		t.Errorf("the create does not say it is working: %v", tt.Texts())
	}
	got := tt.Announcements()
	if len(got) == 0 || got[len(got)-1] != "正在创建" {
		t.Errorf("read out %v, want 正在创建", got)
	}
}

// TestTheWorkingStopsBeingSaid: the saying is for while the work runs, and
// stops when it does. A screen that keeps announcing 正在解锁 after the vault
// is open is a screen that lies about what is happening.
func TestTheWorkingStopsBeingSaid(t *testing.T) {
	a, tt := unlockApp(t)
	a.busy = "正在解锁"
	tt.Frame()
	tt.Announcements()
	a.busy = ""
	tt.Frame()
	if tt.HasText("正在解锁") {
		t.Errorf("the unlock is still said to be working: %v", tt.Texts())
	}
	if got := tt.Announcements(); len(got) != 0 {
		t.Errorf("read out %v after the work was done", got)
	}
}
