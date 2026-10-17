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
