package ccapp

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"bipPass/internal/vault"
)

// genMain opens the app on the main view with one login item in it, which is
// where the generator is opened from.
//
// The item's title is one the seeded vault does not use. A row's label is its
// title, and a click has nothing else to go on: two rows called 招商银行 would
// make the open below act on whichever of them comes first, and the assertions
// would be about a different item than the buttons were pressed on.
func genMain(t *testing.T) (*App, *ui.Tester, string) {
	t.Helper()
	a, tt := newMain(t)
	e, err := a.v.Create(a.main.vaultID, vault.Item{
		Type:     vault.TypeLogin,
		Title:    "京东",
		Username: "carl",
		Password: "{{SECRET_nvgib2yo}}",
	}, fixedUnix)
	if err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	return a, tt, e.ID
}

// openGen opens the generator screen. Every test below starts here.
func openGen(t *testing.T, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click("生成器"); err != nil {
		t.Fatalf("clicking 生成器: %v", err)
	}
	tt.Frame()
	if !tt.HasText("生成密码") {
		t.Fatalf("the generator did not open: %v", tt.Texts())
	}
}

// clickClass turns one character class off, or back on. All four start on, so
// the first click on a label turns that class off.
func clickClass(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatalf("clicking %s: %v", label, err)
	}
	tt.Frame()
}

// regenerate asks for another password with the options as they are.
func regenerate(t *testing.T, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click("重新生成"); err != nil {
		t.Fatalf("clicking 重新生成: %v", err)
	}
	tt.Frame()
}

func TestTheGeneratorMakesAPasswordOfTheLengthAskedFor(t *testing.T) {
	a, tt, _ := genMain(t)
	openGen(t, tt)
	// It opens with a password already in it: a screen that shows nothing
	// until something is pressed is a screen that looks broken.
	before := a.gen.password
	if len([]rune(before)) != 20 {
		t.Fatalf("the generator opened with %q", before)
	}
	field(t, tt, "长度", "32")
	tt.Frame()
	got := a.gen.password
	if len([]rune(got)) != 32 {
		t.Errorf("a length of 32 gave %d characters: %q", len([]rune(got)), got)
	}
	if got == before {
		t.Error("a new length gave the same password back")
	}
	// 重新生成 makes another one: a password screen that only ever shows one
	// password is a password nobody can choose among.
	regenerate(t, tt)
	again := a.gen.password
	if len([]rune(again)) != 32 {
		t.Errorf("a regenerate gave %d characters: %q", len([]rune(again)), again)
	}
	if again == got {
		t.Error("regenerating gave the same password back")
	}
}

func TestAGeneratedPasswordOnlyUsesTheClassesThatAreOn(t *testing.T) {
	a, tt, _ := genMain(t)
	openGen(t, tt)
	// Digits and symbols off: what comes out has neither, however many times
	// it is drawn. A class that is off and still shows up is a class switch
	// that does nothing.
	clickClass(t, tt, "数字")
	clickClass(t, tt, "符号")
	for i := 0; i < 5; i++ {
		regenerate(t, tt)
		got := a.gen.password
		if got == "" {
			t.Fatal("the generator produced nothing")
		}
		if strings.ContainsAny(got, "0123456789") {
			t.Errorf("a password with digits off has a digit: %q", got)
		}
		if strings.ContainsAny(got, "!@#$%^&*") {
			t.Errorf("a password with symbols off has a symbol: %q", got)
		}
	}
}

func TestTheGeneratorRefusesEveryClassOff(t *testing.T) {
	a, tt, id := genMain(t)
	openGen(t, tt)
	clickClass(t, tt, "小写")
	clickClass(t, tt, "大写")
	clickClass(t, tt, "数字")
	clickClass(t, tt, "符号")
	// No alphabet is no password: the screen says so rather than showing an
	// empty one, and nothing reaches the item.
	if !tt.HasText("至少要留一类") {
		t.Errorf("every class off gave: %v", tt.Texts())
	}
	if a.gen.password != "" {
		t.Errorf("every class off produced %q", a.gen.password)
	}
	got, _ := a.v.Entry(id)
	if got.Item.Password != "{{SECRET_nvgib2yo}}" {
		t.Error("the item's password changed with every class off")
	}
}

func TestTheStrengthLabelFollowsTheLength(t *testing.T) {
	a, tt, _ := genMain(t)
	openGen(t, tt)
	field(t, tt, "长度", "8")
	tt.Frame()
	weak := a.gen.strength
	weakBits := a.gen.bits
	field(t, tt, "长度", "40")
	tt.Frame()
	strong := a.gen.strength
	strongBits := a.gen.bits
	if weak == "" || strong == "" {
		t.Fatalf("the strength label is empty: %q and %q", weak, strong)
	}
	if weak == strong {
		t.Errorf("a length of 8 and one of 40 both read %q", weak)
	}
	if strongBits <= weakBits {
		t.Errorf("40 characters is not stronger than 8: %v and %v", strongBits, weakBits)
	}
	// The label names the entropy, so it moves with the alphabet as well as
	// the length: turning a class off costs bits.
	clickClass(t, tt, "小写")
	clickClass(t, tt, "大写")
	clickClass(t, tt, "数字")
	clickClass(t, tt, "符号")
	tt.Frame()
	if got := a.gen.strength; got != "" {
		t.Errorf("no alphabet has a strength label: %q", got)
	}
	if a.gen.bits != 0 {
		t.Errorf("no alphabet has entropy: %v", a.gen.bits)
	}
}

func TestALengthOutsideTheRangeIsRefused(t *testing.T) {
	a, tt, _ := genMain(t)
	openGen(t, tt)
	// Seven is one character short of the shortest password the generator
	// makes, and a field that quietly made seven anyway would be a field whose
	// limits are decoration.
	field(t, tt, "长度", "7")
	tt.Frame()
	if !tt.HasText("长度") {
		t.Errorf("a length of 7 gave: %v", tt.Texts())
	}
	if a.gen.password != "" {
		t.Errorf("a length of 7 produced %q", a.gen.password)
	}
	field(t, tt, "长度", "129")
	tt.Frame()
	if a.gen.password != "" {
		t.Errorf("a length of 129 produced %q", a.gen.password)
	}
}

func TestTheGeneratorCanReplaceTheOpenItemsPassword(t *testing.T) {
	a, tt, id := genMain(t)
	openItem(t, tt, "京东")
	openGen(t, tt)
	// The screen knows which item it was opened from, and it is offered only
	// then: a generator with nothing open has no item to replace.
	if !tt.HasText("用作当前条目密码") {
		t.Fatalf("an open item offers no replacement: %v", tt.Texts())
	}
	made := a.gen.password
	if err := tt.Click("用作当前条目密码"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	got, ok := a.v.Entry(id)
	if !ok {
		t.Fatal("the item is gone")
	}
	if got.Item.Password != made {
		t.Errorf("the item's password is %q, the generated one was %q", got.Item.Password, made)
	}
	// The one it replaced is in the history, where the detail shows it: a
	// password somebody just threw away is still one they may need.
	if len(got.Item.History) != 1 || got.Item.History[0].Password != "{{SECRET_nvgib2yo}}" {
		t.Errorf("the history is %v", got.Item.History)
	}
	// The password is in the vault and nowhere else: it was put there to be
	// used, and the screen that made it is gone.
	if tt.HasText("生成密码") {
		t.Errorf("the generator is still open after a replacement: %v", tt.Texts())
	}
	if a.gen.password != "" {
		t.Errorf("the generated password is still in memory: %q", a.gen.password)
	}
}

func TestTheGeneratorIsQuietAboutWhatItMadeWhenNothingIsOpen(t *testing.T) {
	a, tt, _ := genMain(t)
	openGen(t, tt)
	// With no item open there is nothing to replace, and the button that
	// would replace one is not offered at all.
	if tt.HasText("用作当前条目密码") {
		t.Errorf("no item is open and yet: %v", tt.Texts())
	}
	if err := tt.Click("复制"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("已复制") {
		t.Errorf("the copy said nothing: %v", tt.Texts())
	}
	if got := tt.Clipboard(); got != a.gen.password {
		t.Errorf("the clipboard holds %q, the password is %q", got, a.gen.password)
	}
}

func TestTheGeneratedPasswordUsesEveryClassThatIsOn(t *testing.T) {
	a, tt, _ := genMain(t)
	openGen(t, tt)
	// Every class on means every class present: a 20-character password with
	// no symbol in it is one the switch lied about.
	for i := 0; i < 20; i++ {
		regenerate(t, tt)
		got := a.gen.password
		if !strings.ContainsAny(got, "abcdefghijklmnopqrstuvwxyz") ||
			!strings.ContainsAny(got, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") ||
			!strings.ContainsAny(got, "0123456789") ||
			!strings.ContainsAny(got, "!@#$%^&*") {
			t.Fatalf("a password is missing a class: %q", got)
		}
	}
}

func TestExcludingTheAmbiguousCharactersLeavesThemOut(t *testing.T) {
	a, tt, _ := genMain(t)
	openGen(t, tt)
	before := a.gen.bits
	if err := tt.Click("排除易混字符"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// 0 O 1 l I are the ones that look alike in a password field's font, and
	// a password with them in it is one somebody types wrong.
	for i := 0; i < 20; i++ {
		regenerate(t, tt)
		if got := a.gen.password; strings.ContainsAny(got, "0O1lI") {
			t.Fatalf("an ambiguous character was generated: %q", got)
		}
	}
	// The entropy falls with the alphabet, and the strength line says so
	// rather than pretending the password is as strong as it was.
	if a.gen.bits >= before {
		t.Errorf("a smaller alphabet is not weaker: %v was %v", a.gen.bits, before)
	}
}

func TestLeavingTheGeneratorPutsItsPasswordAway(t *testing.T) {
	a, tt, _ := genMain(t)
	openGen(t, tt)
	if a.gen.password == "" {
		t.Fatal("the generator opened with no password")
	}
	if err := tt.Click("返回"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.gen.open {
		t.Error("the generator is still open")
	}
	// The list is back, and the password it made is gone with the screen: it
	// was never written anywhere, and there is nothing to keep it for.
	if a.gen.password != "" {
		t.Errorf("the password was left behind: %q", a.gen.password)
	}
	if !tt.HasText("京东") {
		t.Errorf("the list is not back: %v", tt.Texts())
	}
}
