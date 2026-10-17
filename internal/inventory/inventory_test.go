package inventory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "inventory.yaml")
	content := `vms:
  - id: "100"
    name: guest-a
    macs:
      - "52:54:00:12:34:56"
      - "52:54:00:AA:BB:CC"
    ek_sha256: "abcd1234"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write inventory: %v", err)
	}

	m, err := LoadYAML(path)
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	if len(m) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(m))
	}
	vm := m["100"]
	if vm == nil || vm.Name != "guest-a" {
		t.Fatalf("unexpected VM: %#v", vm)
	}
	if GetVMIDByMAC(m, "52:54:00:aa:bb:cc") != "100" {
		t.Fatalf("MAC lookup failed")
	}
	if GetVMIDByMAC(m, "00:11:22:33:44:55") != "" {
		t.Fatalf("expected empty for unknown MAC")
	}
	if vm.EKSHA256 != "abcd1234" {
		t.Fatalf("ek_sha256=%q", vm.EKSHA256)
	}
}

func writeInventory(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "inventory.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write inventory: %v", err)
	}
	return path
}

// TestLoadYAMLPublicIPv4 covers the declared public address: it reaches
// RawConfig under the same key the Proxmox backend produces, an absent one
// leaves no key at all, and the two cases are distinguishable.
func TestLoadYAMLPublicIPv4(t *testing.T) {
	m, err := LoadYAML(writeInventory(t, `vms:
  - id: "100"
    name: declares
    public_ipv4: "203.0.113.7"
  - id: "101"
    name: declares-not
`))
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}

	if got := m["100"].RawConfig["public-ipv4"]; got != "203.0.113.7" {
		t.Errorf("VM 100: expected public-ipv4 203.0.113.7, got %q", got)
	}
	if _, ok := m["101"].RawConfig["public-ipv4"]; ok {
		t.Errorf("VM 101 declares nothing, so the key must be absent, got %q",
			m["101"].RawConfig["public-ipv4"])
	}
	// name survives alongside it, which is what the requirement used to pin as
	// the map's only key.
	if m["100"].RawConfig["name"] != "declares" {
		t.Errorf("VM 100: name lost from RawConfig: %#v", m["100"].RawConfig)
	}
}

// TestLoadYAMLRejectsAMalformedPublicIPv4 is the loud failure the record asks
// for: a typo must stop the load, not degrade to "no public address", which
// would be indistinguishable from not having declared one.
func TestLoadYAMLRejectsAMalformedPublicIPv4(t *testing.T) {
	for _, bad := range []string{
		"not-an-address",
		"2001:db8::1",    // IPv6: a public address, but not an IPv4 one
		"203.0.113.7/32", // CIDR, not an address
		"203.0.113.999",  // out of range
		"203.0.113.7:80", // host:port
	} {
		path := writeInventory(t, `vms:
  - id: "100"
    name: guest
    public_ipv4: "`+bad+`"
`)
		m, err := LoadYAML(path)
		if err == nil {
			t.Errorf("public_ipv4 %q: expected the load to fail, got a usable inventory: %#v", bad, m)
			continue
		}
		if m != nil {
			t.Errorf("public_ipv4 %q: expected no inventory on failure, got %#v", bad, m)
		}
		if !strings.Contains(err.Error(), "public_ipv4") || !strings.Contains(err.Error(), "100") {
			t.Errorf("public_ipv4 %q: the error should name the field and the entry, got %q", bad, err)
		}
	}
}

// TestLoadYAMLAcceptsAnAbsentOrEmptyPublicIPv4 keeps the field optional: every
// inventory written before this change must still load.
func TestLoadYAMLAcceptsAnAbsentOrEmptyPublicIPv4(t *testing.T) {
	for _, content := range []string{
		"vms:\n  - id: \"100\"\n    name: guest\n",
		"vms:\n  - id: \"100\"\n    name: guest\n    public_ipv4: \"\"\n",
		"vms:\n  - id: \"100\"\n    name: guest\n    public_ipv4: \"   \"\n",
	} {
		m, err := LoadYAML(writeInventory(t, content))
		if err != nil {
			t.Errorf("expected this inventory to load, got %v for:\n%s", err, content)
			continue
		}
		if _, ok := m["100"].RawConfig["public-ipv4"]; ok {
			t.Errorf("no address declared, so the key must be absent, got %q for:\n%s",
				m["100"].RawConfig["public-ipv4"], content)
		}
	}
}
