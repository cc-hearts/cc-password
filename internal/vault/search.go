package vault

import (
	"sort"
	"strings"
)

// Filter narrows a search to a vault, to a display type, or to the trash.
// An empty Filter is everything: no vault, no type, and no deleted items.
type Filter struct {
	VaultID string
	Type    string
	Trash   bool
}

// Search is the vault's list: the entries a query and a filter select, sorted
// the way the list is shown.
//
// The query is split on whitespace and every term must match somewhere in an
// item — its title, username, urls, tags, notes or its own label/value
// fields — case-insensitively. It is an AND, not an OR: "银行 常用" finds the
// item that is both, and "银行 咖啡" finds nothing.
//
// A purged item is never returned, trash filter or not: there is nothing left
// of it to show, and a user who emptied it should not find it again in a list.
func Search(entries []Entry, query string, f Filter) []Entry {
	terms := splitTerms(query)
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if e.Purged {
			continue
		}
		if e.Deleted != f.Trash {
			// The trash filter selects the deleted items, and leaving it
			// out selects everything else: a list and its trash never
			// overlap.
			continue
		}
		if f.VaultID != "" && e.VaultID != f.VaultID {
			continue
		}
		if f.Type != "" && e.Item.DisplayType() != f.Type {
			continue
		}
		if !matches(e.Item, terms) {
			continue
		}
		out = append(out, e)
	}
	sortEntries(out)
	return out
}

// splitTerms is a query's terms: whitespace runs collapse, and leading and
// trailing whitespace is not a term of its own.
func splitTerms(query string) []string {
	var out []string
	for _, t := range strings.Fields(query) {
		out = append(out, strings.ToLower(t))
	}
	return out
}

// matches reports whether every term appears somewhere in the item.
func matches(it Item, terms []string) bool {
	if len(terms) == 0 {
		return true
	}
	// Everything a user might remember an item by is searched, including the
	// labels and values of its own fields: a card's holder is as likely a
	// memory as its title.
	var hay []string
	hay = append(hay, it.Title, it.Username, it.Notes)
	hay = append(hay, it.Tags...)
	hay = append(hay, it.URLs...)
	for _, f := range it.Fields {
		hay = append(hay, f.Label, f.Value)
	}
	haystack := strings.ToLower(strings.Join(hay, "\n"))
	for _, t := range terms {
		if !strings.Contains(haystack, t) {
			return false
		}
	}
	return true
}

// sortEntries is the list's order: by title case-insensitively, then by id, so
// an item has one place in the list and not two.
func sortEntries(entries []Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		// An untitled item sorts last: a list that starts with its blank
		// rows looks broken, and every other item has a name to be found by.
		if (a.Item.Title == "") != (b.Item.Title == "") {
			return b.Item.Title == ""
		}
		ka, kb := titleKey(a.Item.Title), titleKey(b.Item.Title)
		if ka != kb {
			return ka < kb
		}
		return a.ID < b.ID
	})
}

// titleKey is a title's sort key: trimmed and case-folded, so "apple" and
// "Apple" sort together rather than in two blocks.
//
// The comparison itself is by code point, which puts a Latin title before a
// Chinese one. A pinyin order — 招商银行 beside 支付宝 — needs a collation table
// this build does not carry, and the plan names none, so the order is the
// plain one a user can predict: what they see is what the sort key is.
func titleKey(title string) string {
	return strings.ToLower(strings.TrimSpace(title))
}
