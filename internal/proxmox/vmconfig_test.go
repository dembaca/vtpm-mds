package proxmox

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dembaca/vtpm-mds/internal/inventory"
)

// pveTree builds a directory that plays /etc/pve: one qemu-server directory
// per node, the given config files under each, and local linked to localNode
// unless localNode is empty.
func pveTree(t *testing.T, localNode string, nodes map[string]map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for node, files := range nodes {
		dir := filepath.Join(root, "nodes", node, "qemu-server")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
				t.Fatalf("write %s: %v", name, err)
			}
		}
	}
	if localNode != "" {
		if err := os.Symlink(filepath.Join("nodes", localNode), filepath.Join(root, "local")); err != nil {
			t.Fatalf("link local: %v", err)
		}
	}
	return root
}

func parse(t *testing.T, root string) inventory.VMConfigMap {
	t.Helper()
	m, err := ParseVMConfigs(root)
	if err != nil {
		t.Fatalf("ParseVMConfigs: %v", err)
	}
	return m
}

func TestAGuestConfigBecomesAVMRecord(t *testing.T) {
	m := parse(t, pveTree(t, "hogan", map[string]map[string]string{
		"hogan": {"100.conf": "name: guest-a\nnet0: virtio=52:54:00:A1:B2:C3,bridge=vmbr0\n"},
	}))
	vm := m["100"]
	if vm == nil || vm.Name != "guest-a" || !slices.Equal(vm.MACs, []string{"52:54:00:a1:b2:c3"}) {
		t.Fatalf("unexpected record: %#v", vm)
	}
}

func TestANonVirtioNICYieldsNoMAC(t *testing.T) {
	m := parse(t, pveTree(t, "hogan", map[string]map[string]string{
		"hogan": {"100.conf": "net0: e1000=52:54:00:A1:B2:C3,bridge=vmbr0\n"},
	}))
	vm := m["100"]
	if vm == nil || len(vm.MACs) != 0 {
		t.Fatalf("expected a record with no MACs, got %#v", vm)
	}
	if inventory.GetVMIDByMAC(m, "52:54:00:a1:b2:c3") != "" {
		t.Fatal("a non-virtio NIC bound a caller")
	}
}

func TestNoProxmoxNodeDirectory(t *testing.T) {
	root := pveTree(t, "", map[string]map[string]string{
		"hogan": {"100.conf": "name: guest-a\n"},
	})
	_, err := ParseVMConfigs(root)
	want := filepath.Join(root, "local", "qemu-server")
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("expected an error naming %s, got %v", want, err)
	}
}

func TestAnUnreadableConfigFileIsSkipped(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads files regardless of mode")
	}
	root := pveTree(t, "hogan", map[string]map[string]string{
		"hogan": {"100.conf": "name: guest-a\n", "101.conf": "name: guest-b\n"},
	})
	if err := os.Chmod(filepath.Join(root, "nodes", "hogan", "qemu-server", "101.conf"), 0); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	m := parse(t, root)
	if m["100"] == nil || m["101"] != nil {
		t.Fatalf("expected only 100, got %v", m)
	}
}

func TestThisNodeIsReadNotTheFirstNodeByName(t *testing.T) {
	m := parse(t, pveTree(t, "bbb", map[string]map[string]string{
		"aaa": {"100.conf": "name: on-aaa\nnet0: virtio=52:54:00:00:00:0a\n"},
		"bbb": {"200.conf": "name: on-bbb\nnet0: virtio=52:54:00:00:00:0b\n"},
	}))
	if len(m) != 1 || m["200"] == nil || m["200"].Name != "on-bbb" {
		t.Fatalf("expected only bbb's guest, got %v", m)
	}
	if inventory.GetVMIDByMAC(m, "52:54:00:00:00:0a") != "" {
		t.Fatal("a guest of another node was bound")
	}
}

func TestASnapshotDoesNotBindARemovedNIC(t *testing.T) {
	m := parse(t, pveTree(t, "hogan", map[string]map[string]string{
		"hogan": {"100.conf": `name: guest-b
net0: virtio=52:54:00:00:00:01,bridge=vmbr0

[before-change]
name: guest-old
net0: virtio=52:54:00:00:00:02,bridge=vmbr0
snaptime: 1792000000
`},
	}))
	vm := m["100"]
	if vm == nil || vm.Name != "guest-b" || !slices.Equal(vm.MACs, []string{"52:54:00:00:00:01"}) {
		t.Fatalf("snapshot leaked into the record: %#v", vm)
	}
	if _, ok := vm.RawConfig["snaptime"]; ok {
		t.Fatal("a snapshot key reached the raw configuration")
	}
	if inventory.GetVMIDByMAC(m, "52:54:00:00:00:02") != "" {
		t.Fatal("a MAC only present in a snapshot bound a caller")
	}
}

func TestPendingAndSpecialSectionsAreNotLiveConfiguration(t *testing.T) {
	m := parse(t, pveTree(t, "hogan", map[string]map[string]string{
		"hogan": {"100.conf": `net0: virtio=52:54:00:00:00:01,bridge=vmbr0

[PENDING]
net1: virtio=52:54:00:00:00:03,bridge=vmbr0

[special:cloudinit]
ipconfig0: ip=dhcp
`},
	}))
	vm := m["100"]
	if vm == nil || !slices.Equal(vm.MACs, []string{"52:54:00:00:00:01"}) {
		t.Fatalf("expected only the live NIC, got %#v", vm)
	}
	if _, ok := vm.RawConfig["ipconfig0"]; ok {
		t.Fatal("a special section key reached the raw configuration")
	}
}

func TestAHandWrittenPublicAddressIsNotStored(t *testing.T) {
	for _, line := range []string{"public-ipv4: 203.0.113.7", "public-ipv4=203.0.113.7"} {
		m := parse(t, pveTree(t, "hogan", map[string]map[string]string{
			"hogan": {"100.conf": "name: guest-a\n" + line + "\n"},
		}))
		if v, ok := m["100"].RawConfig["public-ipv4"]; ok {
			t.Fatalf("%q was stored as %q", line, v)
		}
		if m["100"].RawConfig["name"] != "guest-a" {
			t.Fatalf("%q: other keys were lost: %v", line, m["100"].RawConfig)
		}
	}
}

func raw(t *testing.T, tags string) map[string]string {
	t.Helper()
	m := parse(t, pveTree(t, "hogan", map[string]map[string]string{
		"hogan": {"100.conf": "name: guest-a\n" + tags},
	}))
	return m["100"].RawConfig
}

// A zone tag becomes the VM's zone, and declares no region.
func TestAZoneTagBecomesTheVMsZone(t *testing.T) {
	r := raw(t, "tags: mds.zone.rack-b;web\n")
	if r["zone"] != "rack-b" {
		t.Errorf("zone: got %q, want rack-b", r["zone"])
	}
	if _, ok := r["region"]; ok {
		t.Errorf("no region tag, yet region = %q", r["region"])
	}
	if r["tags"] != "mds.zone.rack-b;web" {
		t.Errorf("the tags key itself must still be stored, got %q", r["tags"])
	}
}

// Both kinds, with the lenient separators.
func TestTagsAreSplitAtSemicolonCommaAndSpace(t *testing.T) {
	r := raw(t, "tags: web, mds.zone.a mds.region.r;db\n")
	if r["zone"] != "a" || r["region"] != "r" {
		t.Errorf("got zone %q, region %q; want a and r", r["zone"], r["region"])
	}
}

// Two zone tags declare nothing, and the other kind is unaffected.
func TestTwoZoneTagsDeclareNothing(t *testing.T) {
	r := raw(t, "tags: mds.zone.a;mds.zone.b;mds.region.r\n")
	if _, ok := r["zone"]; ok {
		t.Errorf("two zone tags must declare no zone, got %q", r["zone"])
	}
	if r["region"] != "r" {
		t.Errorf("region: got %q, want r", r["region"])
	}
}

// An invalid tag next to a valid one still counts as two, so a typo is never
// resolved silently in the good tag's favour.
func TestAnInvalidTagNextToAValidOneDeclaresNothing(t *testing.T) {
	r := raw(t, "tags: mds.zone.rack-b;mds.zone.Rack_B!\n")
	if _, ok := r["zone"]; ok {
		t.Errorf("got zone %q, want none", r["zone"])
	}
}

// A single invalid tag is ignored, and another VM's valid tag is unaffected.
func TestAnInvalidPlacementTagIsIgnored(t *testing.T) {
	m := parse(t, pveTree(t, "hogan", map[string]map[string]string{
		"hogan": {
			"100.conf": "tags: mds.zone.\n",
			"101.conf": "tags: mds.zone.rack-b\n",
			"102.conf": "tags: mds.zone.UPPER\n",
		},
	}))
	if _, ok := m["100"].RawConfig["zone"]; ok {
		t.Errorf("VM 100 has an empty zone name, got %q", m["100"].RawConfig["zone"])
	}
	if _, ok := m["102"].RawConfig["zone"]; ok {
		t.Errorf("VM 102 has an invalid zone name, got %q", m["102"].RawConfig["zone"])
	}
	if m["101"].RawConfig["zone"] != "rack-b" {
		t.Errorf("VM 101 was affected: %v", m["101"].RawConfig)
	}
}

// A placement tag in a snapshot section does not count.
func TestAPlacementTagInASnapshotDoesNotCount(t *testing.T) {
	m := parse(t, pveTree(t, "hogan", map[string]map[string]string{
		"hogan": {"100.conf": "name: guest-a\n\n[before-change]\ntags: mds.zone.old\n"},
	}))
	if z, ok := m["100"].RawConfig["zone"]; ok {
		t.Errorf("the snapshot's tag was read: zone %q", z)
	}
}

// A hand-written zone or region line is not stored, in either form.
func TestAHandWrittenZoneAndRegionAreNotStored(t *testing.T) {
	for _, lines := range []string{"zone: rack-x\nregion: bgl-x\n", "zone=rack-x\nregion=bgl-x\n"} {
		r := raw(t, lines)
		for _, key := range []string{"zone", "region"} {
			if v, ok := r[key]; ok {
				t.Errorf("%q: %s was stored as %q", lines, key, v)
			}
		}
	}
}

func hostTree(t *testing.T, node, corosync string) string {
	t.Helper()
	root := pveTree(t, node, map[string]map[string]string{node: {}})
	if corosync != "" {
		if err := os.WriteFile(filepath.Join(root, "corosync.conf"), []byte(corosync), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const corosyncConf = `logging {
  to_syslog: yes
}

totem {
  cluster_name: %s
  config_version: 3
  version: 2
}
`

// The host's placement is this node and this cluster.
func TestTheHostPlacementIsThisNodeAndThisCluster(t *testing.T) {
	got := HostPlacement(hostTree(t, "hogan", strings.Replace(corosyncConf, "%s", "bgl-cluster", 1)))
	if want := (inventory.Placement{Zone: "hogan", Region: "bgl-cluster"}); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// Without a cluster file the region is local, and nothing fails.
func TestAHostWithoutAClusterFileHasRegionLocal(t *testing.T) {
	got := HostPlacement(hostTree(t, "hogan", ""))
	if want := (inventory.Placement{Zone: "hogan", Region: "local"}); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// A cluster file that names no cluster also leaves region local.
func TestAClusterFileWithoutAClusterNameHasRegionLocal(t *testing.T) {
	got := HostPlacement(hostTree(t, "hogan", "totem {\n  version: 2\n}\n"))
	if got.Region != "local" {
		t.Fatalf("got region %q, want local", got.Region)
	}
}

// Capitals in a cluster or node name are lowercased before validation.
func TestHostNamesAreLowercased(t *testing.T) {
	got := HostPlacement(hostTree(t, "Hogan", strings.Replace(corosyncConf, "%s", "BGL-Cluster", 1)))
	if want := (inventory.Placement{Zone: "hogan", Region: "bgl-cluster"}); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// A name that is still not valid after lowercasing is replaced by the default.
func TestAnInvalidHostNameFallsBackToTheDefault(t *testing.T) {
	got := HostPlacement(hostTree(t, "hogan", strings.Replace(corosyncConf, "%s", "bad!name", 1)))
	if got.Region != "local" {
		t.Fatalf("got region %q, want local", got.Region)
	}
	// A root without a local link keeps the default zone.
	noLink := t.TempDir()
	if got := HostPlacement(noLink); got != inventory.DefaultPlacement {
		t.Fatalf("got %+v, want %+v", got, inventory.DefaultPlacement)
	}
}
