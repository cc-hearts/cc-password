// Package ccapp is the application shell: the screens that turn a passphrase
// into an open vault, and the vault into something on screen.
//
// Everything that touches the machine — the home directory, the randomness,
// the work that takes a second, the window's content protection — arrives
// through Options, so the screens are tested without a window, a disk or a
// clock of their own. The root key lives in the session and the vault, and
// this package only holds the handles to them: it never writes one anywhere.
package ccapp

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo/ui"

	"ccpass/internal/identity"
	"ccpass/internal/store"
	"ccpass/internal/vault"
	"ccpass/internal/vaultcrypto"
)

// Options is what the shell needs from the machine it runs on. The zero value
// is a working app on the real machine: the tests set the fields so they do
// not touch one.
type Options struct {
	// Home is the directory holding the keyfile and the database. Empty
	// means the store's own: CCPASS_HOME, or the user's config directory.
	Home string
	// Rand is the randomness an account is made with. Empty means
	// crypto/rand.Reader.
	Rand io.Reader
	// Run runs work off the main thread and hands the result back on it, as
	// Window.Update does. Empty means the work runs where it is called,
	// which is what a test wants.
	Run func(work, done func())
	// Protect turns the window's content protection on and off, as a screen
	// showing a mnemonic needs. Nil means there is nothing to protect.
	Protect func(on bool)
}

// App is the shell. It holds the state the screens are built from: whether
// there is a keyfile, whether the vault is open, and what the screen the user
// is on holds.
type App struct {
	opts Options
	home string

	// screen is which screen is showing: the start screen, or the recovery
	// one.
	screen screen
	// unlock holds what the unlock screen has been given.
	unlock unlockState
	// recover holds what the recovery screen has been given.
	recover recoverState
	// create holds what the first-run flow has been given.
	create createState
	// keyfile is the keyfile that was found in home, if there was one. Its
	// absence is what puts the app on the first-run path.
	keyfile *identity.Keyfile
	// account is the account the local data belongs to, when it is known
	// from a keyfile. A recovery into a home whose data belongs to someone
	// else is refused, and this is what tells them apart.
	accountID string

	// db and v are the open store and vault, nil while locked.
	db *store.DB
	v  *vault.Vault
	// session holds the root key while the vault is open.
	session *identity.Session
	// busy is the work that is running off the main thread.
	busy string
	// err is what the current screen has to say, in the user's words.
	err string
	// pending is what the work off the main thread has produced and the main
	// thread has not taken yet. The work writes it and touches nothing else:
	// the screens' own fields belong to the main thread, and a frame being
	// built while the work runs must never see half of a change.
	pending outcome
}

// outcome is what the work off the main thread produces. It is written there
// and read here, once the work has handed itself back, so it is the only thing
// the two share.
type outcome struct {
	db      *store.DB
	v       *vault.Vault
	session *identity.Session
	// keyfile is set when the work made an account, which is what tells a
	// finished recovery or creation from a finished unlock.
	keyfile *identity.Keyfile
	// created is whether the work made a new account from generated words,
	// which is the one work whose words are kept when it fails.
	created bool
	// recovered is whether the work was a recovery.
	recovered bool
	err       error
}

// screen is one of the app's screens.
type screen int

const (
	// screenStart is the everyday screen: a passphrase when there is a
	// keyfile, and a new account when there is not.
	screenStart screen = iota
	// screenRecover is the recovery screen: a mnemonic and a new passphrase.
	screenRecover
	// screenCreate is the first-run flow: the words a new account is made of.
	screenCreate
)

// unlockState is what the unlock screen holds between frames.
type unlockState struct {
	passphrase string
}

// recoverState is what the recovery screen holds between frames. Every field
// is cleared once the recovery has succeeded, so nothing secret is left in
// memory for longer than it is needed.
type recoverState struct {
	mnemonic   string
	passphrase string
	again      string
	// revealed is whether the mnemonic is showing, which it does only while
	// the reveal button is held.
	revealed bool
}

// New builds the shell for a home directory and looks for a keyfile in it. A
// home that is not a directory is refused here, where the caller can be told,
// rather than at the first write.
func New(opts Options) (*App, error) {
	home := opts.Home
	if home == "" {
		var err error
		if home, err = store.Home(); err != nil {
			return nil, err
		}
	}
	info, err := os.Stat(home)
	switch {
	case err == nil && !info.IsDir():
		return nil, fmt.Errorf("ccapp: %s is not a directory", home)
	case err != nil && !os.IsNotExist(err):
		return nil, fmt.Errorf("ccapp: reading %s: %w", home, err)
	}
	a := &App{opts: opts, home: home}
	if info != nil {
		// A keyfile that is there is read now, so a screen knows what the
		// account is before anything is typed. A file this build cannot
		// parse is left for the unlock to refuse, with the same message a
		// wrong passphrase gets.
		if kf, err := identity.LoadKeyfile(filepath.Join(home, store.KeyfileName)); err == nil {
			a.keyfile, a.accountID = kf, kf.AccountID
		}
	}
	return a, nil
}

// View builds the screen the user is on. It runs on the main thread whenever
// the window needs a frame, so it only reads state and never waits on work.
func (a *App) View(c *ui.Context) {
	ui.Column(c).Fill().Padding(24).Children(func() {
		if a.v != nil {
			a.mainView(c)
			return
		}
		switch a.screen {
		case screenRecover:
			a.recoverView(c)
		case screenCreate:
			a.createView(c)
		default:
			a.startView(c)
		}
	})
}

// recoverView is the recovery screen: a mnemonic and a new passphrase for it.
func (a *App) recoverView(c *ui.Context) {
	ui.Column(c).Gap(12).Children(func() {
		ui.Text(c, "恢复账号").FontSize(20).Bold()
		ui.Text(c, "把 24 个词按顺序填进来，再给这个设备设一个新口令。").TextColor(c.Theme().TextMuted)
		ui.Field(c, "助记词", func() {
			// One line, and masked: a mnemonic is pasted from a note with the
			// newlines that paste carries, and a single-line field folds them
			// to spaces on its own. Only a single-line field can be masked at
			// all, which is why this is not a text area.
			in := ui.TextInput(c, &a.recover.mnemonic).Password().Fill()
			if !a.recover.revealed {
				in.Password()
			}
			in.Placeholder("24 个词，空格分开")
		})
		// The word count is live: a user pasting 23 words should see it before
		// they press anything.
		ui.Textf(c, "已填 %d 个词", wordCount(a.recover.mnemonic)).TextColor(c.Theme().TextMuted)
		// Held, not toggled: the mnemonic shows only while the button is down,
		// so a shoulder-surfer gets one glance and no more.
		reveal := ui.Button(c, "按住显示")
		if reveal.Pressed() {
			a.recover.revealed = true
		} else {
			a.recover.revealed = false
		}
		ui.Field(c, "新口令", func() {
			ui.TextInput(c, &a.recover.passphrase).Password().Fill()
		})
		ui.Field(c, "再输一次", func() {
			ui.TextInput(c, &a.recover.again).Password().Fill()
		})
		if a.busy != "" {
			ui.Text(c, a.busy).TextColor(c.Theme().TextMuted)
		} else {
			ui.Text(c, a.err).TextColor(c.Theme().TextMuted)
		}
		ui.Row(c).Gap(8).Children(func() {
			if ui.PrimaryButton(c, "开始恢复").Disabled(a.busy != "").Clicked() {
				a.recoverAccount()
			}
			ui.Button(c, "返回").Disabled(a.busy != "").OnClick(func() {
				a.screen, a.err = screenStart, ""
				a.clearRecover()
				a.protect(false)
			})
		})
	})
}

// mainView is the placeholder the vault sits in until the list and the detail
// are built. It is here so an unlock has somewhere to arrive.
func (a *App) mainView(c *ui.Context) {
	ui.Column(c).Fill().Gap(12).Children(func() {
		ui.Row(c).Children(func() {
			ui.Text(c, "ccpass").FontSize(20).Bold().Grow(1)
			if ui.Button(c, "锁定").Clicked() {
				a.Lock()
			}
		})
		if a.err != "" {
			ui.Text(c, a.err).TextColor(c.Theme().TextMuted)
			return
		}
		for _, vi := range a.v.Vaults() {
			ui.Text(c, vi.Name)
		}
	})
}

// recoverAccount makes a keyfile from the mnemonic that was typed and a new
// passphrase, and opens it.
func (a *App) recoverAccount() {
	if a.busy != "" {
		return
	}
	// The three mnemonic errors each say exactly what is wrong, down to the
	// position of the word, and nothing about the words themselves.
	if err := vaultcrypto.ValidateMnemonic(a.recover.mnemonic); err != nil {
		var me *vaultcrypto.MnemonicError
		if errors.As(err, &me) {
			switch {
			case errors.Is(me.Err, vaultcrypto.ErrMnemonicLength):
				a.err = fmt.Sprintf("助记词应为 24 个词，现在是 %d 个", me.Words)
			case errors.Is(me.Err, vaultcrypto.ErrMnemonicWord):
				a.err = fmt.Sprintf("第 %d 个词不在词表里", me.Position)
			default:
				a.err = "助记词校验和不对，请检查是否抄错"
			}
			return
		}
		a.err = "助记词不对，请检查是否抄错"
		return
	}
	if a.recover.passphrase != a.recover.again {
		a.err = "两次输入的口令不一致"
		return
	}
	if err := identity.CheckPassphrase(a.recover.passphrase); err != nil {
		a.err = "口令太短，至少 8 个字符"
		return
	}
	// Local data that belongs to another account is left alone. Recovering a
	// different mnemonic into it would write a keyfile whose vault cannot be
	// read, so the user is told which files are at stake instead.
	if a.accountID != "" && a.newAccountID() != a.accountID {
		a.err = fmt.Sprintf("这个目录里的 %s 和 %s 属于另一个账号，恢复会读不了它们。请先备份或换个目录。", store.KeyfileName, store.DBName)
		return
	}
	mnemonic, passphrase := a.recover.mnemonic, a.recover.passphrase
	home, rand := a.home, a.rand()
	a.busy, a.err = "正在恢复", ""
	a.run(func() {
		out := outcome{recovered: true}
		var kf *identity.Keyfile
		var session *identity.Session
		kf, session, out.err = identity.NewAccount(mnemonic, passphrase, nowUTC(), rand)
		if out.err == nil {
			out.err = identity.SaveKeyfile(filepath.Join(home, store.KeyfileName), kf)
		}
		if out.err == nil {
			out.db, out.v, out.err = openVault(home, session)
		}
		out.keyfile, out.session = kf, session
		a.pending = out
	}, a.finish)
}

// newAccountID is the account id the mnemonic on the recovery screen derives
// to. It costs one PBKDF2 over the mnemonic, which is why it is only asked for
// when there is local data to compare against.
func (a *App) newAccountID() string {
	root, accountID, _, err := identity.DeriveAccount(a.recover.mnemonic)
	if err != nil {
		return ""
	}
	clear(root)
	return accountID
}

// openVault opens the store and the vault under home for a session. It runs
// off the main thread, so it touches no screen's state: it hands back the
// handles for the main thread to keep, and closes what it opened if the vault
// refuses them.
func openVault(home string, session *identity.Session) (*store.DB, *vault.Vault, error) {
	db, err := store.Open(filepath.Join(home, store.DBName))
	if err != nil {
		return nil, nil, err
	}
	v, err := vault.Open(db, session, nowUnix())
	if err != nil {
		db.Close()
		return nil, nil, err
	}
	return db, v, nil
}

// finish is what the main thread does with the work's result: it takes the
// handles the work opened, or it says what went wrong.
func (a *App) finish() {
	p := a.pending
	a.pending = outcome{}
	a.busy = ""
	if p.err != nil {
		// Nothing the work opened stays open behind a failure: a store left
		// open holds the file, and the session holds the root key.
		if p.v != nil {
			p.v.Close()
		}
		if p.db != nil {
			p.db.Close()
		}
		if p.session != nil {
			p.session.Lock()
		}
		if p.recovered || p.created {
			// The mnemonic is the account, and a full disk is no reason to
			// make someone type all 24 words again, or to throw away words they
			// have already copied. Nothing was opened, so there is nothing to
			// lock away.
			a.err = failMessage(p.err)
			return
		}
		// Lock empties the screens and the fields, so the message is set after
		// it: one set before would be wiped on the way out. What is left is the
		// start screen saying one thing.
		a.Lock()
		a.err = failMessage(p.err)
		return
	}
	a.db, a.v, a.session = p.db, p.v, p.session
	if p.recovered || p.created {
		a.keyfile, a.accountID = p.keyfile, p.keyfile.AccountID
		if p.created {
			a.clearCreate()
		} else {
			a.clearRecover()
		}
		a.protect(false)
		return
	}
	a.clearUnlock()
	a.err = ""
}

// failMessage is what a failure says, in the user's words. Every
// authentication failure is the same message, whichever half of it was wrong:
// a wrong passphrase and a damaged keyfile are told apart by nobody, and a
// vault that will not open says only that.
func failMessage(err error) string {
	switch {
	case errors.Is(err, identity.ErrUnlock):
		return "口令不对，或密钥文件已损坏"
	case errors.Is(err, vault.ErrOtherAccount):
		return fmt.Sprintf("这个目录里的 %s 和 %s 属于另一个账号。", store.KeyfileName, store.DBName)
	}
	var corrupt *vault.CorruptError
	if errors.As(err, &corrupt) {
		return fmt.Sprintf("有 %d 条数据解不开，可能是文件被改过。已锁定，没有显示任何内容。", corrupt.Count)
	}
	return "打不开，可能是文件被改过。已锁定，没有显示任何内容。"
}

// run runs work the way the machine runs it: off the main thread, with the
// result handed back on it. Without a runner the two halves run one after the
// other where they stand, which is what a test wants.
func (a *App) run(work, done func()) {
	if a.opts.Run != nil {
		a.opts.Run(work, done)
		return
	}
	work()
	done()
}

// rand is the randomness this app was given, or the real source.
func (a *App) rand() io.Reader {
	if a.opts.Rand != nil {
		return a.opts.Rand
	}
	return rand.Reader
}

// protect turns the window's content protection on and off, when there is a
// window to ask.
func (a *App) protect(on bool) {
	if a.opts.Protect != nil {
		a.opts.Protect(on)
	}
}

// clearUnlock empties the unlock screen's fields: once the vault is open
// there is no reason to hold the passphrase a second longer.
func (a *App) clearUnlock() {
	a.unlock.passphrase = ""
}

// clearRecover empties the recovery screen's fields, mnemonic first: the
// words are the account, and they are gone the moment they are no longer
// needed.
func (a *App) clearRecover() {
	a.recover.mnemonic = ""
	a.recover.passphrase = ""
	a.recover.again = ""
	a.recover.revealed = false
}

// Lock drops the vault and the session, and returns to the start screen. The
// root key leaves memory with them: nothing this app holds can read the vault
// again until the passphrase is typed once more.
func (a *App) Lock() {
	if a.v != nil {
		a.v.Close()
	}
	if a.db != nil {
		a.db.Close()
	}
	if a.session != nil {
		a.session.Lock()
	}
	a.v, a.db, a.session = nil, nil, nil
	a.busy, a.err = "", ""
	a.clearUnlock()
	a.clearRecover()
	a.clearCreate()
	a.screen = screenStart
	a.protect(false)
}

// Unlocked reports whether the vault is open.
func (a *App) Unlocked() bool { return a.v != nil }

// Vaults is the names of the open vault's vaults, in the order they were made.
func (a *App) Vaults() []string {
	if a.v == nil {
		return nil
	}
	out := make([]string, 0, 4)
	for _, vi := range a.v.Vaults() {
		out = append(out, vi.ID)
	}
	return out
}

// wordCount is how many words a mnemonic field holds, counted the way a user
// counts them: every run of white space is one break, so a pasted note with
// its newlines counts the same as one typed with spaces.
func wordCount(s string) int {
	n := 0
	inWord := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			inWord = false
			continue
		}
		if !inWord {
			n++
			inWord = true
		}
	}
	return n
}

// nowUTC is the time the app thinks it is. It is the real clock: the plan has
// no injected one, and every timestamp it produces is routing metadata that
// only ever shows on screen.
func nowUTC() time.Time { return time.Now().UTC() }

// nowUnix is nowUTC as the unix seconds the store takes.
func nowUnix() int64 { return nowUTC().Unix() }
