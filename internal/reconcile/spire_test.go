package reconcile

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestSPIREAgainstAServer runs the adapter against a real SPIRE server. It runs
// only when VTPM_RECONCILE_SPIRE_SOCKET names the admin socket of a scratch
// server whose trust domain is example.org; it creates and deletes node aliases
// under spiffe://example.org/pve/test-cluster/ and nothing else.
func TestSPIREAgainstAServer(t *testing.T) {
	socket := os.Getenv("VTPM_RECONCILE_SPIRE_SOCKET")
	if socket == "" {
		t.Skip("VTPM_RECONCILE_SPIRE_SOCKET is not set")
	}
	const td, cluster = "example.org", "test-cluster"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s, err := DialSPIRE(socket, td)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	wanted, _ := Wanted(td, Cluster{Name: cluster, Fingerprints: []string{"aa", "bb"}},
		[]Guest{{VMID: "350", Pool: "web", Tags: "vtpm_mds.io.role.db"}})
	pass := func() (created, deleted int) {
		existing, err := s.Entries(ctx)
		if err != nil {
			t.Fatal(err)
		}
		create, del := Diff(td, cluster, existing, wanted)
		if err := s.Delete(ctx, del); err != nil {
			t.Fatal(err)
		}
		if err := s.Create(ctx, create); err != nil {
			t.Fatal(err)
		}
		return len(create), len(del)
	}

	t.Run("a first pass creates every alias", func(t *testing.T) {
		if c, d := pass(); c != len(wanted) || d != 0 {
			t.Fatalf("created %d deleted %d, want %d and 0", c, d, len(wanted))
		}
	})
	t.Run("a second pass changes nothing", func(t *testing.T) {
		if c, d := pass(); c != 0 || d != 0 {
			t.Fatalf("created %d deleted %d, want 0 and 0", c, d)
		}
	})
	t.Run("a removed tag deletes only its aliases", func(t *testing.T) {
		wanted, _ = Wanted(td, Cluster{Name: cluster, Fingerprints: []string{"aa", "bb"}},
			[]Guest{{VMID: "350", Pool: "web"}})
		if c, d := pass(); c != 0 || d != 2 {
			t.Fatalf("created %d deleted %d, want 0 and 2", c, d)
		}
	})
	t.Run("nothing wanted deletes the rest", func(t *testing.T) {
		wanted = nil
		if c, d := pass(); c != 0 || d != 4 {
			t.Fatalf("created %d deleted %d, want 0 and 4", c, d)
		}
	})
	t.Run("agents can be listed", func(t *testing.T) {
		if _, err := s.Agents(ctx); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("banning an unknown agent fails", func(t *testing.T) {
		if err := s.Ban(ctx, "spiffe://"+td+"/spire/agent/tpm_devid/0000"); err == nil {
			t.Fatal("expected an error")
		}
	})
}
