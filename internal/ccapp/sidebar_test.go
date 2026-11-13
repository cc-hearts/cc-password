package ccapp

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The vaults are a place of their own down the side of the window. It is a
// sidebar, and the library's own: a keyboard is how a lot of people move
// through a list like this, and the arrows, Home, End and typing the first
// letters of a name are what a sidebar does about that. A hand-rolled column of
// rows answers the pointer and nothing else.

// sidebarVaults is the main screen with three vaults in it: the one the app
// starts with, plus two more with names that can be told apart.
func sidebarVaults(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	a, tt, _ := editMain(t)
	for _, name := range []string{"bank", "work"} {
		if err := tt.Click("新建保险库"); err != nil {
			t.Fatalf("clicking 新建保险库: %v", err)
		}
		tt.Frame()
		field(t, tt, "名称", name)
		if err := tt.Click("创建"); err != nil {
			t.Fatalf("clicking 创建: %v", err)
		}
		tt.Frame()
	}
	// Back to the first vault, so there is a choice to be moved off.
	a.main.vaultID = a.v.Vaults()[0].ID
	a.main.selected, a.main.open = 0, ""
	tt.Frame()
	return a, tt
}

// TestTheSidebarTakesTheArrowKeys: with the sidebar's focus, Down moves to the
// next vault and Up back. Somebody who does not use a pointer at all has to be
// able to get to another vault, and a row that only answers the mouse leaves
// them on the one they started on.
func TestTheSidebarTakesTheArrowKeys(t *testing.T) {
	a, tt := sidebarVaults(t)
	vaults := a.v.Vaults()
	first := a.main.vaultID
	// Clicking an item chooses it and gives the sidebar the focus, which is
	// where the keys go from then on.
	if err := tt.Click(vaults[0].Name); err != nil {
		t.Fatalf("clicking the first vault: %v", err)
	}
	tt.Frame()
	if !tt.Focused(vaults[0].Name) {
		t.Fatalf("the sidebar did not take the focus: %v", tt.Texts())
	}
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if a.main.vaultID == first {
		t.Errorf("Down did not move off the first vault: %q", a.main.vaultID)
	}
	second := a.main.vaultID
	tt.Key(0, ui.KeyUp)
	tt.Frame()
	if a.main.vaultID != first {
		t.Errorf("Up went to %q, want back at %q", a.main.vaultID, first)
	}
	if a.main.vaultID == second {
		t.Errorf("Up did not move: %q", a.main.vaultID)
	}
}

// TestTheSidebarTakesHomeAndEnd: the ends of a long list of vaults are where
// the first and the last one are, and Home and End are how a keyboard gets
// there without walking the whole way.
func TestTheSidebarTakesHomeAndEnd(t *testing.T) {
	a, tt := sidebarVaults(t)
	vaults := a.v.Vaults()
	if err := tt.Click(vaults[0].Name); err != nil {
		t.Fatalf("clicking the first vault: %v", err)
	}
	tt.Frame()
	tt.Key(0, ui.KeyEnd)
	tt.Frame()
	if want := vaults[len(vaults)-1].ID; a.main.vaultID != want {
		t.Errorf("End chose %q, want the last vault %q", a.main.vaultID, want)
	}
	tt.Key(0, ui.KeyHome)
	tt.Frame()
	if want := vaults[0].ID; a.main.vaultID != want {
		t.Errorf("Home chose %q, want the first vault %q", a.main.vaultID, want)
	}
}

// TestTheSidebarTakesTyping: typing the first letters of a name is the fastest
// way to a vault in a long list, and it is what a sidebar does. Three vaults
// whose names start differently are enough to tell it works.
func TestTheSidebarTakesTyping(t *testing.T) {
	a, tt := sidebarVaults(t)
	vaults := a.v.Vaults()
	if err := tt.Click(vaults[0].Name); err != nil {
		t.Fatalf("clicking the first vault: %v", err)
	}
	tt.Frame()
	// The letters of the last vault's name, one keypress at a time: the sidebar
	// is on the first vault, and typing the name of another one goes to it.
	last := vaults[len(vaults)-1]
	// One keypress is enough: the letters are what the sidebar matches, and the
	// first letter of a name nobody else starts with is the whole name to it.
	tt.TypeKey(0, ui.KeyW, "w")
	tt.Frame()
	if a.main.vaultID != last.ID {
		t.Errorf("typing w chose %q, want %q (%q)", a.main.vaultID, last.ID, last.Name)
	}
}
