package ccapp

import (
	"crypto/rand"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"

	"bipPass/internal/identity"
	"bipPass/internal/store"
	"bipPass/internal/vaultcrypto"
)

// createStep is one step of the first-run flow. They run in order, and each
// one's only way forward is the next: nothing here can be skipped, because an
// account nobody has written down is an account nobody can recover.
type createStep int

const (
	// stepShowWords shows the 24 words to copy down.
	stepShowWords createStep = iota
	// stepRetype asks for three of them back, to catch a mistranscribed copy.
	stepRetype
	// stepPassphrase sets the passphrase this device unlocks with.
	stepPassphrase
)

// retypeField is one of the retype step's questions: which word it asks for,
// and what has been typed into it.
type retypeField struct {
	position int
	value    string
}

// createState is what the first-run flow holds between frames. The words are
// generated once and kept only until the account is made: they are the
// account, and they leave with the keyfile.
type createState struct {
	step createStep
	// words is the mnemonic being shown, in order.
	words []string
	// asks is what the retype step asks for, in the order it asks.
	asks []retypeField
	// passphrase and again are the two passphrase fields.
	passphrase, again string
	// asking is whether the words step is asking before it writes the words to
	// a file. It is a step rather than a second click on the same button: the
	// words are the account, and a file holding them is the one copy that
	// outlives this window and this machine.
	asking bool
}

// createView is the first-run flow: the words, then three of them typed back,
// then a passphrase twice. There is no way past any step, and the way back
// from any of them starts the flow over.
func (a *App) createView(c *ui.Context) {
	ui.Column(c).Fill().Gap(12).Children(func() {
		switch a.create.step {
		case stepRetype:
			a.retypeStep(c)
		case stepPassphrase:
			a.passphraseStep(c)
		default:
			a.wordsStep(c)
		}
	})
}

// wordsStep shows the mnemonic. They are the account, so the window's content
// protection is on while they are up: no screenshot, no recording, no screen
// sharing sees them. They leave the screen at the next step.
func (a *App) wordsStep(c *ui.Context) {
	ui.Column(c).Gap(12).Children(func() {
		ui.Text(c, "新建账号").FontSize(20).Bold()
		ui.Text(c, "这 24 个词就是你的账号。按顺序抄在纸上，收在别人找不到的地方——丢了它，谁也打不开这个账号。").TextColor(c.Theme().TextMuted)
		// Two to a line. One to a line would push the button off a 640-point
		// window, and the number is what the next step asks for, so it stays
		// beside its word.
		for i := 0; i < len(a.create.words); i += 2 {
			row := a.create.words[i:]
			ui.Row(c).Gap(24).Children(func() {
				for j, w := range row[:min(2, len(row))] {
					ui.Text(c, fmt.Sprintf("%d. %s", i+j+1, w))
				}
			})
		}
		// A file and the clipboard, offered while the words are the only copy
		// there is. Both are a way to keep the words, not a way past the retype
		// check below: an export is somebody's own copy of their own account,
		// and the check is that they read it.
		a.exportWords(c)
		a.busyView(c)
		if a.busy == "" {
			ui.Text(c, a.err).TextColor(c.Theme().TextMuted)
		}
		ui.Row(c).Gap(8).Children(func() {
			if ui.PrimaryButton(c, "我已抄好").Disabled(a.busy != "").Clicked() {
				a.toRetype()
			}
			ui.Button(c, "返回").Disabled(a.busy != "").OnClick(func() {
				a.leaveCreate()
			})
		})
	})
}

// exportWords is the row that gets the words out of this app: into a file on
// the user's machine, or onto the clipboard.
//
// Both are offered only when the machine can do them. A 存成文件 with no save
// dialog to open is a button that lies, so it is not there at all; the words
// stay on screen either way, because paper is always an option.
func (a *App) exportWords(c *ui.Context) {
	ui.Row(c).Gap(8).Children(func() {
		if a.opts.Export != nil {
			if ui.Button(c, "存成文件").Disabled(a.busy != "").Clicked() {
				// Ask first, and take either answer. A file holding the words
				// outlives this window, this machine and this app: it is the one
				// copy nobody has to be asked for twice about.
				a.create.asking = true
			}
		}
		if ui.Button(c, "复制").Disabled(a.busy != "").Clicked() {
			a.copyWords(c)
		}
	})
	a.exportConfirm(c)
}

// exportConfirm asks whether to write the words to a file, and takes either
// answer. It is a modal of the library's own, for the same reason a delete's is:
// the words are the account, and the cost of one mis-click is a file holding
// them somewhere nobody chose. It is also the one question in the app whose
// answer is worth reading, so it gets a dialog that dims the rest of the screen
// until it is answered rather than a line beside the words.
func (a *App) exportConfirm(c *ui.Context) {
	if !a.create.asking {
		return
	}
	switch ui.AlertDialog(c, &a.create.asking, "存到文件里？",
		"这 24 个词就是账号本身，抄走它的人就拥有了它。文件放在哪由你定。", "取消", "确定保存") {
	case 1:
		a.saveWords(c)
	}
}

// saveWords hands the mnemonic to the machine to write where the user picks.
// The words are read here, on the main thread, before anything is handed over:
// they belong to this screen, and a dialog is a place the app is not in control
// of while it is up.
func (a *App) saveWords(c *ui.Context) {
	if a.opts.Export == nil {
		return
	}
	a.create.asking = false
	mnemonic := strings.Join(a.create.words, " ")
	if err := a.opts.Export(mnemonic); err != nil {
		// Nothing was written, so the words are still the only copy: they stay
		// on screen, and the screen says the file did not happen. A user who
		// pressed the button and was told nothing would believe they have a
		// copy they do not have. This one is a line on the screen rather than a
		// toast: it is not news that the words were saved, it is news that they
		// were not, and it should still be there when the eyes come back.
		a.err = "没存上，纸还在屏幕上，再抄一遍也来得及。"
		return
	}
	c.Toast("已存到你选的地方。")
}

// copyWords puts the mnemonic on the clipboard and says so. The clipboard is
// the system's the moment it is there, which is why the copy is the same
// thirty-second one every other copy gets.
func (a *App) copyWords(c *ui.Context) {
	a.copySecret(c, strings.Join(a.create.words, " "))
	c.Toast("已复制，半分钟后自动清掉。")
}

// retypeStep asks for three of the words back, at positions chosen at random.
// The same position is never asked for twice, and there is no way forward
// without them: the copy on paper is the only copy there is.
func (a *App) retypeStep(c *ui.Context) {
	ui.Column(c).Gap(12).Children(func() {
		ui.Text(c, "确认你抄对了").FontSize(20).Bold()
		ui.Text(c, "把下面三个词填进来，第几个就是纸上的第几个。").TextColor(c.Theme().TextMuted)
		for i := range a.create.asks {
			// A pointer into the field itself, so what is typed lands in the
			// state the next frame reads.
			f := &a.create.asks[i]
			ui.Field(c, fmt.Sprintf("第 %d 个词", f.position), func() {
				ui.TextInput(c, &f.value).Fill()
			})
		}
		a.busyView(c)
		if a.busy == "" {
			ui.Text(c, a.err).TextColor(c.Theme().TextMuted)
		}
		ui.Row(c).Gap(8).Children(func() {
			if ui.PrimaryButton(c, "确认").Disabled(a.busy != "").Clicked() {
				a.checkRetype()
			}
			ui.Button(c, "返回").Disabled(a.busy != "").OnClick(func() {
				a.create.step, a.err = stepShowWords, ""
			})
		})
	})
}

// passphraseStep sets the passphrase this device unlocks with. It is asked for
// twice: it is the only thing standing between the file on disk and the root
// key, and nobody can reset it.
func (a *App) passphraseStep(c *ui.Context) {
	ui.Column(c).Gap(12).Children(func() {
		ui.Text(c, "给这个设备设个口令").FontSize(20).Bold()
		ui.Text(c, "口令只在这台设备上，用来保护密钥文件。它和那 24 个词是两件事。").TextColor(c.Theme().TextMuted)
		ui.Field(c, "新口令", func() {
			ui.TextInput(c, &a.create.passphrase).Password().Fill()
		})
		ui.Field(c, "再输一次", func() {
			ui.TextInput(c, &a.create.again).Password().Fill()
		})
		a.busyView(c)
		if a.busy == "" {
			ui.Text(c, a.err).TextColor(c.Theme().TextMuted)
		}
		ui.Row(c).Gap(8).Children(func() {
			if ui.PrimaryButton(c, "创建").Disabled(a.busy != "").Clicked() {
				a.createAccount()
			}
			ui.Button(c, "返回").Disabled(a.busy != "").OnClick(func() {
				a.create.step, a.err = stepRetype, ""
			})
		})
	})
}

// startCreate generates the mnemonic the flow shows and puts the app on the
// words step. It runs here, on the main thread, with the randomness the app was
// given.
func (a *App) startCreate() {
	mnemonic, err := vaultcrypto.GenerateMnemonic(a.rand())
	if err != nil {
		a.err = "生成不了助记词，请重试。"
		return
	}
	a.create = createState{step: stepShowWords, words: strings.Fields(mnemonic)}
	a.screen, a.err = screenCreate, ""
	a.protect(true)
}

// toRetype moves to the retype step, choosing which three words to ask for.
func (a *App) toRetype() {
	if a.busy != "" {
		return
	}
	asks := pickThree(len(a.create.words), a.rand())
	fields := make([]retypeField, len(asks))
	for i, pos := range asks {
		fields[i] = retypeField{position: pos}
	}
	a.create.asks = fields
	a.create.step, a.err = stepRetype, ""
}

// checkRetype compares the three words that were typed against the ones shown.
func (a *App) checkRetype() {
	if a.busy != "" {
		return
	}
	for _, f := range a.create.asks {
		if strings.TrimSpace(f.value) != a.create.words[f.position-1] {
			// Which position, and nothing about what was typed: a wrong word
			// is a copy to check on paper, not a message to echo back.
			a.err = fmt.Sprintf("第 %d 个词不对，对着纸上再看看。", f.position)
			return
		}
	}
	a.create.step, a.err = stepPassphrase, ""
}

// createAccount makes the keyfile from the words and the passphrase, and opens
// the vault. The words leave with it.
func (a *App) createAccount() {
	if a.busy != "" {
		return
	}
	if a.create.passphrase != a.create.again {
		a.err = "两次输入的口令不一致"
		return
	}
	if err := identity.CheckPassphrase(a.create.passphrase); err != nil {
		a.err = "口令太短，至少 8 个字符"
		return
	}
	// What the work needs is read here, on the main thread, before it starts:
	// the words belong to the screen, and the work runs with what it was
	// handed rather than reaching into a screen being drawn.
	mnemonic, passphrase := strings.Join(a.create.words, " "), a.create.passphrase
	home, r := a.home, a.rand()
	a.busy, a.err = "正在创建", ""
	a.run(func() {
		out := outcome{created: true}
		kf, session, err := identity.NewAccount(mnemonic, passphrase, nowUTC(), r)
		out.keyfile, out.session, out.err = kf, session, err
		if err == nil {
			out.err = identity.SaveKeyfile(filepath.Join(home, store.KeyfileName), kf)
		}
		if out.err == nil {
			out.db, out.v, out.err = openVault(home, session)
		}
		a.pending = out
	}, a.finish)
}

// leaveCreate leaves the first-run flow. The words are dropped rather than
// kept for a second attempt: they were shown once, and an account nobody has
// written down is one nobody has.
func (a *App) leaveCreate() {
	a.clearCreate()
	a.screen, a.err = screenStart, ""
	a.protect(false)
}

// clearCreate empties the create flow's state, words first: they are the
// account, and they are gone the moment they are no longer needed.
func (a *App) clearCreate() {
	for i := range a.create.words {
		a.create.words[i] = ""
	}
	a.create = createState{}
}

// pickThree chooses three distinct positions between 1 and n, in ascending
// order. The same position twice would let one correct word cover a mistake
// in another, and the order is ascending so the fields read in the order the
// words appear on paper.
func pickThree(n int, r io.Reader) []int {
	if n < 3 {
		return nil
	}
	if r == nil {
		r = rand.Reader
	}
	// A partial Fisher-Yates over the positions, taking the first three: every
	// set of three is as likely as any other, and no position can come up
	// twice.
	positions := make([]int, n)
	for i := range positions {
		positions[i] = i + 1
	}
	for i := 0; i < 3; i++ {
		j := i + randBelow(r, n-i)
		positions[i], positions[j] = positions[j], positions[i]
	}
	out := append([]int{}, positions[:3]...)
	sort.Ints(out)
	return out
}

// randBelow is a uniform number below n, drawn from r. It rejects rather than
// taking a remainder, which would favour the low numbers.
func randBelow(r io.Reader, n int) int {
	if n <= 1 {
		return 0
	}
	bits := 1
	for 1<<uint(bits) < uint(n) {
		bits++
	}
	buf := make([]byte, (bits+7)/8)
	for {
		if _, err := io.ReadFull(r, buf); err != nil {
			return 0
		}
		v := 0
		for _, b := range buf {
			v = v<<8 | int(b)
		}
		v >>= uint(len(buf)*8 - bits)
		if v < n {
			return v
		}
	}
}
