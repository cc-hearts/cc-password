package vault

import (
	"strings"
	"testing"
)

// searchSet is a fixed set of entries the queries below are written against,
// with ids chosen so an expected list reads unambiguously.
func searchSet() []Entry {
	return []Entry{
		{ID: "b", VaultID: "v1", Item: Item{Schema: SchemaVersion, Type: TypeLogin, Title: "招商银行", Username: "me@example.com", URLs: []string{"https://bank.example.com/login"}, Tags: []string{"银行", "常用"}}},
		{ID: "a", VaultID: "v1", Item: Item{Schema: SchemaVersion, Type: TypeLogin, Title: "支付宝", Username: "13800000000", Tags: []string{"支付"}}},
		{ID: "c", VaultID: "v2", Item: Item{Schema: SchemaVersion, Type: TypeNote, Title: "Wi-Fi 密码", Notes: "楼下咖啡店的密码是 coffee2024"}},
		{ID: "d", VaultID: "v2", Item: Item{Schema: SchemaVersion, Type: TypeCard, Title: "信用卡", Fields: []Field{{Label: "持卡人", Value: "张三"}}}},
		{ID: "e", VaultID: "v1", Item: Item{Schema: SchemaVersion, Type: "driver-licence", Title: "驾照"}},
		{ID: "f", VaultID: "v1", Deleted: true, Item: Item{Schema: SchemaVersion, Type: TypeLogin, Title: "邮箱", Username: "me@example.com"}},
		{ID: "g", VaultID: "v1", Deleted: true, Purged: true, Item: Item{Schema: SchemaVersion, Type: TypeLogin, Title: "已彻底删除"}},
	}
}

func ids(entries []Entry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.ID)
	}
	return out
}

func TestSearchMatchesATitleSubstring(t *testing.T) {
	got := Search(searchSet(), "银行", Filter{})
	if strings.Join(ids(got), ",") != "b" {
		t.Errorf("got %v, want [b]", ids(got))
	}
}

func TestSearchIsCaseInsensitive(t *testing.T) {
	// A Latin title and a Latin query match whatever their case, which is
	// what a user typing on a phone with a capital letter expects.
	for _, q := range []string{"wi-fi", "WI-FI", "Wi-Fi"} {
		got := Search(searchSet(), q, Filter{})
		if strings.Join(ids(got), ",") != "c" {
			t.Errorf("Search(%q) = %v, want [c]", q, ids(got))
		}
	}
}

func TestSearchMatchesUsernameURLTagsAndNotes(t *testing.T) {
	// Every field a user might remember an item by, not just its title.
	for _, c := range []struct{ query, want string }{
		// f holds the same username, but it is deleted: only the trash
		// filter finds it, which TestSearchTrashShowsOnlyDeletedAndHidesPurged
		// covers.
		{"me@example.com", "b"},
		{"bank.example.com", "b"},
		{"支付", "a"},
		{"coffee2024", "c"},
		{"13800000000", "a"},
		{"张三", "d"},
	} {
		got := Search(searchSet(), c.query, Filter{})
		if strings.Join(ids(got), ",") != c.want {
			t.Errorf("Search(%q) = %v, want %s", c.query, ids(got), c.want)
		}
	}
}

func TestSearchRequiresEveryTermToMatch(t *testing.T) {
	// Terms are split on whitespace and all of them must match, in any
	// order: "银行 常用" and "常用 银行" are the same search.
	for _, q := range []string{"银行 常用", "常用  银行", "银行\t常用"} {
		got := Search(searchSet(), q, Filter{})
		if strings.Join(ids(got), ",") != "b" {
			t.Errorf("Search(%q) = %v, want [b]", q, ids(got))
		}
	}
	// A term that matches nothing leaves nothing: an AND, not an OR.
	if got := Search(searchSet(), "银行 咖啡", Filter{}); len(got) != 0 {
		t.Errorf("Search(%q) = %v, want none", "银行 咖啡", ids(got))
	}
}

func TestSearchAnEmptyQueryReturnsEverythingNotPurged(t *testing.T) {
	got := Search(searchSet(), "", Filter{})
	// The purged item is never in the list, and a deleted one is not either
	// without the trash filter.
	if strings.Join(ids(got), ",") != "c,d,b,a,e" {
		t.Errorf("got %v", ids(got))
	}
}

func TestSearchSortsByTitleCaseInsensitivelyThenByID(t *testing.T) {
	set := []Entry{
		{ID: "y", Item: Item{Type: TypeLogin, Title: "banana"}},
		{ID: "x", Item: Item{Type: TypeLogin, Title: "Apple"}},
		{ID: "w", Item: Item{Type: TypeLogin, Title: "apple"}},
		{ID: "v", Item: Item{Type: TypeLogin, Title: "Banana"}},
		{ID: "u", Item: Item{Type: TypeLogin, Title: ""}},
	}
	// "Apple" and "apple" fold to the same key, so the id breaks the tie;
	// "banana" and "Banana" likewise. An untitled item is last.
	got := Search(set, "", Filter{})
	if strings.Join(ids(got), ",") != "w,x,v,y,u" {
		t.Errorf("got %v, want [w x v y u]", ids(got))
	}
}

func TestSearchFiltersByVault(t *testing.T) {
	got := Search(searchSet(), "", Filter{VaultID: "v2"})
	if strings.Join(ids(got), ",") != "c,d" {
		t.Errorf("got %v, want [d c]", ids(got))
	}
	// A query and a vault filter both apply.
	got = Search(searchSet(), "银行", Filter{VaultID: "v2"})
	if len(got) != 0 {
		t.Errorf("got %v, want none", ids(got))
	}
	// A vault with nothing in it is not an error.
	if got := Search(searchSet(), "", Filter{VaultID: "nope"}); len(got) != 0 {
		t.Errorf("an unknown vault gave %v", ids(got))
	}
}

func TestSearchFiltersByType(t *testing.T) {
	for _, c := range []struct{ typ, want string }{
		{TypeLogin, "b,a"},
		{TypeNote, "c,e"},
		{TypeCard, "d"},
		// An unknown type displays as a note, so the note filter finds it:
		// the filter is on what the user sees, not on the stored type. Its
		// own type finds nothing, because no item is displayed as it.
		{"driver-licence", ""},
	} {
		got := Search(searchSet(), "", Filter{Type: c.typ})
		if strings.Join(ids(got), ",") != c.want {
			t.Errorf("Filter{Type: %q} = %v, want %s", c.typ, ids(got), c.want)
		}
	}
}

func TestSearchTrashShowsOnlyDeletedAndHidesPurged(t *testing.T) {
	got := Search(searchSet(), "", Filter{Trash: true})
	if strings.Join(ids(got), ",") != "f" {
		t.Errorf("the trash = %v, want [f]", ids(got))
	}
	// Without the trash filter a deleted item is not in the list at all.
	got = Search(searchSet(), "", Filter{})
	if strings.Join(ids(got), ",") != "c,d,b,a,e" {
		t.Errorf("the list still holds a deleted item: %v", ids(got))
	}
	// A purged item is never shown, trash or no trash.
	for _, trash := range []bool{false, true} {
		for _, e := range Search(searchSet(), "已彻底删除", Filter{Trash: trash}) {
			if e.Purged {
				t.Error("a purged item was returned")
			}
		}
	}
}

func TestSearchMatchesAPurgedItemByNothing(t *testing.T) {
	// Its title is gone from the payload, so there is nothing to match, but
	// the filter is what guarantees it even if a tombstone carried one.
	got := Search(searchSet(), "已彻底删除", Filter{Trash: true})
	if len(got) != 0 {
		t.Errorf("got %v, want none", ids(got))
	}
}

func TestSearchAnEmptyQueryWithATrashFilterAndNoDeletedItems(t *testing.T) {
	set := []Entry{{ID: "a", Item: Item{Type: TypeLogin, Title: "t"}}}
	if got := Search(set, "", Filter{Trash: true}); len(got) != 0 {
		t.Errorf("an empty trash gave %v", ids(got))
	}
}

func TestSearchOfAnEmptyListIsEmpty(t *testing.T) {
	if got := Search(nil, "anything", Filter{}); len(got) != 0 {
		t.Errorf("got %v", ids(got))
	}
	if got := Search(nil, "", Filter{Trash: true}); len(got) != 0 {
		t.Errorf("got %v", ids(got))
	}
}

func TestSearchDoesNotMutateTheInput(t *testing.T) {
	set := searchSet()
	before := ids(set)
	Search(set, "银行 常用", Filter{})
	if strings.Join(ids(set), ",") != strings.Join(before, ",") {
		t.Errorf("the input was reordered: %v", ids(set))
	}
}

func TestSearchIgnoresSurroundingWhitespaceInAQuery(t *testing.T) {
	for _, q := range []string{"  银行  ", " 银行 ", "银行"} {
		got := Search(searchSet(), q, Filter{})
		if strings.Join(ids(got), ",") != "b" {
			t.Errorf("Search(%q) = %v, want [b]", q, ids(got))
		}
	}
}
