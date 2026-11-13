package ccapp

import (
	"testing"
	"time"
)

// A copy is a small thing that happened, and the saying of it is a small thing
// too: it should be there when the eyes look down at it, and gone by the time
// the next thing is done. These three tests are that, for the three places a
// copy can come from — the detail, the generator, and the created item.

// TestACopyIsSaidAndThenGoesAway: a copy is told about, and the telling goes
// away on its own. A line of grey text that stays until the next thing is a
// line the user has to read over, and one that is still there next time
// looking like news.
func TestACopyIsSaidAndThenGoesAway(t *testing.T) {
	_, tt, _ := detailVault(t)
	if err := tt.Click("复制用户名"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("已复制") {
		t.Fatalf("a copy said nothing: %v", tt.Texts())
	}
	// The saying is a toast: it leaves without anybody clearing it.
	tt.Advance(5 * time.Second)
	tt.Frame()
	if tt.HasText("已复制") {
		t.Errorf("the copy is still being talked about: %v", tt.Texts())
	}
}

// TestTheGeneratorSaysItsCopyTheSameWay: the generator is a second screen with
// its own copy, and it says so the same way. Two screens that report the same
// thing in two different ways is a user wondering whether one of them is wrong.
func TestTheGeneratorSaysItsCopyTheSameWay(t *testing.T) {
	_, tt, _ := genMain(t)
	openGen(t, tt)
	if err := tt.Click("复制"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("已复制") {
		t.Fatalf("a copy said nothing: %v", tt.Texts())
	}
	tt.Advance(5 * time.Second)
	tt.Frame()
	if tt.HasText("已复制") {
		t.Errorf("the copy is still being talked about: %v", tt.Texts())
	}
}

// TestASaidCopyClearsItselfBeforeTheClipboardDoes: what was copied is on the
// system's clipboard, and the app clears it after thirty seconds. The saying
// should not outlive the thing it is about — a toast that says 已复制 while the
// clipboard has already been emptied is a claim about nothing.
func TestASaidCopyClearsItselfBeforeTheClipboardDoes(t *testing.T) {
	_, tt, _ := detailVault(t)
	if err := tt.Click("复制用户名"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := tt.Clipboard(); got == "" {
		t.Fatal("nothing was copied")
	}
	// Past the copy's own saying, and well inside the clipboard's thirty
	// seconds: the clipboard is still holding what it was given.
	tt.Advance(10 * time.Second)
	tt.Frame()
	if got := tt.Clipboard(); got == "" {
		t.Errorf("the clipboard was cleared while it was still meant to be holding the copy")
	}
	if tt.HasText("已复制") {
		t.Errorf("the copy is still being talked about: %v", tt.Texts())
	}
}
