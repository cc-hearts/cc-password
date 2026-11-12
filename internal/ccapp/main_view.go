package ccapp

import (
	"github.com/egoist/mygo/ui"

	"ccpass/internal/vault"
)

// mainState is what the main screen holds between frames: which vault is
// open, what the list is narrowed to, and which item the detail shows.
//
// The list's rows are recomputed every frame from the vault: the vault is the
// only copy of the data, and holding a slice of it here would be a second one
// that could go stale the moment an item changed.
type mainState struct {
	// vaultID is the vault the list shows. Empty means the first vault, which
	// is the account's own, and is only ever empty before the vault is open.
	vaultID string
	// query is what the search field holds, narrowing the list as it is typed.
	query string
	// typ narrows the list to one type. Empty means every type.
	typ string
	// trash shows the deleted items instead of the live ones.
	trash bool
	// selected is the row the list's choice is on. It is the list's own, and
	// the list keeps it with the item it belongs to when the rows move.
	selected int
	// open is the item the detail shows, empty when the list is what is on
	// screen.
	open string
	// newVault is the name being typed for a new vault, and making is whether
	// its form is showing.
	newVault string
	making   bool
	// wantList gives the list the focus once, when the screen appears. It is
	// not Focus() every frame: that would take the focus back from the search
	// field the moment someone typed in it.
	wantList bool
	// listed is whether the list has had the focus once already. It is what
	// stops wantList being asked for again on every frame, and it is cleared
	// when the detail closes, so the list has the keys again on the way back.
	listed bool
	// wantSearch asks for the search field to take the focus, which is what
	// Cmd+K does. The field is inside a Field, so asking the field for the
	// focus gives it to the input inside, where typing goes.
	wantSearch bool
}

// mainView is the vault itself: the vaults down the side, the list beside
// them, and the detail on top of both when an item is open.
func (a *App) mainView(c *ui.Context) {
	// Cmd+K is the window's own: it works whichever of the screen's controls
	// has the focus, including the list. It is declared here, at the top of
	// the view and before any element is built, which is what makes it the
	// window's rather than an element's.
	c.OnShortcut(ui.Cmd, ui.KeyK, func() { a.main.wantSearch = true })
	a.ensureVault()
	rows := a.rows()
	ui.Column(c).Fill().Gap(12).Children(func() {
		ui.Row(c).Children(func() {
			ui.Text(c, "ccpass").FontSize(20).Bold().Grow(1)
			if ui.Button(c, "新建条目").Clicked() {
				a.edit = newEditState(a.main.vaultID, vault.TypeLogin)
			}
			if ui.Button(c, "生成器").Clicked() {
				a.openGen(a.main.open)
			}
			if ui.Button(c, "锁定").Clicked() {
				a.Lock()
			}
		})
		if a.err != "" {
			ui.Text(c, a.err).TextColor(c.Theme().TextMuted)
			return
		}
		// The form is a screen of its own over everything the main screen
		// shows: while it is open there is no list to walk and no detail to
		// read, so nothing behind it is built and nothing behind it takes a
		// key.
		if a.edit.open {
			a.editForm(c)
			return
		}
		// The generator is a screen over the detail as well as over the list:
		// it is opened from either, and while it is showing there is no list to
		// walk and no item to read, so nothing behind it takes a key.
		if a.gen.open {
			a.genView(c)
			return
		}
		if a.main.open != "" {
			a.detailView(c)
			return
		}
		// The list has the keys the moment the vault opens: its rows are ready
		// to be walked with the arrows, with no click first. Once, and again
		// after the detail closes — asking for it every frame would take the
		// focus back off the search field the moment someone typed in it.
		if !a.main.listed {
			a.main.wantList = true
		}
		ui.Row(c).Fill().Gap(12).Children(func() {
			a.sidebarView(c)
			a.listView(c, rows)
		})
	})
}

// ensureVault picks the vault the list shows when it has none: the first one,
// which is the account's own. It runs every frame, so a vault that has gone
// — deleted elsewhere, or a home whose first vault is another — falls back to
// one that is there.
func (a *App) ensureVault() {
	if a.v == nil {
		return
	}
	for _, vi := range a.v.Vaults() {
		if vi.ID == a.main.vaultID {
			return
		}
	}
	if all := a.v.Vaults(); len(all) > 0 {
		a.main.vaultID = all[0].ID
	}
}

// rows is what the list shows: the vault's own items, narrowed by the search
// field and the filters, in the order the engine sorts them.
func (a *App) rows() []vault.Entry {
	if a.v == nil {
		return nil
	}
	f := vault.Filter{VaultID: a.main.vaultID, Type: a.main.typ, Trash: a.main.trash}
	return vault.Search(a.v.Entries(), a.main.query, f)
}

// typeLabel is what an item's type is called on screen.
func typeLabel(t string) string {
	switch t {
	case vault.TypeLogin:
		return "登录"
	case vault.TypeNote:
		return "笔记"
	case vault.TypeCard:
		return "信用卡"
	case vault.TypeIdentity:
		return "身份"
	}
	return "笔记"
}

// filters are the type filters, in the order they are shown: everything first,
// then the type most items are. The trash is not one of them: it is a place
// the list goes, not a kind of item.
var filters = []struct {
	label string
	typ   string
}{
	{label: "全部", typ: ""},
	{label: "登录", typ: vault.TypeLogin},
}
