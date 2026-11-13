// Command bipPass is the password manager the plan describes: one binary, a
// window drawn by the Go UI, and a vault that stays encrypted on disk.
//
// The key never leaves this process's memory. What the shell needs from the
// machine — the window's content protection, the main thread a finished
// unlock hands its result back on — is wired here, so the screens themselves
// are tested without any of it.
package main

import (
	"errors"
	"log"
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"bipPass/internal/ccapp"
)

// saveMnemonic asks where to put a file holding the mnemonic and writes it
// there. The dialog is the system's and the write is the app's own, so the
// words are handed to the dialog and never logged.
func saveMnemonic(win *mygo.Window, mnemonic string) error {
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      win,
		Title:       "存下助记词",
		Message:     "这 24 个词就是你的账号，抄走它的人就拥有了它。",
		ButtonLabel: "存储",
		DefaultPath: "bipPass-助记词.txt",
		Filters:     []mygo.FileFilter{{Name: "文本文件", Extensions: []string{"txt"}}},
	})
	if err != nil {
		return err
	}
	// A cancelled dialog is not a failure: nothing was asked for, and nothing
	// was written. It is reported as one so the screen does not claim a file
	// that does not exist.
	if path == "" {
		return errNoSave
	}
	return writeMnemonic(path, mnemonic)
}

// writeMnemonic puts the mnemonic in a file and closes every door but the
// owner's.
//
// The chmod is after the write, not a mode handed to it: WriteFile only applies
// its mode when it creates the file, so a path the user picked that already
// holds a world-readable file would keep those permissions and the words would
// sit in it for anyone to read. The words are on disk for the moment between
// the two either way — a write is a write — but nothing after that leaves them
// open.
func writeMnemonic(path, mnemonic string) error {
	if err := os.WriteFile(path, []byte(mnemonic+"\n"), 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

// errNoSave is what a cancelled save dialog reports. It is not an error of the
// machine's, which is why it is not one of the io or os ones: it says the user
// changed their mind, and the screen says so too.
var errNoSave = errors.New("no file chosen")

func main() {
	// One vault, one window: a second launch hands its arguments to this one
	// and exits, rather than opening a second window on the same file.
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}

	var (
		app *ccapp.App
		win *mygo.Window
	)
	mygo.App.WhenReady(func() {
		var err error
		app, err = ccapp.New(ccapp.Options{
			// Work that takes a second — Argon2id, and the vault's own
			// decryption — goes off the main thread and hands its result
			// back on it, so the window keeps answering while it runs.
			Run: func(work, done func()) {
				go func() {
					work()
					win.Update(done)
				}()
			},
			// A mnemonic on screen is the account: it is kept out of
			// screenshots and recordings while it is there.
			Protect: func(on bool) { win.SetContentProtection(on) },
			// The words are written where the user says, as a file that is
			// exactly the mnemonic: it can be pasted straight back into the
			// recovery screen, and it holds nothing else about the account.
			Export: func(text string) error { return saveMnemonic(win, text) },
		})
		if err != nil {
			log.Fatalf("bipPass: %v", err)
		}
		win = mygo.NewWindow(mygo.WindowOptions{
			Title:    "bipPass",
			Width:    960,
			Height:   640,
			StateKey: "main",
			Content:  ui.View(app.View),
		})
		win.Show()
		win.Focus()
	})

	// The screen locking is the same as the user locking the vault: the root
	// key leaves memory rather than sitting there behind a lock screen.
	mygo.Power.OnLockScreen(func() {
		if app != nil {
			app.Lock()
		}
	})
	mygo.Power.OnSuspend(func() {
		if app != nil {
			app.Lock()
		}
	})
	// A second launch brings this window forward.
	mygo.App.OnSecondInstance(func(args []string, workingDir string) {
		if win != nil {
			win.Show()
			win.Focus()
		}
	})

	// The window closing is a lock: nothing stays open behind it.
	if err := mygo.App.Run(); err != nil {
		log.Fatalf("bipPass: %v", err)
	}
	if app != nil {
		app.Lock()
	}
}
