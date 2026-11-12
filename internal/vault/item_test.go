package vault

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// roundTrip marshals an item and reads it back, which is what every re-save
// does.
func roundTrip(t *testing.T, it Item) Item {
	t.Helper()
	raw, err := MarshalItem(it)
	if err != nil {
		t.Fatal(err)
	}
	back, err := UnmarshalItem(raw)
	if err != nil {
		t.Fatal(err)
	}
	return back
}

func TestItemCarriesTheSpecFields(t *testing.T) {
	it := Item{
		Type:      TypeLogin,
		Title:     "招商银行",
		VaultID:   "v1",
		CreatedAt: 1700000000,
		UpdatedAt: 1700000100,
		Tags:      []string{"银行", "常用"},
		URLs:      []string{"https://example.com"},
		Username:  "me@example.com",
		Password:  "a-password",
		Notes:     "some notes",
		OTP:       &OTP{Secret: "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", Digits: 6, Period: 30, Algorithm: "SHA1"},
	}
	back := roundTrip(t, it)
	if back.Type != TypeLogin || back.Title != it.Title || back.VaultID != it.VaultID {
		t.Errorf("type/title/vault = %q/%q/%q", back.Type, back.Title, back.VaultID)
	}
	if back.CreatedAt != it.CreatedAt || back.UpdatedAt != it.UpdatedAt {
		t.Errorf("timestamps = %d/%d", back.CreatedAt, back.UpdatedAt)
	}
	if strings.Join(back.Tags, ",") != strings.Join(it.Tags, ",") {
		t.Errorf("tags = %v", back.Tags)
	}
	if strings.Join(back.URLs, ",") != strings.Join(it.URLs, ",") {
		t.Errorf("urls = %v", back.URLs)
	}
	if back.Username != it.Username || back.Password != it.Password || back.Notes != it.Notes {
		t.Errorf("credentials = %q/%q/%q", back.Username, back.Password, back.Notes)
	}
	if back.OTP == nil || *back.OTP != *it.OTP {
		t.Errorf("otp = %+v, want %+v", back.OTP, it.OTP)
	}
	// The schema version is written, so a future format can be recognised.
	raw, _ := MarshalItem(it)
	if !strings.Contains(string(raw), `"schema"`) {
		t.Error("the payload has no schema")
	}
}

func TestUnknownJSONFieldsSurviveARoundTrip(t *testing.T) {
	// A payload from a newer build, or from an import, carries fields this
	// one does not know. They are kept verbatim and written back out, so a
	// re-save loses nothing.
	raw := []byte(`{
	  "schema": 1,
	  "type": "login",
	  "title": "某处",
	  "username": "me",
	  "password": "pw",
	  "somethingNew": {"a": [1, 2, 3]},
	  "anotherUnknown": "kept",
	  "fields": [{"label": "卡号", "value": "4111", "kind": "text", "secret": true}],
	  "passwordHistory": [{"password": "old", "changedAt": 1690000000}]
	}`)
	it, err := UnmarshalItem(raw)
	if err != nil {
		t.Fatal(err)
	}
	if it.Title != "某处" || it.Username != "me" || it.Password != "pw" {
		t.Fatalf("the known fields did not survive: %+v", it)
	}
	out, err := MarshalItem(it)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"somethingNew"`, `"anotherUnknown"`, `"kept"`,
		`"fields"`, `"卡号"`, `"passwordHistory"`, `"old"`,
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("the re-saved payload lost %s", want)
		}
	}
	// And the unknown values are the same values, not re-encoded into a
	// different shape: whitespace may be compacted, nothing else may move.
	var a, b map[string]any
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out, &b); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"somethingNew", "anotherUnknown", "fields", "passwordHistory"} {
		if !reflect.DeepEqual(a[k], b[k]) {
			t.Errorf("%s changed: %v → %v", k, a[k], b[k])
		}
	}
}

func TestAnUnknownTypeIsReadOnlyButKeepsItsData(t *testing.T) {
	raw := []byte(`{"schema":1,"type":"driver-licence","title":"驾照","state":"CA"}`)
	it, err := UnmarshalItem(raw)
	if err != nil {
		t.Fatal(err)
	}
	// It displays as a note, and its own type and extra fields are kept for
	// the re-save: an unknown type is a type another manager added, not a
	// future bipPass, so editing it as a note loses nothing.
	if got := it.DisplayType(); got != TypeNote {
		t.Errorf("DisplayType() = %q, want %q", got, TypeNote)
	}
	if it.ReadOnly() {
		t.Error("an unknown type is read-only, so it can never be edited")
	}
	out, err := MarshalItem(it)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"driver-licence"`, `"state"`, `"驾照"`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("the re-save lost %s", want)
		}
	}
}

func TestASchemaNewerThanThisBuildIsReadOnly(t *testing.T) {
	for _, v := range []int{2, 3, 99} {
		it := Item{Schema: v, Type: TypeLogin, Title: "t"}
		if !it.ReadOnly() {
			t.Errorf("schema %d is not read-only", v)
		}
	}
	if (Item{Schema: SchemaVersion, Type: TypeLogin}).ReadOnly() {
		t.Error("the current schema is read-only")
	}
	if (Item{Schema: SchemaVersion - 1, Type: TypeLogin}).ReadOnly() {
		t.Error("an older schema is read-only")
	}
}

func TestTemplateFields(t *testing.T) {
	// Card and identity items are pre-filled with the labels a user expects,
	// so an import or a new item does not start from a blank form.
	for _, c := range []struct {
		typ  string
		want []string
	}{
		{TypeCard, []string{"卡号", "持卡人", "有效期", "CVV"}},
		{TypeIdentity, []string{"姓名", "证件号", "生日", "签发日", "到期日"}},
	} {
		fields := TemplateFields(c.typ)
		if len(fields) != len(c.want) {
			t.Fatalf("%s: %d fields, want %d", c.typ, len(fields), len(c.want))
		}
		for i, w := range c.want {
			if fields[i].Label != w {
				t.Errorf("%s field %d = %q, want %q", c.typ, i, fields[i].Label, w)
			}
		}
	}
	// A login or a note has no template: the form is its own.
	for _, typ := range []string{TypeLogin, TypeNote, "", "driver-licence"} {
		if got := TemplateFields(typ); len(got) != 0 {
			t.Errorf("TemplateFields(%q) = %+v, want none", typ, got)
		}
	}
}

func TestFieldsRoundTrip(t *testing.T) {
	it := Item{
		Type:  TypeCard,
		Title: "信用卡",
		Fields: []Field{
			{Label: "卡号", Value: "4111111111111111", Kind: "text", Secret: true},
			{Label: "备注", Value: "不要外泄", Kind: "text"},
		},
	}
	back := roundTrip(t, it)
	if len(back.Fields) != 2 {
		t.Fatalf("%d fields", len(back.Fields))
	}
	if back.Fields[0] != it.Fields[0] || back.Fields[1] != it.Fields[1] {
		t.Errorf("fields = %+v", back.Fields)
	}
}

func TestPasswordHistoryRoundTrip(t *testing.T) {
	it := Item{
		Type:     TypeLogin,
		Title:    "某处",
		Username: "me",
		Password: "current",
		History: []PasswordChange{
			{Password: "older", ChangedAt: 1690000000},
			{Password: "oldest", ChangedAt: 1680000000},
		},
	}
	back := roundTrip(t, it)
	if len(back.History) != 2 {
		t.Fatalf("%d history entries", len(back.History))
	}
	if back.History[0] != it.History[0] || back.History[1] != it.History[1] {
		t.Errorf("history = %+v", back.History)
	}
	// The current password is not in the history: changing it moves the old
	// one in, and the history keeps its own order.
	if back.Password != "current" {
		t.Errorf("password = %q", back.Password)
	}
}

func TestUnmarshalItemRefusesGarbage(t *testing.T) {
	for _, in := range []string{
		"",
		"not json",
		"[]",
		"null",
		`{"title": 5}`,
		`{"tags": "not-a-list"}`,
	} {
		if _, err := UnmarshalItem([]byte(in)); err == nil {
			t.Errorf("UnmarshalItem accepted %q", in)
		}
	}
}

func TestMarshalItemLeavesOutEmptyValues(t *testing.T) {
	it := Item{Type: TypeLogin, Title: "t"}
	raw, err := MarshalItem(it)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"username"`, `"password"`, `"notes"`, `"urls"`, `"tags"`, `"otp"`, `"fields"`, `"passwordHistory"`} {
		if strings.Contains(string(raw), field) {
			t.Errorf("an empty item carries %s", field)
		}
	}
	// A vault id and the timestamps are always there: the engine fills them
	// in, and a payload without them cannot be re-sealed.
	if !strings.Contains(string(raw), `"type"`) || !strings.Contains(string(raw), `"title"`) {
		t.Error("the item's own fields are missing")
	}
}

func TestOTPFieldNamesMatchTheCryptoPackage(t *testing.T) {
	// The payload's otp object and vaultcrypto.OTP have to spell their
	// fields the same way, or a stored code stops working.
	raw, err := MarshalItem(Item{Type: TypeLogin, OTP: &OTP{Secret: "S", Digits: 8, Period: 60, Algorithm: "SHA256"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"secret"`, `"digits"`, `"period"`, `"algorithm"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the otp object has no %s", want)
		}
	}
}

func TestDisplayTypeForEachType(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{TypeLogin, TypeLogin},
		{TypeNote, TypeNote},
		{TypeCard, TypeCard},
		{TypeIdentity, TypeIdentity},
		{"", TypeNote},
		{"driver-licence", TypeNote},
		{"LOGIN", TypeNote},
	} {
		it := Item{Type: c.in}
		if got := it.DisplayType(); got != c.want {
			t.Errorf("DisplayType(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
