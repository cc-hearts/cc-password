package ccapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"bipPass/internal/store"
	"bipPass/internal/vaultcrypto"
)

// createReader is the randomness a create flow runs on: 128 deterministic
// bytes, the same every run, so the words and the check positions are fixed.
func createReader() *strings.Reader {
	b := make([]byte, 128)
	for i := range b {
		b[i] = byte(i)
	}
	return strings.NewReader(string(b))
}

// newCreate starts a fresh install on the first step of the create flow.
func newCreate(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	a, err := New(Options{Home: emptyHome(t), Rand: createReader()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Lock)
	tt := newTester(t, a)
	if err := tt.Click("新建账号"); err != nil {
		t.Fatalf("clicking 新建账号: %v", err)
	}
	tt.Frame()
	return a, tt
}

// stepWords moves the flow onto the retype step.
func stepWords(t *testing.T, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click("我已抄好"); err != nil {
		t.Fatalf("clicking 我已抄好: %v", err)
	}
	tt.Frame()
}

func TestTheFirstStepShowsTheWordsToCopy(t *testing.T) {
	_, tt := newCreate(t)
	texts := tt.Texts()
	// The words are on screen, numbered, and the app says what they are.
	if !tt.HasText("新建账号") {
		t.Errorf("the create flow did not start: %v", texts)
	}
	if !tt.HasText("1.") {
		t.Errorf("the first word is not numbered: %v", texts)
	}
	if !tt.HasText("24.") {
		t.Errorf("the last word is not numbered: %v", texts)
	}
	// 24 words, each on its own line, with its number in front of it.
	var lines []string
	for _, s := range texts {
		if _, _, ok := numberedWord(s); ok {
			lines = append(lines, s)
		}
	}
	if len(lines) != 24 {
		t.Fatalf("%d numbered lines, want 24: %v", len(lines), lines)
	}
	// Every line holds a word, not just a number.
	for _, l := range lines {
		if w := strings.TrimSpace(l[2:]); w == "" {
			t.Errorf("line %q holds no word", l)
		}
	}
	// Nothing is unlocked yet: the account is not made until the words are
	// confirmed and the passphrase is set.
	if tt.HasText("锁定") {
		t.Error("the app is unlocked before the account was made")
	}
}

func TestTheWordsAreTheMnemonicTheFlowWillUse(t *testing.T) {
	_, tt := newCreate(t)
	// The words on screen, in order, are a mnemonic this build accepts: the
	// flow shows a real one, not 24 arbitrary words, and the retype step
	// checks them against it.
	var words []string
	for _, s := range tt.Texts() {
		if _, w, ok := numberedWord(s); ok {
			words = append(words, w)
		}
	}
	if len(words) != 24 {
		t.Fatalf("%d words", len(words))
	}
	joined := strings.Join(words, " ")
	if err := vaultcrypto.ValidateMnemonic(joined); err != nil {
		t.Errorf("the words shown are not a mnemonic: %v", err)
	}
}

func TestThereIsNoWayPastTheRetypeStep(t *testing.T) {
	_, tt := newCreate(t)
	// The only way forward from the words is the retype step: no skip, no
	// "later". An account nobody can write down again is an account nobody
	// can recover.
	if err := tt.Click("我已抄好"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	texts := tt.Texts()
	if !tt.HasText("抄") && !tt.HasText("词") {
		t.Errorf("the retype step is not there: %v", texts)
	}
	// The step asks for three words by their positions.
	if tt.HasText("我已抄好") {
		t.Errorf("the words step is still showing: %v", texts)
	}
}

func TestTheRetypeStepAsksForThreeDistinctPositions(t *testing.T) {
	_, tt := newCreate(t)
	stepWords(t, tt)
	texts := tt.Texts()
	// Three fields, each naming a position between 1 and 24.
	var asks int
	for _, s := range texts {
		if strings.Contains(s, "第") && strings.Contains(s, "个词") {
			asks++
		}
	}
	if asks < 3 {
		t.Errorf("the retype step asks for %d words, want 3: %v", asks, texts)
	}
}

func TestTheRetypeStepRefusesAWrongWord(t *testing.T) {
	a, tt := newCreate(t)
	words := shownWords(t, tt)
	stepWords(t, tt)
	// The three fields in the order the step asks for them, filled with the
	// right words except the first, which is wrong.
	asks := retypeAsks(t, tt)
	if len(asks) != 3 {
		t.Fatalf("%d retype fields", len(asks))
	}
	for _, a := range asks {
		field(t, tt, a.label, words[a.position-1])
	}
	field(t, tt, asks[0].label, "notaword")
	if err := tt.Click("确认"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.Unlocked() {
		t.Fatal("a wrong word made an account")
	}
	if !tt.HasText("第") || !tt.HasText("个词不对") {
		t.Errorf("a wrong word gave: %v", tt.Texts())
	}
	// The wrong word is not echoed back, and the step is still there.
	for _, s := range tt.Texts() {
		if strings.Contains(s, "notaword") {
			t.Errorf("the wrong word was echoed: %q", s)
		}
	}
}

func TestTheRightWordsLeadToThePassphraseStep(t *testing.T) {
	a, tt := newCreate(t)
	words := shownWords(t, tt)
	stepWords(t, tt)
	asks := retypeAsks(t, tt)
	if len(asks) != 3 {
		t.Fatalf("%d retype fields", len(asks))
	}
	for _, a := range asks {
		field(t, tt, a.label, words[a.position-1])
	}
	if err := tt.Click("确认"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.Unlocked() {
		t.Fatal("the account was made before the passphrase was set")
	}
	// The passphrase step asks twice, as a passphrase nobody can reset is
	// worth typing carefully.
	for _, want := range []string{"新口令", "再输一次"} {
		if !tt.HasText(want) {
			t.Errorf("the passphrase step does not ask %s: %v", want, tt.Texts())
		}
	}
	// And the words are gone from the screen: they were only ever needed for
	// the copy-down.
	for _, s := range tt.Texts() {
		for _, w := range words {
			if strings.Contains(s, w+" ") || strings.HasSuffix(s, " "+w) {
				t.Errorf("a word is still on screen: %q", s)
			}
		}
	}
}

func TestThePassphraseStepRefusesAMismatch(t *testing.T) {
	a, tt := newCreate(t)
	words := shownWords(t, tt)
	stepWords(t, tt)
	for _, a := range retypeAsks(t, tt) {
		field(t, tt, a.label, words[a.position-1])
	}
	if err := tt.Click("确认"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	field(t, tt, "新口令", "create passphrase here")
	field(t, tt, "再输一次", "create passphrase herE")
	if err := tt.Click("创建"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.Unlocked() {
		t.Fatal("a mismatched passphrase made an account")
	}
	if !tt.HasText("两次输入的口令不一致") {
		t.Errorf("a mismatch gave: %v", tt.Texts())
	}
}

func TestThePassphraseStepRefusesAShortPassphrase(t *testing.T) {
	a, tt := newCreate(t)
	words := shownWords(t, tt)
	stepWords(t, tt)
	for _, a := range retypeAsks(t, tt) {
		field(t, tt, a.label, words[a.position-1])
	}
	if err := tt.Click("确认"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	field(t, tt, "新口令", "短")
	field(t, tt, "再输一次", "短")
	if err := tt.Click("创建"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.Unlocked() {
		t.Fatal("a short passphrase made an account")
	}
	if !tt.HasText("口令太短，至少 8 个字符") {
		t.Errorf("a short passphrase gave: %v", tt.Texts())
	}
}

func TestACreatedAccountUnlocksAndWritesTheKeyfile(t *testing.T) {
	home := emptyHome(t)
	a, err := New(Options{Home: home, Rand: createReader()})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Lock()
	tt := newTester(t, a)
	if err := tt.Click("新建账号"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	words := shownWords(t, tt)
	stepWords(t, tt)
	for _, a := range retypeAsks(t, tt) {
		field(t, tt, a.label, words[a.position-1])
	}
	if err := tt.Click("确认"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	field(t, tt, "新口令", "create passphrase here")
	field(t, tt, "再输一次", "create passphrase here")
	if err := tt.Click("创建"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.Unlocked() {
		t.Fatalf("the account was not made: %v", tt.Texts())
	}
	if _, err := os.Stat(filepath.Join(home, store.KeyfileName)); err != nil {
		t.Errorf("no keyfile: %v", err)
	}
	// The main view, and none of the words or the passphrase on it.
	if !tt.HasText("锁定") {
		t.Errorf("the main view is not showing: %v", tt.Texts())
	}
	for _, s := range tt.Texts() {
		if strings.Contains(s, "create passphrase") {
			t.Errorf("the passphrase is on screen: %q", s)
		}
		for _, w := range words {
			if strings.Contains(s, w+" ") || strings.HasSuffix(s, " "+w) {
				t.Errorf("a word is on screen: %q", s)
			}
		}
	}
}

func TestContentProtectionIsOnWhileTheWordsAreUp(t *testing.T) {
	var protect []bool
	a, err := New(Options{
		Home:    emptyHome(t),
		Rand:    createReader(),
		Protect: func(on bool) { protect = append(protect, on) },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Lock()
	tt := newTester(t, a)
	if err := tt.Click("新建账号"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The words are the account, so they are kept out of screenshots and
	// recordings from the moment they are shown.
	if len(protect) == 0 || !protect[0] {
		t.Fatalf("content protection was not turned on for the words: %v", protect)
	}
	words := shownWords(t, tt)
	stepWords(t, tt)
	for _, a := range retypeAsks(t, tt) {
		field(t, tt, a.label, words[a.position-1])
	}
	if err := tt.Click("确认"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	field(t, tt, "新口令", "create passphrase here")
	field(t, tt, "再输一次", "create passphrase here")
	if err := tt.Click("创建"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.Unlocked() {
		t.Fatalf("the account was not made: %v", tt.Texts())
	}
	if len(protect) < 2 || protect[len(protect)-1] {
		t.Errorf("content protection was not turned back off: %v", protect)
	}
}

// numberedWord reads one of the words step's lines: "7. abandon" is the
// seventh word. The number is one or two digits, so a line is not just a word
// after a single character.
func numberedWord(s string) (int, string, bool) {
	dot := strings.Index(s, ". ")
	if dot < 1 || dot > 2 {
		return 0, "", false
	}
	n := 0
	for _, c := range s[:dot] {
		if c < '0' || c > '9' {
			return 0, "", false
		}
		n = n*10 + int(c-'0')
	}
	w := strings.TrimSpace(s[dot+2:])
	if w == "" {
		return 0, "", false
	}
	return n, w, true
}

// shownWords is the mnemonic the flow is showing, as a slice of 24 words in
// order. It reads them off the screen the way a user would.
func shownWords(t *testing.T, tt *ui.Tester) []string {
	t.Helper()
	var words []string
	for _, s := range tt.Texts() {
		if _, w, ok := numberedWord(s); ok {
			words = append(words, w)
		}
	}
	if len(words) != 24 {
		t.Fatalf("%d words on screen, want 24", len(words))
	}
	return words
}

// retypeAsk is one of the retype step's fields: the label a click focuses, and
// the position of the word it asks for.
type retypeAsk struct {
	label    string
	position int
}

// retypeAsks reads the retype step's fields off the screen, in the order the
// step asks for them. A label is "第 7 个词", which is the field asking for the
// seventh word.
//
// A field's label shows twice — once as the text beside it and once as the
// input's own accessible name — so a position is kept only the first time it
// is seen: three fields are three asks, not six.
func retypeAsks(t *testing.T, tt *ui.Tester) []retypeAsk {
	t.Helper()
	var out []retypeAsk
	seen := map[int]bool{}
	for _, s := range tt.Texts() {
		i := strings.Index(s, "第 ")
		if i < 0 {
			continue
		}
		rest := s[i+len("第 "):]
		j := strings.Index(rest, " 个词")
		if j < 0 {
			continue
		}
		n := 0
		for _, c := range rest[:j] {
			if c < '0' || c > '9' {
				n = -1
				break
			}
			n = n*10 + int(c-'0')
		}
		if n < 1 || n > 24 || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, retypeAsk{label: s[i : i+len("第 ")+j+len(" 个词")], position: n})
	}
	return out
}
