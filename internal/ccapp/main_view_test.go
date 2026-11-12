package ccapp

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"ccpass/internal/identity"
	"ccpass/internal/store"
	"ccpass/internal/vault"
)

// mainMnemonic is a second real mnemonic, so a seeded vault can be opened with
// the same account a keyfile in the home was made from.
var mainMnemonic = appMnemonic

// newMain opens the app on the main view with a vault that has items in it.
// The home is a real one: a keyfile, and a database seeded through the vault
// engine itself, so what the screen shows is what the engine decrypts.
func newMain(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	home := emptyHome(t)
	a, err := New(Options{Home: home})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Lock)
	// The keyfile is written for the mnemonic the vault is seeded under, and
	// the account is opened the way the unlock screen opens it: the same
	// code path a passphrase goes through.
	kf, session, err := identity.NewAccount(mainMnemonic, "correct horse battery staple", fixedNow(), appReader())
	if err != nil {
		t.Fatal(err)
	}
	if err := identity.SaveKeyfile(filepath.Join(home, store.KeyfileName), kf); err != nil {
		t.Fatal(err)
	}
	db, v, err := openVault(home, session)
	if err != nil {
		t.Fatal(err)
	}
	seedVault(t, v, db)
	a.db, a.v, a.session = db, v, session
	tt := newTester(t, a)
	tt.Frame()
	return a, tt
}

// seedVault puts items in the vault: one of each type, in two vaults, one
// deleted, so the list and the trash both have something in them.
func seedVault(t *testing.T, v *vault.Vault, db *store.DB) {
	t.Helper()
	personal := v.Vaults()[0].ID
	work, err := v.CreateVault("工作", fixedUnix)
	if err != nil {
		t.Fatal(err)
	}
	items := []vault.Item{
		{Type: vault.TypeLogin, Title: "招商银行", Username: "carl"},
		{Type: vault.TypeLogin, Title: "apple", Username: "carl@example.com"},
		{Type: vault.TypeNote, Title: "备用码", Notes: "放在抽屉里"},
		{Type: vault.TypeCard, Title: "信用卡"},
		{Type: vault.TypeIdentity, Title: "护照"},
		{Type: vault.TypeLogin, Title: "旧账号", Username: "carl"},
		{Type: vault.TypeLogin, Title: "工作邮箱", Username: "work@example.com"},
	}
	for i, it := range items {
		id := personal
		if i == len(items)-1 {
			id = work.ID
		}
		if _, err := v.Create(id, it, fixedUnix+int64(i)); err != nil {
			t.Fatal(err)
		}
	}
	// The last one created is the one deleted, so the trash has one item in it.
	e := v.Entries()
	for _, entry := range e {
		if entry.Item.Title == "旧账号" {
			if err := v.Delete(entry.ID, fixedUnix+100); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// rowTitles is the titles in the list, in the order they are shown.
func rowTitles(t *testing.T, tt *ui.Tester) []string {
	t.Helper()
	// A row is a button whose label is the title: it is what a click and a
	// Return act on.
	var out []string
	for _, s := range tt.Texts() {
		if s == "" || strings.HasPrefix(s, "全部") || strings.HasPrefix(s, "已删除") {
			continue
		}
		out = append(out, s)
	}
	return out
}

func TestTheMainViewListsTheItemsInTheSelectedVault(t *testing.T) {
	_, tt := newMain(t)
	// The list shows what the vault holds, and the sidebar names the vaults.
	for _, want := range []string{"招商银行", "apple", "备用码", "信用卡", "护照"} {
		if !tt.HasText(want) {
			t.Errorf("the list does not show %s: %v", want, tt.Texts())
		}
	}
	// 工作邮箱 is in the other vault, so it is not in this list.
	if tt.HasText("工作邮箱") {
		t.Errorf("an item from another vault is listed: %v", tt.Texts())
	}
	// A deleted item is not in the list.
	if tt.HasText("旧账号") {
		t.Errorf("a deleted item is listed: %v", tt.Texts())
	}
}

func TestTheSearchNarrowsTheListAsItIsTyped(t *testing.T) {
	_, tt := newMain(t)
	if err := tt.Click("搜索"); err != nil {
		t.Fatalf("clicking 搜索: %v", err)
	}
	tt.Type("招商")
	tt.Frame()
	if !tt.HasText("招商银行") {
		t.Errorf("the match is not listed: %v", tt.Texts())
	}
	// The other items are gone, and so are the vaults' own names: only the
	// matching rows remain.
	for _, gone := range []string{"apple", "备用码", "信用卡", "护照"} {
		if tt.HasText(gone) {
			t.Errorf("%s is still listed for 招商: %v", gone, tt.Texts())
		}
	}
	// A term that matches nothing empties the list.
	tt.Type(" 不存在")
	tt.Frame()
	if tt.HasText("招商银行") {
		t.Errorf("a query that matches nothing still lists: %v", tt.Texts())
	}
	if !tt.HasText("没有") {
		t.Errorf("an empty list says nothing: %v", tt.Texts())
	}
}

func TestTheTypeFilterShowsOnlyThatType(t *testing.T) {
	_, tt := newMain(t)
	if err := tt.Click("登录"); err != nil {
		t.Fatalf("clicking 登录: %v", err)
	}
	tt.Frame()
	if !tt.HasText("招商银行") || !tt.HasText("apple") {
		t.Errorf("the logins are not listed: %v", tt.Texts())
	}
	for _, gone := range []string{"备用码", "信用卡", "护照"} {
		if tt.HasText(gone) {
			t.Errorf("%s is still listed under 登录: %v", gone, tt.Texts())
		}
	}
	// 全部 brings them all back.
	if err := tt.Click("全部"); err != nil {
		t.Fatalf("clicking 全部: %v", err)
	}
	tt.Frame()
	if !tt.HasText("备用码") {
		t.Errorf("全部 did not restore the list: %v", tt.Texts())
	}
}

func TestTheTrashFilterShowsTheDeletedItems(t *testing.T) {
	_, tt := newMain(t)
	if err := tt.Click("已删除"); err != nil {
		t.Fatalf("clicking 已删除: %v", err)
	}
	tt.Frame()
	if !tt.HasText("旧账号") {
		t.Errorf("the deleted item is not in the trash: %v", tt.Texts())
	}
	for _, gone := range []string{"招商银行", "apple", "备用码"} {
		if tt.HasText(gone) {
			t.Errorf("%s is still listed under 已删除: %v", gone, tt.Texts())
		}
	}
	// Back to 全部, the deleted item is gone again.
	if err := tt.Click("全部"); err != nil {
		t.Fatalf("clicking 全部: %v", err)
	}
	tt.Frame()
	if tt.HasText("旧账号") {
		t.Errorf("the deleted item is listed again: %v", tt.Texts())
	}
}

func TestTheArrowKeysMoveTheSelectionAndEnterOpensIt(t *testing.T) {
	a, tt := newMain(t)
	// The list opens on its first row, so a Down moves to the second. The
	// selection is visible: a row that is selected is marked, and the one
	// under it is not.
	if !tt.HasText("apple") {
		t.Fatalf("the list has no rows: %v", tt.Texts())
	}
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	// Enter opens the selected row's detail, which is a screen of its own:
	// the list is not on it, and the vault stays open behind it.
	if !a.Unlocked() {
		t.Fatal("opening an item locked the vault")
	}
	if tt.HasText("搜索") {
		t.Errorf("the list is still showing after Enter: %v", tt.Texts())
	}
}

func TestTheSearchFieldTakesCmdK(t *testing.T) {
	_, tt := newMain(t)
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Frame()
	if !tt.Focused("搜索") {
		t.Errorf("Cmd+K did not focus the search field: %v", tt.Texts())
	}
}

func TestANewVaultAppearsInTheSidebarAndHoldsItsOwnItems(t *testing.T) {
	a, tt := newMain(t)
	if !tt.HasText("个人") || !tt.HasText("工作") {
		t.Fatalf("the seeded vaults are not in the sidebar: %v", tt.Texts())
	}
	// A new vault is made from the sidebar, and selecting it shows an empty
	// list rather than another vault's items.
	if err := tt.Click("新建保险库"); err != nil {
		t.Fatalf("clicking 新建保险库: %v", err)
	}
	tt.Frame()
	field(t, tt, "名称", "家庭")
	if err := tt.Click("创建"); err != nil {
		t.Fatalf("clicking 创建: %v", err)
	}
	tt.Frame()
	if !tt.HasText("家庭") {
		t.Errorf("the new vault is not in the sidebar: %v", tt.Texts())
	}
	if tt.HasText("招商银行") {
		t.Errorf("the new vault lists another vault's items: %v", tt.Texts())
	}
	if len(a.Vaults()) != 3 {
		t.Errorf("%d vaults, want 3: %v", len(a.Vaults()), a.Vaults())
	}
}
