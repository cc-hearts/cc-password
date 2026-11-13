package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTheFileOnlyItsOwnerCanRead: a file holding the mnemonic is a file
// holding the account. It is written where the user said, and it is theirs:
// nobody else on the machine gets to open it, including a file that was
// already there with its doors open.
func TestTheFileOnlyItsOwnerCanRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bipPass-助记词.txt")
	// The file is already there, world-readable, as one a user exported
	// before — or one they picked by accident — would be.
	if err := os.WriteFile(path, []byte("something else\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeMnemonic(path, "abandon amount"); err != nil {
		t.Fatalf("writing the mnemonic: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// The words are exactly what was handed over: no heading, no date, nothing
	// a recovery screen would have to be taught to strip.
	if string(got) != "abandon amount\n" {
		t.Errorf("the file holds %q", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("the file is %#o, want 0600", perm)
	}
}
