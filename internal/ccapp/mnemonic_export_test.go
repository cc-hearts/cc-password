package ccapp

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// exportApp is a create flow with an export that records what it was handed.
// The hook is the app's own seam: the real one asks the machine where to put
// the file, and a test cannot ask a window anything.
func exportApp(t *testing.T) (*App, *ui.Tester, *[]string, *error) {
	t.Helper()
	var got []string
	var err error
	a, e := New(Options{
		Home:   emptyHome(t),
		Rand:   createReader(),
		Export: func(text string) error { got = append(got, text); return err },
	})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(a.Lock)
	tt := newTester(t, a)
	if err := tt.Click("新建账号"); err != nil {
		t.Fatalf("clicking 新建账号: %v", err)
	}
	tt.Frame()
	return a, tt, &got, &err
}

// askFirst asks the export to write the mnemonic, and answers the question it
// asks with yes. The words stay on screen through both frames, which is what
// makes this a question rather than a step the words disappear behind.
func askFirst(t *testing.T, tt *ui.Tester, yes string) {
	t.Helper()
	if err := tt.Click("存成文件"); err != nil {
		t.Fatalf("clicking 存成文件: %v", err)
	}
	tt.Frame()
	if err := tt.Click(yes); err != nil {
		t.Fatalf("clicking %s: %v", yes, err)
	}
	tt.Frame()
	// One more frame: the answer is drawn in the frame after the one that
	// received it, and a screen read in between shows the question and nothing
	// the question produced.
	tt.Frame()
}

// TestTheWordsStepOffersBothWaysOut: the words are the account, so the screen
// that shows them offers more than a pen. A file and the clipboard are both
// offered, and neither is the only way forward.
func TestTheWordsStepOffersBothWaysOut(t *testing.T) {
	_, tt := newCreate(t)
	for _, want := range []string{"存成文件", "复制"} {
		if !tt.HasText(want) {
			t.Errorf("the words step does not offer %s: %v", want, tt.Texts())
		}
	}
	// The copy-down is still the way forward: an export is a way to keep the
	// words, not a way past the check that they were read.
	if !tt.HasText("我已抄好") {
		t.Errorf("the way forward is gone: %v", tt.Texts())
	}
}

// TestTheExportAsksFirst: a file leaves this app's hands, and the machine it
// lands on backs it up. The words are asked about before anything is written.
func TestTheExportAsksFirst(t *testing.T) {
	_, tt, got, _ := exportApp(t)
	if err := tt.Click("存成文件"); err != nil {
		t.Fatalf("clicking 存成文件: %v", err)
	}
	tt.Frame()
	if !tt.HasText("存到文件里") {
		t.Errorf("the export did not ask first: %v", tt.Texts())
	}
	if len(*got) != 0 {
		t.Errorf("the file was written before the question was answered: %q", *got)
	}
}

// TestTheExportWritesTheWordsItWasShown: what lands in the file is the
// mnemonic on screen, in order, with nothing else in it.
func TestTheExportWritesTheWordsItWasShown(t *testing.T) {
	_, tt, got, _ := exportApp(t)
	words := shownWords(t, tt)
	askFirst(t, tt, "确定保存")
	if len(*got) != 1 {
		t.Fatalf("the export ran %d times, want 1", len(*got))
	}
	if want := strings.Join(words, " "); (*got)[0] != want {
		t.Errorf("the file holds %q, and the words are %q", (*got)[0], want)
	}
	// The question is answered and gone, and the words are still the words.
	if tt.HasText("确定保存") {
		t.Errorf("the question is still up: %v", tt.Texts())
	}
}

// TestTheExportCanBeDeclined: a question that can only be answered one way is
// not a question.
func TestTheExportCanBeDeclined(t *testing.T) {
	_, tt, got, _ := exportApp(t)
	if err := tt.Click("存成文件"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("取消"); err != nil {
		t.Fatalf("clicking 取消: %v", err)
	}
	tt.Frame()
	if len(*got) != 0 {
		t.Errorf("a declined export wrote a file: %q", *got)
	}
	if !tt.HasText("我已抄好") {
		t.Errorf("the words step is not back: %v", tt.Texts())
	}
}

// TestAFailedExportSaysSo: a disk that is full, or a path that cannot be
// written, is a file nobody has. The screen says so rather than leaving
// somebody who pressed the button believing they have a copy.
func TestAFailedExportSaysSo(t *testing.T) {
	_, tt, _, failure := exportApp(t)
	*failure = errors.New("disk is full")
	askFirst(t, tt, "确定保存")
	if !tt.HasText("没存上") {
		t.Errorf("a failed export gave: %v", tt.Texts())
	}
	// And the words are still there to be written down by hand: a failed
	// export is not a reason to start the flow over.
	if !tt.HasText("我已抄好") {
		t.Errorf("the words step is gone: %v", tt.Texts())
	}
}

// TestTheWordsCanBeCopied: the clipboard is the other way out, and it is the
// system's the moment the words are on it. What this app does about that is
// the same thirty seconds every other copy gets.
func TestTheWordsCanBeCopied(t *testing.T) {
	_, tt := newCreate(t)
	words := shownWords(t, tt)
	if err := tt.Click("复制"); err != nil {
		t.Fatalf("clicking 复制: %v", err)
	}
	tt.Frame()
	if want := strings.Join(words, " "); tt.Clipboard() != want {
		t.Errorf("the clipboard holds %q, and the words are %q", tt.Clipboard(), want)
	}
}

// TestACopiedMnemonicIsCleared: the words on a clipboard are out of this app's
// hands, so they do not stay there. Half a minute and they are gone, which is
// the plan's answer to the clipboard being the system's.
func TestACopiedMnemonicIsCleared(t *testing.T) {
	_, tt := newCreate(t)
	if err := tt.Click("复制"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Advance(31 * time.Second)
	tt.Frame()
	if tt.Clipboard() != "" {
		t.Errorf("the words are still on the clipboard after half a minute: %q", tt.Clipboard())
	}
}

// TestNoExportWithoutAWayToSave: a screen that offers a button which cannot
// do anything is a screen that lies. With no export to call, there is no
// button — and the words are still there to copy by hand.
func TestNoExportWithoutAWayToSave(t *testing.T) {
	a, err := New(Options{Home: emptyHome(t), Rand: createReader()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Lock)
	tt := newTester(t, a)
	if err := tt.Click("新建账号"); err != nil {
		t.Fatalf("clicking 新建账号: %v", err)
	}
	tt.Frame()
	if tt.HasText("存成文件") {
		t.Errorf("an export is offered with nothing to save with: %v", tt.Texts())
	}
	if !tt.HasText("我已抄好") {
		t.Errorf("the way forward is gone: %v", tt.Texts())
	}
}
