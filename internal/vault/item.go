package vault

import (
	"encoding/json"
	"fmt"

	"bipPass/internal/vaultcrypto"
)

// The item types. An item this build does not know displays as a note and
// keeps its own type for the re-save, so an import from another manager
// loses nothing.
const (
	TypeLogin    = "login"
	TypeNote     = "note"
	TypeCard     = "card"
	TypeIdentity = "identity"
)

// SchemaVersion is the payload version this build writes. It rises only on a
// breaking change: a new optional field does not touch it, so a newer build's
// payload still opens in an older one.
const SchemaVersion = 1

// OTP is a stored two-factor secret. It is vaultcrypto.OTP, re-exported so a
// caller of the vault engine does not have to import the crypto package to
// read an item.
type OTP = vaultcrypto.OTP

// Field is one label/value pair of an item's own fields: the escape hatch for
// anything a login form has that bipPass does not name, and where a card's or
// an identity's specific fields live.
type Field struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Kind   string `json:"kind,omitempty"`
	Secret bool   `json:"secret,omitempty"`
}

// PasswordChange is one old password of an item, kept so the user can see what
// they used to use.
type PasswordChange struct {
	Password  string `json:"password"`
	ChangedAt int64  `json:"changedAt"`
}

// Item is a decrypted payload. Everything in it was inside the sealed blob,
// and everything in it goes back into one on a re-save.
type Item struct {
	Schema    int    `json:"schema"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	VaultID   string `json:"vaultId,omitempty"`
	CreatedAt int64  `json:"createdAt,omitempty"`
	UpdatedAt int64  `json:"updatedAt,omitempty"`

	Tags     []string `json:"tags,omitempty"`
	URLs     []string `json:"urls,omitempty"`
	Username string   `json:"username,omitempty"`
	Password string   `json:"password,omitempty"`
	OTP      *OTP     `json:"otp,omitempty"`
	Notes    string   `json:"notes,omitempty"`

	Fields  []Field          `json:"fields,omitempty"`
	History []PasswordChange `json:"passwordHistory,omitempty"`

	// extra is every JSON key this build does not name. It is written back
	// out unchanged, so a re-save by an older build loses nothing.
	extra map[string]json.RawMessage
}

// DisplayType is the type to show the user. An unknown type is a note: the
// data is all still there, and the form for a type this build does not know
// would be a guess.
func (it Item) DisplayType() string {
	switch it.Type {
	case TypeLogin, TypeNote, TypeCard, TypeIdentity:
		return it.Type
	}
	return TypeNote
}

// ReadOnly reports whether the item may be edited. A payload whose schema is
// newer than this build's is shown but not re-saved: writing it back would
// drop the fields the newer format added.
func (it Item) ReadOnly() bool {
	return it.Schema > SchemaVersion
}

// MarshalItem encodes a payload. Unknown keys are written back exactly as they
// were read, which is what makes a re-save lossless.
func MarshalItem(it Item) ([]byte, error) {
	if it.Schema == 0 {
		it.Schema = SchemaVersion
	}
	// Encode through a map so the unknown keys ride along: a struct encode
	// would drop them, and re-encoding the raw message would drop this
	// build's own edits.
	out := map[string]json.RawMessage{}
	for k, v := range it.extra {
		out[k] = v
	}
	own, err := json.Marshal(it)
	if err != nil {
		return nil, fmt.Errorf("vault: writing the item: %w", err)
	}
	var ownFields map[string]json.RawMessage
	if err := json.Unmarshal(own, &ownFields); err != nil {
		return nil, fmt.Errorf("vault: writing the item: %w", err)
	}
	for k, v := range ownFields {
		out[k] = v
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("vault: writing the item: %w", err)
	}
	return raw, nil
}

// UnmarshalItem reads a payload. Unknown keys are kept for the re-save, and
// unknown types are kept as they are.
func UnmarshalItem(raw []byte) (Item, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return Item{}, fmt.Errorf("vault: reading the item: %w", err)
	}
	var it Item
	if err := json.Unmarshal(raw, &it); err != nil {
		return Item{}, fmt.Errorf("vault: reading the item: %w", err)
	}
	// A payload that is not an object at all is refused: a list or a string
	// would decode into a zero Item and be re-saved as an empty one.
	if len(probe) == 0 {
		return Item{}, fmt.Errorf("vault: the payload is not an item")
	}
	it.extra = map[string]json.RawMessage{}
	for k, v := range probe {
		switch k {
		case "schema", "type", "title", "vaultId", "createdAt", "updatedAt",
			"tags", "urls", "username", "password", "otp", "notes",
			"fields", "passwordHistory":
			continue
		}
		it.extra[k] = v
	}
	if it.Schema == 0 {
		it.Schema = SchemaVersion
	}
	return it, nil
}

// TemplateFields is the pre-filled field list of a new item of that type. A
// login or a note has no template: its form is its own, and a card's or an
// identity's labels are guesses a user should not have to make.
func TemplateFields(typ string) []Field {
	switch typ {
	case TypeCard:
		return []Field{
			{Label: "卡号", Kind: "text", Secret: true},
			{Label: "持卡人", Kind: "text"},
			{Label: "有效期", Kind: "text", Secret: true},
			{Label: "CVV", Kind: "text", Secret: true},
		}
	case TypeIdentity:
		return []Field{
			{Label: "姓名", Kind: "text"},
			{Label: "证件号", Kind: "text", Secret: true},
			{Label: "生日", Kind: "text"},
			{Label: "签发日", Kind: "text"},
			{Label: "到期日", Kind: "text"},
		}
	}
	return nil
}
