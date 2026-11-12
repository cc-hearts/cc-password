package ccapp

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"bipPass/internal/vaultcrypto"
)

// genState is what the generator screen holds between frames: the options the
// password on screen was drawn with, the password itself, and what the length
// field holds.
type genState struct {
	// open is whether the screen is showing at all. It is a flag of its own
	// rather than a test on the other fields, because an empty form is a form
	// nobody has typed in and a closed one is not the same thing.
	open bool
	// itemID is the item the password can replace, and is empty when the
	// generator was opened from the list with nothing open. It is the item the
	// screen was opened from rather than a choice made here: a generator that
	// could write to any item would be a picker nobody asked for.
	itemID string
	opts   vaultcrypto.GenOptions
	// shown is what the password on screen was drawn with. Another one is drawn
	// when the options move away from it, or when 重新生成 says so — never every
	// frame: a password that changed on its own between two frames is one
	// nobody can read, and one a copy button cannot be trusted about.
	shown vaultcrypto.GenOptions
	// again asks for another password with the options exactly as they are.
	again bool
	// length is what the length field holds, as text. It is text rather than an
	// int of its own because a field being typed into holds "3" on its way to
	// "32", and a length the app has already rounded is a field that fights the
	// person typing in it.
	length string
	// password is the one on screen, in the clear. It is the one place the plan
	// allows a password outside the sealed blob and the form, and it is emptied
	// when the screen closes: it was made to be copied or saved, and after that
	// there is nothing to keep it for.
	password string
	// bits is the entropy of these options, and strength what it is called.
	bits     float64
	strength string
	// copied is what the last copy said, so somebody who cannot see the
	// clipboard knows the copy happened.
	copied string
}

// newGenState is the generator as it opens: the default options, and no item to
// write to unless one was open.
func newGenState(itemID string) genState {
	opts := vaultcrypto.DefaultGenOptions()
	return genState{open: true, itemID: itemID, opts: opts, length: strconv.Itoa(opts.Length)}
}

// openGen opens the generator over whatever screen it was opened from, and
// remembers which item — if any — its password can replace.
func (a *App) openGen(itemID string) {
	a.gen = newGenState(itemID)
	a.err = ""
}

// closeGen leaves the generator and empties it. The password goes with it: it
// has been copied, saved or thrown away, and either way there is no reason to
// hold it a second longer.
func (a *App) closeGen() {
	a.gen = genState{}
}

// genView is the generator: a password, what it is made of, and the ways to
// make another one. It is a screen over the list and over the detail, because
// it is opened from either and it is not a place anybody stays.
func (a *App) genView(c *ui.Context) {
	a.readGenLength()
	a.genDraw()
	ui.Column(c).Fill().Gap(12).Children(func() {
		ui.Row(c).Gap(8).Children(func() {
			if ui.Button(c, "返回").Clicked() {
				a.closeGen()
			}
			ui.Text(c, "生成密码").FontSize(20).Bold().Grow(1)
		})
		// The password is the one thing on this screen, so it is the biggest
		// thing on it, and it is shown rather than masked: it was made to be
		// read or copied, and a password behind a hold-to-reveal is one nobody
		// can check before they use it somewhere.
		ui.Text(c, a.gen.password).FontSize(28).SingleLine().Grow(1)
		// The strength is the number of bits with a name beside it. 强 on its
		// own is a claim, and the bits are the reason for it — and the reason
		// turning a class off costs something.
		if p := a.gen.problem(); p != "" {
			ui.Text(c, p).TextColor(c.Theme().TextMuted)
		} else {
			ui.Text(c, fmt.Sprintf("%s · %.0f 位熵", a.gen.strength, a.gen.bits)).TextColor(c.Theme().TextMuted)
		}
		ui.Field(c, "长度", func() {
			ui.TextInput(c, &a.gen.length).SingleLine().Fill()
		})
		ui.Row(c).Gap(8).Children(func() {
			a.genClass(c, "小写", &a.gen.opts.Lower)
			a.genClass(c, "大写", &a.gen.opts.Upper)
			a.genClass(c, "数字", &a.gen.opts.Digits)
			a.genClass(c, "符号", &a.gen.opts.Symbols)
		})
		// The ambiguous characters are the ones that look alike in a password
		// field's font, and a password with them in it is one somebody types
		// wrong on the site it was made for.
		excl := ui.Toggle(c, &a.gen.opts.NoAmbiguous, "排除易混字符")
		_ = excl
		ui.Row(c).Gap(8).Children(func() {
			if ui.PrimaryButton(c, "重新生成").Clicked() {
				a.gen.again = true
			}
			if ui.Button(c, "复制").Clicked() {
				a.copyGen(c)
			}
			// Offered only when there is an item to put it in: a generator
			// opened from the list has nothing to replace, and a button that
			// would do nothing is a button that lies.
			if a.gen.itemID != "" {
				if ui.Button(c, "用作当前条目密码").Clicked() {
					a.useGenerated()
				}
			}
		})
		if a.gen.copied != "" {
			ui.Text(c, a.gen.copied).TextColor(c.Theme().TextMuted)
		}
		if a.err != "" {
			ui.Text(c, a.err).TextColor(c.Theme().TextMuted)
		}
	})
}

// genClass is one character class's switch. It is a toggle rather than a
// checkbox because it is a setting that is on or off, and the four of them read
// as one control.
func (a *App) genClass(c *ui.Context, label string, on *bool) {
	ui.Toggle(c, on, label)
}

// readGenLength puts what the length field holds into the options. A field that
// is not a number is a length of nothing, which is out of range and says so:
// the alternative is a field that quietly keeps the length it had before.
func (a *App) readGenLength() {
	n, err := strconv.Atoi(strings.TrimSpace(a.gen.length))
	if err != nil {
		n = 0
	}
	a.gen.opts.Length = n
}

// problem is what these options cannot do, or "" when they describe a password
// that can be generated. It is asked every frame, so a field being typed into
// says what is wrong with it while it is being typed rather than once it is
// finished.
func (g genState) problem() string {
	if !g.opts.Lower && !g.opts.Upper && !g.opts.Digits && !g.opts.Symbols {
		return "至少要留一类字符。"
	}
	if g.opts.Length < 8 || g.opts.Length > 128 {
		return "长度得在 8 到 128 之间。"
	}
	return ""
}

// genDraw is the password on screen, and how strong it is. A new one is drawn
// when the options have moved or another was asked for; the one already there
// is kept otherwise, so a screen nobody touched shows the same password on
// every frame.
func (a *App) genDraw() {
	if a.gen.problem() != "" {
		// No alphabet, or a length that is not one. There is no password to
		// show and no strength to claim, rather than a weak password dressed up
		// as a strong one.
		a.gen.password, a.gen.bits, a.gen.strength = "", 0, ""
		return
	}
	if a.gen.password != "" && a.gen.opts == a.gen.shown && !a.gen.again {
		return
	}
	pw, err := vaultcrypto.GeneratePassword(a.gen.opts, a.rand())
	if err != nil {
		// The options were checked above, so this is the randomness failing.
		// There is no password, and there is no strength to claim for one.
		a.gen.password, a.gen.bits, a.gen.strength = "", 0, ""
		return
	}
	a.gen.password, a.gen.shown, a.gen.again = pw, a.gen.opts, false
	a.gen.bits = a.gen.opts.EntropyBits()
	a.gen.strength = strengthLabel(a.gen.bits)
}

// strengthLabel is what an amount of entropy is called.
func strengthLabel(bits float64) string {
	switch vaultcrypto.StrengthLevel(bits) {
	case 1:
		return "一般"
	case 2:
		return "强"
	case 3:
		return "很强"
	}
	return "弱"
}

// copyGen puts the password on the clipboard and says so. What happens to it
// after that is the system's, which is why the copy is also cleared on a timer
// — Task 17's, not this screen's.
func (a *App) copyGen(c *ui.Context) {
	if a.gen.password == "" {
		return
	}
	a.copySecret(c, a.gen.password)
	a.gen.copied = "已复制"
}

// useGenerated writes the password on screen into the item the generator was
// opened from, and goes back to that item.
//
// The write goes through the engine like every other one, so the password it
// replaces goes into the item's history rather than being dropped: a password
// somebody just threw away is still one they may need somewhere.
func (a *App) useGenerated() {
	if a.gen.itemID == "" || a.gen.password == "" {
		return
	}
	e, ok := a.v.Entry(a.gen.itemID)
	if !ok {
		// Gone: purged, or the vault was closed under the screen. There is
		// nothing to write to, and the password is left where it is.
		a.closeGen()
		return
	}
	it := e.Item
	it.Password = a.gen.password
	if _, err := a.v.Update(e.ID, it, nowUnix()); err != nil {
		a.err = failMessage(err)
		return
	}
	a.closeGen()
	a.err = ""
}
