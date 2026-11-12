package vaultcrypto

import (
	"bytes"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"testing"
)

// bip39Vectors are the 24-word English vectors of the BIP39 reference
// implementation (github.com/trezor/python-mnemonic, vectors.json): the
// entropy, its mnemonic, and the seed for the passphrase "TREZOR".
var bip39Vectors = []struct{ entropy, mnemonic, seed string }{
	{
		"0000000000000000000000000000000000000000000000000000000000000000",
		"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art",
		"bda85446c68413707090a52022edd26a1c9462295029f2e60cd7c4f2bbd3097170af7a4d73245cafa9c3cca8d561a7c3de6f5d4a10be8ed2a5e608d68f92fcc8",
	},
	{
		"7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f",
		"legal winner thank year wave sausage worth useful legal winner thank year wave sausage worth useful legal winner thank year wave sausage worth title",
		"bc09fca1804f7e69da93c2f2028eb238c227f2e9dda30cd63699232578480a4021b146ad717fbb7e451ce9eb835f43620bf5c514db0f8add49f5d121449d3e87",
	},
	{
		"8080808080808080808080808080808080808080808080808080808080808080",
		"letter advice cage absurd amount doctor acoustic avoid letter advice cage absurd amount doctor acoustic avoid letter advice cage absurd amount doctor acoustic bless",
		"c0c519bd0e91a2ed54357d9d1ebef6f5af218a153624cf4f2da911a0ed8f7a09e2ef61af0aca007096df430022f7a2b6fb91661a9589097069720d015e4e982f",
	},
	{
		"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		"zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo vote",
		"dd48c104698c30cfe2b6142103248622fb7bb0ff692eebb00089b32d22484e1613912f0a5b694407be899ffd31ed3992c456cdf60f5d4564b8ba3f05a69890ad",
	},
	{
		"68a79eaca2324873eacc50cb9c6eca8cc68ea5d936f98787c60c7ebc74e6ce7c",
		"hamster diagram private dutch cause delay private meat slide toddler razor book happy fancy gospel tennis maple dilemma loan word shrug inflict delay length",
		"64c87cde7e12ecf6704ab95bb1408bef047c22db4cc7491c4271d170a1b213d20b385bc1588d9c7b38f1b39d415665b8a9030c9ec653d75e65f847d8fc1fc440",
	},
	{
		"9f6a2878b2520799a44ef18bc7df394e7061a224d2c33cd015b157d746869863",
		"panda eyebrow bullet gorilla call smoke muffin taste mesh discover soft ostrich alcohol speed nation flash devote level hobby quick inner drive ghost inside",
		"72be8e052fc4919d2adf28d5306b5474b0069df35b02303de8c1729c9538dbb6fc2d731d5f832193cd9fb6aeecbc469594a70e3dd50811b5067f3b88b28c3e8d",
	},
	{
		"066dca1a2bb7e8a1db2832148ce9933eea0f3ac9548d793112d9a95c9407efad",
		"all hour make first leader extend hole alien behind guard gospel lava path output census museum junior mass reopen famous sing advance salt reform",
		"26e975ec644423f4a4c4f4215ef09b4bd7ef924e85d1d17c4cf3f136c2863cf6df0a475045652c57eb5fb41513ca2a2d67722b77e954b4b3fc11f7590449191d",
	},
	{
		"f585c11aec520db57dd353c69554b21a89b20fb0650966fa0a9d6f74fd989d8f",
		"void come effort suffer camp survey warrior heavy shoot primary clutch crush open amazing screen patrol group space point ten exist slush involve unfold",
		"01f5bced59dec48e362f2c45b5de68b9fd6c92c6634f44d6d40aab69056506f0e35524a518034ddc1192e1dacd32c1ed3eaa3c3b131c88ed8e7e54c49a5d0998",
	},
}

func TestBIP39WordlistIsTheOfficialEnglishList(t *testing.T) {
	sum := sha256.Sum256([]byte(englishWordlist))
	if got := hex.EncodeToString(sum[:]); got != "2f5eed53a4727b4bf8880d8f3f199efc90e58503646d9ff8eff3a2ed3b24dbda" {
		t.Fatalf("english.txt sha256 = %s, not the BIP39 English list", got)
	}
}

func TestMnemonicFromEntropyMatchesBIP39Vectors(t *testing.T) {
	for _, v := range bip39Vectors {
		entropy, _ := hex.DecodeString(v.entropy)
		got, err := MnemonicFromEntropy(entropy)
		if err != nil {
			t.Fatalf("MnemonicFromEntropy(%s): %v", v.entropy, err)
		}
		if got != v.mnemonic {
			t.Errorf("MnemonicFromEntropy(%s)\n got %q\nwant %q", v.entropy, got, v.mnemonic)
		}
	}
}

func TestMnemonicSeedMatchesBIP39Vectors(t *testing.T) {
	for _, v := range bip39Vectors {
		if err := ValidateMnemonic(v.mnemonic); err != nil {
			t.Errorf("ValidateMnemonic(vector %s\U00002026): %v", v.entropy[:8], err)
		}
		if got := hex.EncodeToString(MnemonicSeed(v.mnemonic, "TREZOR")); got != v.seed {
			t.Errorf("MnemonicSeed(vector %s\U00002026) = %s, want %s", v.entropy[:8], got, v.seed)
		}
	}
}

func TestMnemonicSeedNormalizesThePassphrase(t *testing.T) {
	m := bip39Vectors[0].mnemonic
	// U+FB01 (the "fi" ligature) is "fi" under NFKD.
	want, err := pbkdf2.Key(sha512.New, m, []byte("mnemonicfi"), 2048, 64)
	if err != nil {
		t.Fatal(err)
	}
	if got := MnemonicSeed(m, "\U0000FB01"); !bytes.Equal(got, want) {
		t.Fatalf("seed for passphrase U+FB01 = %x, want the seed for \"fi\" %x", got, want)
	}
}

func TestGenerateMnemonicFromZeroEntropy(t *testing.T) {
	got, err := GenerateMnemonic(bytes.NewReader(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.Repeat("abandon ", 23) + "art"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestGenerateMnemonicFromCryptoRandValidates(t *testing.T) {
	seen := map[string]bool{}
	for range 20 {
		m, err := GenerateMnemonic(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		if n := len(strings.Fields(m)); n != 24 {
			t.Fatalf("generated %d words", n)
		}
		if err := ValidateMnemonic(m); err != nil {
			t.Fatalf("a generated mnemonic does not validate: %v", err)
		}
		seen[m] = true
	}
	if len(seen) != 20 {
		t.Fatalf("20 generated mnemonics hold %d distinct ones", len(seen))
	}
}

func TestGenerateMnemonicFailsOnShortRandomness(t *testing.T) {
	if _, err := GenerateMnemonic(bytes.NewReader(make([]byte, 31))); err == nil {
		t.Fatal("31 bytes of randomness made a mnemonic")
	}
}

func TestMnemonicFromEntropyRejectsOtherSizes(t *testing.T) {
	for _, n := range []int{0, 16, 31, 33} {
		if _, err := MnemonicFromEntropy(make([]byte, n)); err == nil {
			t.Errorf("%d bytes of entropy made a mnemonic", n)
		}
	}
}

func TestNormalizeMnemonicFoldsPastedText(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"case and line breaks", "  Hamster\r\nDIAGRAM\tprivate  ", "hamster diagram private"},
		{"ideographic space and NBSP", "hamster\U00003000diagram\U000000A0private", "hamster diagram private"},
		{"full-width letters", "\U0000FF48\U0000FF41\U0000FF4D\U0000FF53\U0000FF54\U0000FF45\U0000FF52 \U0000FF24\U0000FF29\U0000FF21\U0000FF27\U0000FF32\U0000FF21\U0000FF2D", "hamster diagram"},
		{"zero-width characters", "ham\U0000200Bster\U0000FEFF diagram\U000000AD", "hamster diagram"},
		{"blank lines", "hamster\n\n\ndiagram\n", "hamster diagram"},
		{"empty", " \n\t", ""},
	}
	for _, tc := range cases {
		if got := NormalizeMnemonic(tc.in); got != tc.want {
			t.Errorf("%s: NormalizeMnemonic(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

// fullWidth spells ASCII letters with their full-width forms, as some
// Chinese input methods type them.
func fullWidth(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r - 'a' + 0xff41
		case r >= 'A' && r <= 'Z':
			return r - 'A' + 0xff21
		}
		return r
	}, s)
}

func TestValidateMnemonicAcceptsAPastedNote(t *testing.T) {
	v := bip39Vectors[4]
	w := strings.Fields(v.mnemonic)
	pasted := "  " + strings.ToUpper(strings.Join(w[0:6], "\n")) + "\r\n" +
		strings.Join(w[6:12], "\U00003000") + "\t" +
		strings.Join(w[12:18], "\U000000A0") + "\n\n" +
		fullWidth(strings.Join(w[18:24], " ")) + "\n"
	if err := ValidateMnemonic(pasted); err != nil {
		t.Fatalf("ValidateMnemonic(pasted note): %v", err)
	}
	if got := hex.EncodeToString(MnemonicSeed(pasted, "TREZOR")); got != v.seed {
		t.Fatalf("seed of the pasted note = %s, want %s", got, v.seed)
	}
}

func TestValidateMnemonicRejections(t *testing.T) {
	base := strings.Fields(bip39Vectors[4].mnemonic)
	unknown := slices.Clone(base)
	unknown[6] = "zzsecretzz"
	cases := []struct {
		name            string
		in              string
		kind            error
		words, position int
	}{
		{"23 words", strings.Join(base[:23], " "), ErrMnemonicLength, 23, 0},
		{"25 words", strings.Join(append(slices.Clone(base), "abandon"), " "), ErrMnemonicLength, 25, 0},
		{"a valid 12-word mnemonic", strings.Repeat("abandon ", 11) + "about", ErrMnemonicLength, 12, 0},
		{"nothing", "  ", ErrMnemonicLength, 0, 0},
		{"unknown word 7", strings.Join(unknown, " "), ErrMnemonicWord, 24, 7},
		// "art" carries the checksum of the zero entropy.
		{"bad checksum", strings.Repeat("abandon ", 24), ErrMnemonicChecksum, 24, 0},
	}
	for _, tc := range cases {
		err := ValidateMnemonic(tc.in)
		if !errors.Is(err, tc.kind) {
			t.Errorf("%s: got %v, want %v", tc.name, err, tc.kind)
			continue
		}
		var me *MnemonicError
		if !errors.As(err, &me) {
			t.Errorf("%s: %T is not a *MnemonicError", tc.name, err)
			continue
		}
		if me.Words != tc.words || me.Position != tc.position {
			t.Errorf("%s: words %d position %d, want %d and %d", tc.name, me.Words, me.Position, tc.words, tc.position)
		}
		msg := err.Error()
		if first := strings.Fields(tc.in); strings.Contains(msg, "zzsecretzz") || len(first) > 0 && strings.Contains(msg, first[0]) {
			t.Errorf("%s: error %q repeats words of the mnemonic", tc.name, msg)
		}
	}
}
