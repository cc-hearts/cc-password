package ccapp

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"bipPass/internal/vault"
)

// editMain opens the app on the main view with one login item in it, and
// returns its id.
//
// The item's title is one the seeded vault does not use. A row's label is its
// title, and a click has nothing else to go on: two rows called 招商银行 would
// make every open, edit and delete below act on whichever of them comes first,
// and the assertions would be about a different item than the buttons were
// pressed on.
func editMain(t *testing.T) (*App, *ui.Tester, string) {
	t.Helper()
	a, tt := newMain(t)
	e, err := a.v.Create(a.main.vaultID, vault.Item{
		Type:     vault.TypeLogin,
		Title:    "京东",
		Username: "carl",
		Password: "s3cret-passphrase",
	}, fixedUnix)
	if err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	return a, tt, e.ID
}

// openItem opens the detail of the item with that title. A click on a row
// chooses it, and Enter opens the choice: the list takes the focus from the
// click, so the keys it holds go to it.
func openItem(t *testing.T, tt *ui.Tester, title string) {
	t.Helper()
	if err := tt.Click(title); err != nil {
		t.Fatalf("clicking %s: %v", title, err)
	}
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
}

func TestANewItemIsMadeAndListedAndPersisted(t *testing.T) {
	a, tt, _ := editMain(t)
	if err := tt.Click("新建条目"); err != nil {
		t.Fatalf("clicking 新建条目: %v", err)
	}
	tt.Frame()
	// A login's own form: the fields a login has, and nothing a note has.
	field(t, tt, "名称", "支付宝")
	field(t, tt, "用户名", "carl@example.com")
	field(t, tt, "密码", "alipay pass phrase")
	if err := tt.Click("保存"); err != nil {
		t.Fatalf("clicking 保存: %v", err)
	}
	tt.Frame()
	// It is in the list, in this vault, with what was typed in it.
	if !tt.HasText("支付宝") {
		t.Errorf("the new item is not listed: %v", tt.Texts())
	}
	got, ok := a.v.Entry(itemID(t, a, "支付宝"))
	if !ok {
		t.Fatal("the new item is not in the vault")
	}
	if got.Item.Username != "carl@example.com" {
		t.Errorf("the username is %q", got.Item.Username)
	}
	if got.Item.Password != "alipay pass phrase" {
		t.Error("the password was not kept")
	}
	// And it is still there when the vault is opened again: the save was a
	// write, not a screen.
	reopen(t, a)
	if !tt.HasText("支付宝") {
		t.Errorf("the new item did not survive a reopen: %v", tt.Texts())
	}
}

func TestAnItemCannotBeSavedWithoutATitle(t *testing.T) {
	a, tt, _ := editMain(t)
	if err := tt.Click("新建条目"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	field(t, tt, "用户名", "nobody")
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("起个名字") {
		t.Errorf("an untitled item gave: %v", tt.Texts())
	}
	// Nothing was written.
	for _, e := range a.v.Entries() {
		if e.Item.Username == "nobody" {
			t.Error("an untitled item was saved")
		}
	}
}

func TestEditingThePasswordKeepsTheOldOneInHistory(t *testing.T) {
	a, tt, id := editMain(t)
	openItem(t, tt, "京东")
	if err := tt.Click("编辑"); err != nil {
		t.Fatalf("clicking 编辑: %v", err)
	}
	tt.Frame()
	// The form opens on what the item holds: the username is what the item
	// has, and it is not the seeded row's.
	if a.edit.username != "carl" {
		t.Errorf("the form is not filled in: %q", a.edit.username)
	}
	field(t, tt, "密码", "a newer pass phrase")
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	got, ok := a.v.Entry(id)
	if !ok {
		t.Fatal("the item is gone")
	}
	if got.Item.Password != "a newer pass phrase" {
		t.Errorf("the password is %q", got.Item.Password)
	}
	// The old one is in the history, where the detail shows it, and the
	// engine put it there rather than the form.
	if len(got.Item.History) != 1 || got.Item.History[0].Password != "s3cret-passphrase" {
		t.Errorf("the history is %v", got.Item.History)
	}
}

func TestAnEmptyPasswordFieldKeepsTheOneTheItemHas(t *testing.T) {
	a, tt, id := editMain(t)
	openItem(t, tt, "京东")
	if err := tt.Click("编辑"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The password field opens empty. It is masked, so it is not one anybody
	// can read back and retype from, and a form that opens with it filled in
	// asks for exactly that.
	if a.edit.password != "" {
		t.Errorf("the form pre-filled the password: %q", a.edit.password)
	}
	field(t, tt, "网址", "https://jd.com")
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	got, _ := a.v.Entry(id)
	if got.Item.Password != "s3cret-passphrase" {
		t.Errorf("saving the form left the password %q", got.Item.Password)
	}
	if len(got.Item.History) != 0 {
		t.Errorf("an untouched password went into the history: %v", got.Item.History)
	}
}

func TestABadOTPIsRefusedAndNothingIsSaved(t *testing.T) {
	a, tt, id := editMain(t)
	openItem(t, tt, "京东")
	if err := tt.Click("编辑"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	field(t, tt, "两步验证", "not a secret at all")
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("两步验证") {
		t.Errorf("a bad OTP gave: %v", tt.Texts())
	}
	// The item is exactly as it was.
	got, _ := a.v.Entry(id)
	if got.Item.OTP != nil {
		t.Error("a bad OTP was saved")
	}
	if got.Item.Password != "s3cret-passphrase" {
		t.Error("the password changed on a refused save")
	}
}

func TestAGoodOTPIsSaved(t *testing.T) {
	a, tt, id := editMain(t)
	openItem(t, tt, "京东")
	if err := tt.Click("编辑"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	field(t, tt, "两步验证", "JBSWY3DPEHPK3PXP")
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	got, _ := a.v.Entry(id)
	if got.Item.OTP == nil || got.Item.OTP.Secret != "JBSWY3DPEHPK3PXP" {
		t.Errorf("the OTP is %v", got.Item.OTP)
	}
}

func TestACardsTemplateFieldsAreOffered(t *testing.T) {
	a, tt, _ := editMain(t)
	if err := tt.Click("新建条目"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The type picker offers the four kinds, and a card brings its own
	// template: the fields a card has, with the secret ones marked.
	if err := tt.Click("信用卡"); err != nil {
		t.Fatalf("clicking 信用卡: %v", err)
	}
	tt.Frame()
	for _, want := range []string{"卡号", "持卡人", "CVV"} {
		if !tt.HasText(want) {
			t.Errorf("a card's form does not ask %s: %v", want, tt.Texts())
		}
	}
	// The template is a starting point, not a fixed form: its fields are
	// editable, and it saves as a card.
	field(t, tt, "名称", "招行信用卡")
	field(t, tt, "卡号", "6225 8888 1234 5678")
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	e, ok := a.v.Entry(itemID(t, a, "招行信用卡"))
	if !ok {
		t.Fatal("the card was not saved")
	}
	if e.Item.DisplayType() != vault.TypeCard {
		t.Errorf("the item is a %s, want a card", e.Item.DisplayType())
	}
	var card vault.Field
	for _, f := range e.Item.Fields {
		if f.Label == "卡号" {
			card = f
		}
	}
	if card.Value != "6225 8888 1234 5678" {
		t.Errorf("the card number is %q", card.Value)
	}
}

func TestASecretCustomFieldIsMaskedInTheForm(t *testing.T) {
	a, tt, _ := editMain(t)
	if err := tt.Click("新建条目"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	field(t, tt, "名称", "带秘密字段的条目")
	// A field the user adds themselves can be marked secret, and it is masked
	// while it is being typed like a password is.
	if err := tt.Click("添加字段"); err != nil {
		t.Fatalf("clicking 添加字段: %v", err)
	}
	tt.Frame()
	if !tt.HasText("字段名") {
		t.Fatalf("the new-field row did not appear: %v", tt.Texts())
	}
	field(t, tt, "字段名", "取款密码")
	field(t, tt, "字段值", "112233")
	// It is marked secret here, where it is being typed, rather than after it
	// is saved: a field somebody adds themselves is exactly the one whose
	// sensitivity this build cannot guess. The row has its own 加上, so
	// marking it is not the last thing done to it.
	if err := tt.Click("秘密"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("加上"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The field is now one of the form's own, and its row has the form's
	// own toggle: a field added secret stays secret through a save.
	if !tt.HasText("取款密码") {
		t.Errorf("the added field is not on the form: %v", tt.Texts())
	}
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	e, ok := a.v.Entry(itemID(t, a, "带秘密字段的条目"))
	if !ok {
		t.Fatal("the item was not saved")
	}
	var found bool
	for _, f := range e.Item.Fields {
		if f.Label == "取款密码" {
			found = true
			if !f.Secret {
				t.Error("the custom field was not marked secret")
			}
		}
	}
	if !found {
		t.Errorf("the custom field is not there: %v", e.Item.Fields)
	}
}

func TestCancelLeavesTheItemAlone(t *testing.T) {
	a, tt, id := editMain(t)
	openItem(t, tt, "京东")
	if err := tt.Click("编辑"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	field(t, tt, "名称", "改个名字")
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	got, _ := a.v.Entry(id)
	if got.Item.Title != "京东" {
		t.Errorf("the title is %q after a cancel", got.Item.Title)
	}
	if tt.HasText("保存") {
		t.Errorf("the form is still showing after a cancel: %v", tt.Texts())
	}
}

func TestDeletePutsAnItemInTheTrashAndRestoreBringsItBack(t *testing.T) {
	a, tt, id := editMain(t)
	openItem(t, tt, "京东")
	if err := tt.Click("删除"); err != nil {
		t.Fatalf("clicking 删除: %v", err)
	}
	tt.Frame()
	// Gone from the list, and in the trash, where it can be restored.
	if tt.HasText("京东") {
		t.Errorf("the deleted item is still listed: %v", tt.Texts())
	}
	if err := tt.Click("已删除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("京东") {
		t.Errorf("the deleted item is not in the trash: %v", tt.Texts())
	}
	// Restore brings it back to the vault, with everything it had. The way
	// back is on the item's own detail: the trash row is a row like any other,
	// and it offers no buttons of its own.
	openItem(t, tt, "京东")
	if !tt.HasText("还原") {
		t.Errorf("the trash offers no way back: %v", tt.Texts())
	}
	if err := tt.Click("还原"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	got, ok := a.v.Entry(id)
	if !ok {
		t.Fatal("the item is gone after a restore")
	}
	if got.Deleted {
		t.Error("the item is still deleted after a restore")
	}
	if got.Item.Password != "s3cret-passphrase" {
		t.Error("a restored item lost its password")
	}
	// And the way out of the trash is the same 全部 chip the list always had:
	// the trash is a place the list went, not a screen of its own.
	if err := tt.Click("返回"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("全部"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("京东") {
		t.Errorf("the restored item is not listed: %v", tt.Texts())
	}
}

func TestAPurgeAsksFirstAndThenIsPermanent(t *testing.T) {
	a, tt, id := editMain(t)
	openItem(t, tt, "京东")
	if err := tt.Click("删除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("已删除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	openItem(t, tt, "京东")
	// 彻底删除 asks before it destroys: there is no undo for it, and a
	// mis-click here is data gone for good.
	if err := tt.Click("彻底删除"); err != nil {
		t.Fatalf("clicking 彻底删除: %v", err)
	}
	tt.Frame()
	if !tt.HasText("无法恢复") {
		t.Errorf("a purge did not ask first: %v", tt.Texts())
	}
	if err := tt.Click("确认删除"); err != nil {
		t.Fatalf("clicking 确认删除: %v", err)
	}
	tt.Frame()
	// What is left is a tombstone: the row stays, so the sync that carries it
	// still has something to carry, and it holds no payload at all — not the
	// password, not even the title. That is the purge, and there is no undo.
	gone, ok := a.v.Entry(id)
	if !ok {
		t.Fatal("the row is gone from the vault altogether")
	}
	if !gone.Purged || !gone.Deleted {
		t.Errorf("the tombstone is not marked purged: %+v", gone)
	}
	if gone.Item.Title != "" || gone.Item.Password != "" {
		t.Errorf("the tombstone still carries data: %+v", gone.Item)
	}
	if tt.HasText("京东") {
		t.Errorf("a purged item is still listed: %v", tt.Texts())
	}
	// And the answer can be no: the item is still there, still deleted, with
	// its payload intact. The trash still holds the seeded 旧账号, and that is the
	// item a second purge is asked about — 京东 is a tombstone with no detail left
	// to open.
	openItem(t, tt, "旧账号")
	if err := tt.Click("彻底删除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	seeded := itemID(t, a, "旧账号")
	kept, ok := a.v.Entry(seeded)
	if !ok {
		t.Fatal("a cancelled purge destroyed the item")
	}
	if kept.Purged {
		t.Error("a cancelled purge purged the item anyway")
	}
	if kept.Item.Username != "carl" {
		t.Errorf("a cancelled purge lost the payload: %+v", kept.Item)
	}
}

func TestAReadOnlyItemCannotBeEdited(t *testing.T) {
	a, tt := newMain(t)
	// An item from a newer build is shown, but its form is not offered: a
	// build that does not know the format must not rewrite it.
	e, err := a.v.Create(a.main.vaultID, vault.Item{
		Type:   vault.TypeLogin,
		Title:  "来自未来的条目",
		Schema: vault.SchemaVersion + 1,
	}, fixedUnix)
	if err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.main.open = e.ID
	tt.Frame()
	if tt.HasText("编辑") {
		t.Errorf("a read-only item offers an edit form: %v", tt.Texts())
	}
	if tt.HasText("删除") {
		t.Errorf("a read-only item offers a delete: %v", tt.Texts())
	}
}

func TestANewItemGoesInTheVaultTheListWasShowing(t *testing.T) {
	a, tt, _ := editMain(t)
	// The vault the list is showing is the vault a new item goes in: the form
	// is opened from that list, and a save that put the item somewhere else
	// would leave it in a list the user is not looking at.
	if err := tt.Click("工作"); err != nil {
		t.Fatalf("clicking 工作: %v", err)
	}
	tt.Frame()
	want := a.main.vaultID
	if err := tt.Click("新建条目"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	field(t, tt, "名称", "工作用的条目")
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	got, ok := a.v.Entry(itemID(t, a, "工作用的条目"))
	if !ok {
		t.Fatal("the item was not saved")
	}
	if got.VaultID != want {
		t.Errorf("the item is in vault %s, want %s", got.VaultID, want)
	}
	// The other vaults are left out of it: a new item is never quietly put
	// somewhere the list is not showing.
	for _, vi := range a.v.Vaults() {
		if vi.ID == want {
			continue
		}
		if got.VaultID == vi.ID {
			t.Errorf("the item went into %s, which the list was not showing", vi.Name)
		}
	}
}

// itemID is the id of the item with that title, for a test that has just saved
// one and needs to read it back out of the vault.
func itemID(t *testing.T, a *App, title string) string {
	t.Helper()
	for _, e := range a.v.Entries() {
		if e.Item.Title == title {
			return e.ID
		}
	}
	t.Fatalf("no item titled %q: %v", title, a.v.Entries())
	return ""
}

// reopen closes the vault and opens it again from the same files, the way
// locking and unlocking does. A save that only changed the screen is a save
// that is gone when the app is restarted.
func reopen(t *testing.T, a *App) {
	t.Helper()
	session := a.session
	a.v.Close()
	a.db.Close()
	db, v, err := openVault(a.home, session)
	if err != nil {
		t.Fatal(err)
	}
	a.db, a.v = db, v
}
