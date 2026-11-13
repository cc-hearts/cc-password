package ccapp

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// detailDelete is a detail on screen with the item it shows, opened from the
// vault the main screen holds. It is the shape the delete tests need: a live
// item that can be deleted and then looked for again.
func detailDelete(t *testing.T) (*App, *ui.Tester, string) {
	t.Helper()
	a, tt, e := detailVault(t)
	return a, tt, e.ID
}

// TestDeleteAsksBeforeItGoes: a delete is one click away from losing
// something. The trash is the way back, but the trash is also one click from
// a purge — so the first click asks what it is about to do, in the words of
// the thing it is about to do it to.
func TestDeleteAsksBeforeItGoes(t *testing.T) {
	a, tt, id := detailDelete(t)
	if err := tt.Click("删除"); err != nil {
		t.Fatalf("clicking 删除: %v", err)
	}
	tt.Frame()
	if !tt.HasText("删除这条？") {
		t.Errorf("a delete did not ask first: %v", tt.Texts())
	}
	if !tt.HasText("回收站") {
		t.Errorf("the question does not say where the item goes: %v", tt.Texts())
	}
	// Nothing has happened yet: the item is still live, still where it was.
	if e, ok := a.v.Entry(id); !ok || e.Deleted {
		t.Errorf("the item moved before the question was answered: %+v", e)
	}
}

// TestADeleteCanBeDeclined: an answer that is only yes is not a question. The
// item stays where it was, live, with everything it holds.
func TestADeleteCanBeDeclined(t *testing.T) {
	a, tt, id := detailDelete(t)
	if err := tt.Click("删除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("取消"); err != nil {
		t.Fatalf("clicking 取消: %v", err)
	}
	tt.Frame()
	e, ok := a.v.Entry(id)
	if !ok {
		t.Fatal("a declined delete lost the item altogether")
	}
	if e.Deleted {
		t.Errorf("a declined delete still deleted it: %+v", e)
	}
	if e.Item.Password == "" {
		t.Errorf("a declined delete cost the item its payload: %+v", e)
	}
	// And the detail is still open on it: the question was answered, and
	// nothing else moved.
	if a.main.open != id {
		t.Errorf("the detail closed on a declined delete: %q", a.main.open)
	}
}

// TestADeleteIsSaidOutLoud: a delete moves the item out of the list, which is
// a thing the user did and should be told happened. The saying is a toast —
// the library's own — so it goes away on its own and a screen reader reads it
// out, rather than a line of grey text that stays until the next thing.
func TestADeleteIsSaidOutLoud(t *testing.T) {
	_, tt, _ := detailDelete(t)
	if err := tt.Click("删除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("确认删除"); err != nil {
		t.Fatalf("clicking 确认删除: %v", err)
	}
	tt.Frame()
	if !tt.HasText("已删") {
		t.Errorf("a delete was not said out loud: %v", tt.Texts())
	}
}
