package ccapp

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"bipPass/internal/vault"
)

// sidebarView is the vaults down the side: which one the list is showing, and
// the way to make another. A vault's name is what a user called it, and it is
// the only thing about it that is ever in the clear: everything inside it is
// ciphertext behind the vault key.
//
// It is the library's own Sidebar, because the vaults are a list a keyboard has
// to be able to walk: Up and Down move among them, Home and End go to the ends,
// and typing the first letters of a name finds it. A column of hand-rolled rows
// answers the pointer and nothing else, and a user who does not use one is left
// on the vault they started on. The library's also gives the screen reader a
// tree of items, which a column of rows is not.
func (a *App) sidebarView(c *ui.Context) {
	t := *c.Theme()
	// A card, not a bare column against the window's white: the vaults are a
	// place of their own beside the list, and the gutter of white between
	// the two is what says so.
	card := ui.Column(c).Width(200).Shrink(0).Gap(t.Space(1.5)).
		Padding(t.Space(2)).Radius(t.Radius+2).
		Background(t.Surface).Border(1, t.Border)
	card.Children(func() {
		// The choice is the app's own: the sidebar holds the vault the list is
		// showing, so a vault chosen with the keyboard is the one the list shows.
		sb := ui.Sidebar(c, &a.main.vaultID, func() {
			for _, vi := range a.v.Vaults() {
				ui.SidebarItem(c, vi.ID, nil, vi.Name)
			}
		}).Fill()
		// A vault chosen with the keyboard is a vault the list has to follow,
		// and a new vault holds none of the rows the old one had on screen.
		if sb.Changed() {
			a.main.selected = 0
			a.main.open, a.err = "", ""
		}
		// The way to make another sits under the sidebar rather than in it: it
		// is not a vault, and a row among the vaults that is not one is a row
		// the arrows stop on for nothing.
		if a.main.making {
			ui.Field(c, "名称", func() {
				ui.TextInput(c, &a.main.newVault).Fill()
			})
			ui.Row(c).Gap(8).Children(func() {
				if ui.PrimaryButton(c, "创建").Clicked() {
					a.makeVault()
				}
				if ui.Button(c, "取消").Clicked() {
					a.main.making, a.main.newVault = false, ""
				}
			})
			return
		}
		if ui.Button(c, "新建保险库").Clicked() {
			a.main.making = true
		}
	})
}

// makeVault makes the vault whose name was typed and shows it. An empty name
// is refused here rather than stored: a vault nobody named is a vault nobody
// can pick out of a list again.
func (a *App) makeVault() {
	name := trimSpace(a.main.newVault)
	if name == "" {
		a.err = "给保险库起个名字。"
		return
	}
	vi, err := a.v.CreateVault(name, nowUnix())
	if err != nil {
		a.err = failMessage(err)
		return
	}
	a.main.making, a.main.newVault = false, ""
	a.main.vaultID, a.main.selected, a.main.open = vi.ID, 0, ""
	a.err = ""
}

// trimSpace is the white space a typed name has around it. A name of spaces
// only is no name.
func trimSpace(s string) string {
	return strings.TrimSpace(s)
}

// listView is the list beside the sidebar: the search field, the filters, and
// the rows. The list takes the keys while it has the focus, so the arrows move
// the choice and Enter opens what is chosen.
func (a *App) listView(c *ui.Context, rows []vault.Entry) {
	t := *c.Theme()
	// The list is a card too, and the search field and the filters sit in its
	// padding: the white around them is the gutter, not the page.
	card := ui.Column(c).Grow(1).MinWidth(0).Gap(t.Space(3)).
		Padding(t.Space(3)).Radius(t.Radius+2).
		Background(t.Surface).Border(1, t.Border)
	card.Children(func() {
		// A Field around the input rather than a SearchField of its own: the
		// field names its input with its own label, which is what lets a test
		// ask whether the search field has the focus by its name, and what
		// lets a click on the name put the cursor in the input.
		ui.Field(c, "搜索", func() {
			in := ui.TextInput(c, &a.main.query).Fill()
			// Typing is the user being there, and it is the auto-lock's only
			// witness: a vault that locked while somebody was narrowing a list
			// would make them unlock to press nothing. Changed is the input's
			// own word for a character having landed, so a frame that drew a
			// character is a frame that moved the deadline.
			if in.Changed() {
				a.touch(c.Now())
			}
			if a.main.wantSearch {
				in.Focus()
			}
			in.Placeholder("标题、用户名、备注")
		})
		a.main.wantSearch = false
		// One control rather than a row of buttons, and the trash is one of
		// its chips: these are all ways of saying what the list is showing,
		// and drawing them as one control says so. It is also what makes them
		// answer the pointer — a button given a face of its own loses the
		// hover and the press the theme would otherwise give it, which is
		// how these used to sit there looking dead.
		ui.ToggleGroup(c, func() {
			for _, f := range filters {
				on := f.typ == a.main.typ
				if ui.Toggle(c, &on, f.label).Changed() {
					a.main.typ = f.typ
					a.main.selected = 0
					// 全部 names the whole vault rather than a kind of item, so
					// it is also the way out of the trash: the trash is a place
					// the list went, and this is the chip that brings it back.
					// The others narrow what is in the place the list is in.
					if f.typ == "" {
						a.main.trash = false
					}
				}
			}
			// The trash is a place rather than a kind, so it is a toggle and
			// not a filter: it shows whether the list is looking at it.
			if ui.Toggle(c, &a.main.trash, "已删除").Changed() {
				a.main.selected = 0
			}
		}).Label("筛选")
		a.list(c, rows)
	})
}

// list is the rows themselves. It is a List rather than a stack of buttons: a
// List keeps the choice with the item it belongs to as rows come and go, and
// takes the arrow keys and Enter itself, which is what makes a keyboard work.
func (a *App) list(c *ui.Context, rows []vault.Entry) {
	if len(rows) == 0 {
		ui.Text(c, emptyMessage(a.main)).TextColor(c.Theme().TextMuted)
		return
	}
	sel := a.main.selected
	s := ui.ListState{
		Selected: &sel,
		Key: func(row int) any {
			if row < 0 || row >= len(rows) {
				return nil
			}
			return rows[row].ID
		},
		Label: func(row int) string {
			if row < 0 || row >= len(rows) {
				return ""
			}
			return title(rows[row])
		},
	}
	a.main.selected = sel
	list := ui.List(c, &s, len(rows), func(row int) {
		e := rows[row]
		ui.Row(c).Fill().Gap(8).Padding(6, 8).Children(func() {
			ui.Text(c, title(e)).SingleLine().Grow(1)
			if e.Item.Username != "" {
				ui.Text(c, e.Item.Username).SingleLine().TextColor(c.Theme().TextMuted)
			}
			ui.Text(c, typeLabel(e.Item.DisplayType())).SingleLine().TextColor(c.Theme().TextMuted)
		})
	}).Grow(1)
	if a.main.wantList {
		list.Focus()
		a.main.wantList = false
		a.main.listed = true
	}
	if list.Submitted() {
		a.openSelected(rows)
	}
	a.main.selected = sel
}

// openSelected opens the row the list's choice is on. A choice on nothing —
// an empty list, or one whose rows went while the detail was open — opens
// nothing rather than the wrong item.
func (a *App) openSelected(rows []vault.Entry) {
	if a.main.selected < 0 || a.main.selected >= len(rows) {
		return
	}
	a.main.open = rows[a.main.selected].ID
}

// emptyMessage is what an empty list says. It names the reason, because "no
// results" over a vault that does have items in it is a question, not an
// answer.
func emptyMessage(m mainState) string {
	switch {
	case m.trash:
		return "回收站是空的。"
	case m.query != "":
		return fmt.Sprintf("没有和「%s」相符的。", m.query)
	case m.typ != "":
		return "这个分类下还没有。"
	}
	return "还没有条目。"
}
