package inventory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestYAMLFingerprintFollowsContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inventory.yaml")
	missing := YAMLFingerprint(path)

	if err := os.WriteFile(path, []byte("vms: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	empty := YAMLFingerprint(path)
	if empty == missing {
		t.Fatal("creating the file did not change the fingerprint")
	}
	if again := YAMLFingerprint(path); again != empty {
		t.Fatal("an unchanged file changed the fingerprint")
	}

	if err := os.WriteFile(path, []byte("vms:\n  - id: \"100\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if YAMLFingerprint(path) == empty {
		t.Fatal("changing the content did not change the fingerprint")
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if YAMLFingerprint(path) != missing {
		t.Fatal("a removed file does not fingerprint like a missing one")
	}
}

func TestFingerprintFieldsDoNotRunTogether(t *testing.T) {
	a := NewFingerprint()
	a.AddString("ab")
	a.AddString("c")
	b := NewFingerprint()
	b.AddString("a")
	b.AddString("bc")
	if a.Sum() == b.Sum() {
		t.Fatal("different field boundaries gave the same fingerprint")
	}

	// Without the length, one field could spell out a second field's kind
	// byte and an empty length.
	c := NewFingerprint()
	c.AddString("x")
	c.AddString("")
	d := NewFingerprint()
	d.AddString("xs\x00\x00\x00\x00\x00\x00\x00\x00")
	if c.Sum() == d.Sum() {
		t.Fatal("one field forged two fields' encoding")
	}
}

func TestFingerprintUnreadableFileDiffersFromMissing(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a 0000 file")
	}
	path := filepath.Join(t.TempDir(), "inventory.yaml")
	missing := YAMLFingerprint(path)
	if err := os.WriteFile(path, []byte("vms: []\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	if YAMLFingerprint(path) == missing {
		t.Fatal("an unreadable file fingerprints like a missing one")
	}
}
