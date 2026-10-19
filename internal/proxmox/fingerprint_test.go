package proxmox

import (
	"os"
	"path/filepath"
	"testing"
)

const fpGuest = "name: g\nnet0: virtio=BC:24:11:06:1D:B2,bridge=vmbr_imds\n"

// Each input ParseVMConfigs or HostTopology reads changes the fingerprint, and
// nothing else does.
func TestSourceFingerprintFollowsWhatTheLoaderReads(t *testing.T) {
	root := pveTree(t, "hogan", map[string]map[string]string{
		"hogan":  {"399.conf": fpGuest},
		"holger": {"500.conf": fpGuest},
	})
	base := SourceFingerprint(root)
	if SourceFingerprint(root) != base {
		t.Fatal("an unchanged tree changed the fingerprint")
	}

	steps := []struct {
		name   string
		change func(t *testing.T)
	}{
		{"a guest config changes", func(t *testing.T) {
			write(t, filepath.Join(root, "nodes", "hogan", "qemu-server", "399.conf"),
				"name: g\nnet0: virtio=BC:24:11:06:1D:B3,bridge=vmbr_imds\n")
		}},
		{"a guest config appears (created or migrated in)", func(t *testing.T) {
			write(t, filepath.Join(root, "nodes", "hogan", "qemu-server", "401.conf"), fpGuest)
		}},
		{"a guest config is renamed away (migrated out)", func(t *testing.T) {
			if err := os.Rename(
				filepath.Join(root, "nodes", "hogan", "qemu-server", "401.conf"),
				filepath.Join(root, "nodes", "holger", "qemu-server", "401.conf")); err != nil {
				t.Fatal(err)
			}
		}},
		{"the node description appears", func(t *testing.T) {
			write(t, filepath.Join(root, "nodes", "hogan", "config"), "#topology.vtpm_mds.io/zone%3A DUS4\n")
		}},
		{"corosync.conf appears", func(t *testing.T) {
			write(t, filepath.Join(root, "corosync.conf"), "totem {\n  cluster_name: bgl\n}\n")
		}},
		{"the local link moves to another node", func(t *testing.T) {
			if err := os.Remove(filepath.Join(root, "local")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join("nodes", "holger"), filepath.Join(root, "local")); err != nil {
				t.Fatal(err)
			}
		}},
	}
	prev := base
	for _, s := range steps {
		s.change(t)
		got := SourceFingerprint(root)
		if got == prev {
			t.Fatalf("%s: fingerprint unchanged", s.name)
		}
		prev = got
	}

	// Another node's files are not read by the loader (local now points at
	// holger, so change hogan's) and must not change the fingerprint.
	write(t, filepath.Join(root, "nodes", "hogan", "qemu-server", "399.conf"), "name: other\n")
	write(t, filepath.Join(root, "nodes", "hogan", "lrm_status"), "{}")
	if SourceFingerprint(root) != prev {
		t.Fatal("a file the loader does not read changed the fingerprint")
	}
}

// The node name feeds availability-zone and node, so the link target counts
// even when both nodes hold identical files.
func TestSourceFingerprintFollowsTheLinkTarget(t *testing.T) {
	root := pveTree(t, "hogan", map[string]map[string]string{
		"hogan":  {"399.conf": fpGuest},
		"holger": {"399.conf": fpGuest},
	})
	before := SourceFingerprint(root)
	if err := os.Remove(filepath.Join(root, "local")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("nodes", "holger"), filepath.Join(root, "local")); err != nil {
		t.Fatal(err)
	}
	if SourceFingerprint(root) == before {
		t.Fatal("moving local to a node with identical files did not change the fingerprint")
	}
}

func TestSourceFingerprintWithoutALocalLink(t *testing.T) {
	root := pveTree(t, "", map[string]map[string]string{"hogan": {"399.conf": fpGuest}})
	noLink := SourceFingerprint(root)
	if err := os.Symlink(filepath.Join("nodes", "hogan"), filepath.Join(root, "local")); err != nil {
		t.Fatal(err)
	}
	if SourceFingerprint(root) == noLink {
		t.Fatal("the local link appearing did not change the fingerprint")
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
