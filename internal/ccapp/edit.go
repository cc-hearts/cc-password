package ccapp

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"ccpass/internal/vault"
	"ccpass/internal/vaultcrypto"
)

// editState is what the edit screen holds between frames: the form being
// filled in, and what it is being filled in for.
//
// The form is a copy of the item rather than the item itself: the vault is
// written only when the user says so, so a half-filled form — or one they
// cancel — never reaches the payload. The password is held here in the clear
// while the form is open, which is the one place the plan allows it outside
// the sealed blob, and it is emptied when the form closes.
type editState struct {
	// open is whether the form is showing at all. It is a flag of its own
	// rather than a test on the other fields, because a new item's form has no
	// id yet and an empty form is a form nobody has typed in.
	open bool
	// id is empty for a new item, and the item's own when one is being edited.
	id string
	// vaultID is the vault the item goes in: the one the list was showing when
	// the form was opened. An item's vault is fixed once it is written — the
	// engine pins it, so a form that offered another one would be a picker
	// whose choice was thrown away on the save.
	vaultID string
	typ     string
	title   string
	// urls is one line per url: a url list is a list of lines to a user, and
	// one line each is what a text area gives them.
	urls     string
	username string
	// password is what the password field holds, which is empty until it is
	// typed in. An edited item's own password is left in the vault rather than
	// put in the field: the field is masked, so a password already in it is one
	// nobody can read back to check what they typed, and a save that leaves it
	// alone keeps the one the item has rather than clearing it.
	password string
	otp      string
	// tags is the tag list as one comma-separated line.
	tags  string
	notes string
	// fields are the item's own, in the order the form shows them. A new field
	// starts empty and is dropped when it has neither a name nor a value.
	fields []vault.Field
	// adding is whether the new-field row is showing.
	adding bool
	// newLabel and newValue are the field being added, and newSecret whether
	// it is a secret one.
	newLabel  string
	newValue  string
	newSecret bool
	// confirming is whether the purge is being confirmed. It is a step rather
	// than a click: 彻底删除 destroys, and a mis-click is data gone for good.
	confirming bool
}

// newEditState is the form for a new item of a type, with that type's template
// fields in it. A login and a note have no template, so their forms start
// empty.
func newEditState(vaultID, typ string) editState {
	return editState{open: true, vaultID: vaultID, typ: typ, fields: vault.TemplateFields(typ)}
}

// editForm is the form itself: what an item holds, and the way to save it or
// leave it.
//
// The heading, the type picker and the buttons stay outside the scroll, and the
// fields scroll under them: a form is longer than a small window, and a 保存 that
// has scrolled out of sight is a form nobody can finish. Neither is a control
// that only appears once the scroll has been used, which is why the type picker
// is up here with them.
func (a *App) editForm(c *ui.Context) {
	ui.Column(c).Fill().Gap(12).Children(func() {
		ui.Row(c).Gap(8).Children(func() {
			if ui.Button(c, "取消").Clicked() {
				a.closeEdit()
			}
			ui.Text(c, a.editHeading()).FontSize(20).Bold().Grow(1)
		})
		// The type is a picker, not a field: an item's type decides which of
		// its fields mean anything, and a login saved as a note is an item
		// whose password nobody finds.
		ui.Row(c).Gap(8).Children(func() {
			for _, t := range []string{vault.TypeLogin, vault.TypeNote, vault.TypeCard, vault.TypeIdentity} {
				b := ui.Button(c, typeLabel(t))
				if t == a.edit.typ {
					b.Background(c.Theme().Surface)
				}
				if b.Clicked() {
					a.edit.typ = t
					// A template belongs to the type that came with it: moving
					// a half-filled card form to a login leaves its card fields
					// behind rather than saving a login with a CVV in it.
					a.edit.fields = vault.TemplateFields(t)
				}
			}
		})
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).Gap(8).Children(func() {
				a.editField(c, "名称", &a.edit.title, false)
				a.editField(c, "用户名", &a.edit.username, false)
				a.editSecretField(c, "密码", &a.edit.password)
				a.editField(c, "网址", &a.edit.urls, false)
				a.editField(c, "两步验证", &a.edit.otp, false)
				a.editField(c, "标签", &a.edit.tags, false)
				ui.Field(c, "备注", func() {
					ui.TextInput(c, &a.edit.notes).Fill()
				})
				a.editFields(c)
				if ui.Button(c, "添加字段").Clicked() {
					a.edit.adding = true
				}
				if a.edit.adding {
					a.editNewField(c)
				}
			})
		})
		if a.err != "" {
			ui.Text(c, a.err).TextColor(c.Theme().TextMuted)
		}
		ui.Row(c).Gap(8).Children(func() {
			if ui.PrimaryButton(c, "保存").Clicked() {
				a.saveEdit()
			}
			ui.Button(c, "取消").OnClick(func() { a.closeEdit() })
		})
	})
}

// editHeading is what the form calls itself, which is the one thing a user
// with two forms open at different times needs to be sure of.
func (a *App) editHeading() string {
	if a.edit.id == "" {
		return "新建条目"
	}
	return "编辑条目"
}

// editField is one plain line of the form. A url is data that can carry
// anything, so it is typed as text and never as a link while it is being
// edited.
func (a *App) editField(c *ui.Context, label string, into *string, secret bool) {
	ui.Field(c, label, func() {
		in := ui.TextInput(c, into).Fill()
		if secret {
			in.Password()
		}
	})
}

// editSecretField is the password's own line. It is masked, because it is a
// password; and it is not a Field around a TextInput, because a password is
// the one thing on the form that must never be read off a screen over a
// shoulder, including while it is being changed.
func (a *App) editSecretField(c *ui.Context, label string, into *string) {
	ui.Field(c, label, func() {
		ui.TextInput(c, into).Password().Fill()
	})
}

// editFields are the item's own fields: a card's template, or the ones the user
// added. A secret one's value is masked and stays masked: the form is for
// changing it, not for reading it.
func (a *App) editFields(c *ui.Context) {
	for i, f := range a.edit.fields {
		ui.Row(c).Fill().Gap(8).Children(func() {
			ui.Field(c, f.Label, func() {
				value := &a.edit.fields[i].Value
				in := ui.TextInput(c, value).Fill()
				if f.Secret {
					in.Password()
				}
			})
			secret := a.edit.fields[i].Secret
			t := ui.Button(c, "秘密")
			if secret {
				t.Background(c.Theme().Surface)
			}
			if t.Clicked() {
				a.edit.fields[i].Secret = !secret
			}
			if ui.Button(c, "去掉").Clicked() {
				a.edit.fields = append(a.edit.fields[:i], a.edit.fields[i+1:]...)
			}
		})
	}
}

// editNewField is the row a field the user adds themselves is named in. Its
// secret toggle is here rather than only on a template's, because a field
// somebody added is exactly the one whose sensitivity this build cannot guess.
func (a *App) editNewField(c *ui.Context) {
	// The row comes into view when it appears. It is added below the fields,
	// which is where a form longer than the window has already scrolled past:
	// a row that opens under the fold is a button that looks broken.
	ui.Row(c).Fill().Gap(8).ScrollIntoView().Children(func() {
		a.editField(c, "字段名", &a.edit.newLabel, false)
		a.editField(c, "字段值", &a.edit.newValue, a.edit.newSecret)
		s := ui.Button(c, "秘密")
		if a.edit.newSecret {
			s.Background(c.Theme().Surface)
		}
		if s.Clicked() {
			a.edit.newSecret = !a.edit.newSecret
		}
		if ui.PrimaryButton(c, "加上").Clicked() {
			a.addField()
		}
	})
}

// addField puts the field being named into the form. A field with a name is
// kept even with no value: someone adding 备用码 has somewhere to put the code
// later. A field with no name is dropped, because a nameless field is one
// nobody can find again.
func (a *App) addField() {
	label := strings.TrimSpace(a.edit.newLabel)
	if label == "" {
		return
	}
	a.edit.fields = append(a.edit.fields, vault.Field{
		Label:  label,
		Value:  a.edit.newValue,
		Secret: a.edit.newSecret,
	})
	a.edit.newLabel, a.edit.newValue, a.edit.newSecret = "", "", false
	a.edit.adding = false
}

// saveEdit writes the form into the vault. Every field is checked before
// anything is written: a save that half-happened is an item nobody can
// identify, and a two-factor secret that does not parse is one that will
// produce no codes.
func (a *App) saveEdit() {
	title := strings.TrimSpace(a.edit.title)
	if title == "" {
		a.err = "起个名字，以后才好找。"
		return
	}
	var otp *vault.OTP
	if o := strings.TrimSpace(a.edit.otp); o != "" {
		parsed, err := vaultcrypto.ParseOTP(o)
		if err != nil {
			a.err = "两步验证的密钥不对，检查一下。"
			return
		}
		otp = &parsed
	}
	// The password is the item's own until the field has one in it: an empty
	// field is a field nobody typed in, not a request to clear the password.
	password := a.edit.password
	if password == "" && a.edit.id != "" {
		password = currentPassword(a.v, a.edit.id)
	}
	it := vault.Item{
		Type:     a.edit.typ,
		Title:    title,
		Username: strings.TrimSpace(a.edit.username),
		Password: password,
		OTP:      otp,
		Notes:    a.edit.notes,
		Tags:     parseTags(a.edit.tags),
		URLs:     parseLines(a.edit.urls),
		Fields:   a.edit.fields,
	}
	if a.edit.id == "" {
		if _, err := a.v.Create(a.edit.vaultID, it, nowUnix()); err != nil {
			a.err = failMessage(err)
			return
		}
		a.closeEdit()
		a.err = ""
		return
	}
	if _, err := a.v.Update(a.edit.id, it, nowUnix()); err != nil {
		a.err = failMessage(err)
		return
	}
	a.closeEdit()
	a.err = ""
}

// currentPassword is the password the vault holds for an item, or empty for
// one it does not hold. It is read at the save, when the field is being
// decided, rather than held in the form since it was opened: a vault another
// screen changed in between — a sync, a purge — is the vault the save must
// write to, not the one the form was filled in from.
func currentPassword(v *vault.Vault, id string) string {
	e, ok := v.Entry(id)
	if !ok {
		return ""
	}
	return e.Item.Password
}

// closeEdit leaves the form and empties it. The password goes with it: it has
// been written or discarded, and either way there is no reason to hold it a
// second longer.
func (a *App) closeEdit() {
	a.edit = editState{}
}

// openEdit opens the form on an item, filled in with what it holds.
func (a *App) openEdit(e vault.Entry) {
	if e.Item.ReadOnly() {
		// A payload from a build that does not exist yet. Its data is shown in
		// full, and the form is not offered at all: a build that does not know
		// the format must not rewrite it and lose what it cannot read.
		return
	}
	a.edit = editState{
		open:     true,
		id:       e.ID,
		vaultID:  e.VaultID,
		typ:      e.Item.DisplayType(),
		title:    e.Item.Title,
		username: e.Item.Username,
		notes:    e.Item.Notes,
		urls:     strings.Join(e.Item.URLs, "\n"),
		tags:     strings.Join(e.Item.Tags, ", "),
		fields:   append([]vault.Field(nil), e.Item.Fields...),
	}
	if e.Item.OTP != nil {
		a.edit.otp = e.Item.OTP.Secret
	}
}

// itemActions are what the detail does to the item itself: edit it, delete it,
// or — from the trash — bring it back or destroy it for good.
//
// The buttons offered depend on where the item is. An item in the trash cannot
// be edited, because editing a deleted row would silently un-delete it; and a
// purge is only offered from there, so the everyday detail has no button that
// destroys.
func (a *App) itemActions(c *ui.Context, e vault.Entry) {
	if e.Item.ReadOnly() {
		return
	}
	ui.Row(c).Gap(8).Children(func() {
		if e.Deleted {
			if ui.Button(c, "还原").Clicked() {
				a.restoreItem(e)
			}
			if ui.Button(c, "彻底删除").Clicked() {
				a.edit.confirming = true
			}
			return
		}
		if ui.Button(c, "编辑").Clicked() {
			a.openEdit(e)
		}
		if ui.Button(c, "删除").Clicked() {
			a.deleteItem(e)
		}
	})
	a.purgeConfirm(c, e)
}

// purgeConfirm asks before a purge, and takes either answer. It is a step of
// its own rather than a second click on the same button: a purge has no undo,
// and the cost of one mis-click is every byte of that item.
func (a *App) purgeConfirm(c *ui.Context, e vault.Entry) {
	if !a.edit.confirming {
		return
	}
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, "彻底删除后无法恢复，确定吗？").TextColor(c.Theme().TextMuted)
		ui.Row(c).Gap(8).Children(func() {
			if ui.PrimaryButton(c, "确认删除").Clicked() {
				a.purgeItem(e)
			}
			if ui.Button(c, "取消").Clicked() {
				a.edit.confirming = false
			}
		})
	})
}

// deleteItem moves an item to the trash. Its payload stays sealed and intact,
// which is what makes a restore possible: the trash is a place an item waits,
// not a way of losing it.
func (a *App) deleteItem(e vault.Entry) {
	if err := a.v.Delete(e.ID, nowUnix()); err != nil {
		a.err = failMessage(err)
		return
	}
	a.main.open = ""
	a.err = ""
}

// restoreItem brings a deleted item back, with everything it had.
func (a *App) restoreItem(e vault.Entry) {
	if err := a.v.Restore(e.ID, nowUnix()); err != nil {
		a.err = failMessage(err)
		return
	}
	a.err = ""
}

// purgeItem destroys an item for good. There is nothing after it: the row is a
// tombstone with no ciphertext, so there is no payload left to restore.
func (a *App) purgeItem(e vault.Entry) {
	if err := a.v.Purge(e.ID, nowUnix()); err != nil {
		a.err = failMessage(err)
		return
	}
	a.edit.confirming = false
	a.main.open = ""
	a.err = ""
}

// parseTags reads the tag list: one line of comma-separated tags. An empty tag
// is dropped rather than saved, because a tag of spaces is a filter that
// matches nothing and a label nobody chose.
func parseTags(s string) []string {
	var out []string
	for _, t := range strings.Split(s, ",") {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// parseLines reads one url per line. Blank lines are dropped: an empty url is
// a link that opens nothing.
func parseLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}
