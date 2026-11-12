// Command bipPass is the password manager the plan describes: one binary, a
// window drawn by the Go UI, and a vault that stays encrypted on disk.
//
// The key never leaves this process's memory. What the shell needs from the
// machine — the window's content protection, the main thread a finished
// unlock hands its result back on — is wired here, so the screens themselves
// are tested without any of it.
package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"bipPass/internal/ccapp"
)

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
