package inventory

import (
	"os"
	"path/filepath"
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
}
