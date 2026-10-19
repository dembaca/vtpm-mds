package inventory

import (
	"log"
	"os"
	"path/filepath"
	"reflect"
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

// Declared tags reach the record, trimmed, with their case; an entry without
// tags carries none.
func TestLoadYAMLTags(t *testing.T) {
	m, err := LoadYAML(writeInventory(t, `vms:
  - id: "100"
    tags:
      owner: " team-a "
      Rack: b.2
  - id: "101"
`))
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	if got := m["100"].Tags; len(got) != 2 || got["owner"] != "team-a" || got["Rack"] != "b.2" {
		t.Errorf("got tags %v, want owner=team-a and Rack=b.2", got)
	}
	if m["101"].Tags != nil {
		t.Errorf("VM 101 declares no tags, got %v", m["101"].Tags)
	}
}

// An invalid tag is logged and ignored, and does not stop the load, unlike a
// malformed public_ipv4.
func TestLoadYAMLIgnoresAnInvalidTag(t *testing.T) {
	var buf strings.Builder
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	m, err := LoadYAML(writeInventory(t, `vms:
  - id: "100"
    tags:
      owner: team-a
      "bad key": x
      empty: ""
      dotted.key: x
  - id: "101"
    tags:
      owner: team-b
`))
	if err != nil {
		t.Fatalf("an invalid tag must not fail the load: %v", err)
	}
	if got := m["100"].Tags; len(got) != 1 || got["owner"] != "team-a" {
		t.Errorf("got tags %v, want only owner=team-a", got)
	}
	if m["101"].Tags["owner"] != "team-b" {
		t.Errorf("another entry was affected: %v", m["101"].Tags)
	}
	for _, want := range []string{`"100"`, "bad key"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("the log lacks %s: %q", want, buf.String())
		}
	}
}

// A zone and a region in an entry no longer place anything: the load succeeds,
// the record carries neither anywhere, and the log says so.
func TestLoadYAMLFormerZoneAndRegionPlaceNothing(t *testing.T) {
	var buf strings.Builder
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	m, err := LoadYAML(writeInventory(t, `vms:
  - id: "100"
    zone: rack-b
    region: bgl-1
`))
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	for _, key := range []string{"zone", "region"} {
		if v, ok := m["100"].RawConfig[key]; ok {
			t.Errorf("%s reached the record as %q", key, v)
		}
	}
	if m["100"].Tags != nil {
		t.Errorf("got tags %v, want none", m["100"].Tags)
	}
	if !strings.Contains(buf.String(), "legacy zone") || !strings.Contains(buf.String(), "legacy region") {
		t.Errorf("the log must name both, got %q", buf.String())
	}
}

func TestValidTagKeyAndValue(t *testing.T) {
	for key, want := range map[string]bool{
		"owner": true, "Rack-B": true, "a_b+c": true, strings.Repeat("k", 63): true,
		"": false, "a.b": false, "a b": false, "a/b": false, "a:b": false, strings.Repeat("k", 64): false,
	} {
		if got := ValidTagKey(key); got != want {
			t.Errorf("ValidTagKey(%q) = %v, want %v", key, got, want)
		}
	}
	for value, want := range map[string]bool{
		"team-a": true, "b.2.07": true, "A_b+c": true, strings.Repeat("v", 255): true,
		"": false, "a b": false, "a/b": false, "a:b": false, "a=b": false, strings.Repeat("v", 256): false,
	} {
		if got := ValidTagValue(value); got != want {
			t.Errorf("ValidTagValue(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestValidPlacementName(t *testing.T) {
	long := strings.Repeat("a", 63)
	for name, want := range map[string]bool{
		"hogan": true, "rack-b": true, "a": true, "0x": true, "a_b.c+d-e": true, long: true,
		"DUS4": true, "BGL-Cluster": true,
		"": false, "-a": false, ".a": false, "a b": false, "a!": false,
		long + "a": false, "a/b": false,
	} {
		if got := ValidPlacementName(name); got != want {
			t.Errorf("ValidPlacementName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestHostTopologyDefaultsUntilSet(t *testing.T) {
	SetHostTopology(DefaultTopology())
	if got := HostTopology(); !reflect.DeepEqual(got, DefaultTopology()) {
		t.Fatalf("got %+v, want the default %+v", got, DefaultTopology())
	}
	SetHostTopology(Topology{"availability-zone": "hogan", "region": "bgl", "rack": "r1"})
	defer SetHostTopology(DefaultTopology())
	if got := HostTopology(); got["availability-zone"] != "hogan" || got["rack"] != "r1" || len(got) != 3 {
		t.Fatalf("got %+v after SetHostTopology", got)
	}
}

// What a caller holds is a copy: changing it never changes the host's topology.
func TestHostTopologyIsACopy(t *testing.T) {
	SetHostTopology(Topology{"availability-zone": "hogan", "region": "bgl"})
	defer SetHostTopology(DefaultTopology())
	got := HostTopology()
	got["availability-zone"] = "changed"
	if HostTopology()["availability-zone"] != "hogan" {
		t.Fatal("the returned topology aliases the host's")
	}
}

func TestValidTopologyKey(t *testing.T) {
	for key, want := range map[string]bool{
		"rack": true, "availability-zone": true, "Zone": true, "host-id": true, "a_b+c": true,
		"": false, "a.b": false, "a b": false, "a/b": false, strings.Repeat("k", 64): false,
	} {
		if got := ValidTopologyKey(key); got != want {
			t.Errorf("ValidTopologyKey(%q) = %v, want %v", key, got, want)
		}
	}
}

func TestLoadYAMLRejectsUnknownKey(t *testing.T) {
	path := writeInventory(t, `vms:
  - id: "100"
    name: guest-a
    macs: ["52:54:00:12:34:56"]
    not_a_real_field: true
`)
	if _, err := LoadYAML(path); err == nil {
		t.Fatal("expected error for unknown VM field")
	}
}
