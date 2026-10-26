package bridgebind

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeNFT holds one table; Apply replaces it with the script, List returns it.
type fakeNFT struct {
	table    string
	exists   bool
	applies  int
	attempts int
	failing  bool
	failErr  string
}

func (f *fakeNFT) Apply(script string) error {
	f.attempts++
	if f.failing {
		if f.failErr != "" {
			return errors.New(f.failErr)
		}
		return errors.New("netlink: Operation not permitted")
	}
	f.applies++
	f.table, f.exists = script, true
	return nil
}

func (f *fakeNFT) List() (string, error) {
	if !f.exists {
		return "", errors.New("No such file or directory")
	}
	return f.table, nil
}

type rig struct {
	t    *testing.T
	root string
	nft  *fakeNFT
	b    *Binder
	logs []string
}

func newRig(t *testing.T, confs map[string]string) *rig {
	t.Helper()
	r := &rig{t: t, root: t.TempDir(), nft: &fakeNFT{}}
	dir := filepath.Join(r.root, "nodes", "node-a", "qemu-server")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("nodes", "node-a"), filepath.Join(r.root, "local")); err != nil {
		t.Fatal(err)
	}
	for name, content := range confs {
		r.write(name, content)
	}
	r.b = &Binder{PVERoot: r.root, Bridge: "vmbr_imds", NFT: r.nft,
		Logf: func(f string, a ...any) { r.logs = append(r.logs, fmt.Sprintf(f, a...)) }}
	return r
}

func (r *rig) write(name, content string) {
	r.t.Helper()
	path := filepath.Join(r.root, "nodes", "node-a", "qemu-server", name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *rig) count(substr string) int {
	n := 0
	for _, l := range r.logs {
		if strings.Contains(l, substr) {
			n++
		}
	}
	return n
}

const vm350 = "net1: virtio=BC:24:11:00:02:99,bridge=vmbr_imds\n"

func TestStartAppliesTheTable(t *testing.T) {
	r := newRig(t, map[string]string{"350.conf": vm350})
	if !r.b.Tick() {
		t.Fatal("the first check did not apply")
	}
	if !strings.Contains(r.nft.table, `iifname "tap350i1" ether saddr != bc:24:11:00:02:99`) {
		t.Fatalf("applied table lacks VM 350:\n%s", r.nft.table)
	}
	if r.count("Applied table bridge vtpm_mds_bind: 1 guest ports on vmbr_imds") != 1 {
		t.Fatalf("logs: %q", r.logs)
	}
}

func TestNothingChangesNothingIsAppliedOrLogged(t *testing.T) {
	r := newRig(t, map[string]string{"350.conf": vm350})
	r.b.Tick()
	r.logs = nil
	for i := 0; i < 3; i++ {
		if r.b.Tick() {
			t.Fatalf("check %d applied an unchanged table", i)
		}
	}
	if r.nft.applies != 1 || len(r.logs) != 0 {
		t.Fatalf("applies %d, logs %q", r.nft.applies, r.logs)
	}
}

func TestAChangedConfigIsApplied(t *testing.T) {
	r := newRig(t, map[string]string{"350.conf": vm350})
	r.b.Tick()
	r.write("400.conf", "net0: virtio=52:54:00:00:04:00,bridge=vmbr_imds\n")
	if !r.b.Tick() || !strings.Contains(r.nft.table, `"tap400i0"`) {
		t.Fatalf("an added guest was not applied:\n%s", r.nft.table)
	}
	r.write("350.conf", "net1: virtio=BC:24:11:00:02:9A,bridge=vmbr_imds\n")
	if !r.b.Tick() || !strings.Contains(r.nft.table, "bc:24:11:00:02:9a") {
		t.Fatalf("a changed MAC was not applied:\n%s", r.nft.table)
	}
}

func TestAChangeThatBindsNothingNewIsNotApplied(t *testing.T) {
	r := newRig(t, map[string]string{"350.conf": vm350})
	r.b.Tick()
	r.write("350.conf", "name: renamed\n"+vm350+"net0: virtio=52:54:00:00:00:01,bridge=vmbr0\n")
	if r.b.Tick() || r.nft.applies != 1 {
		t.Fatalf("a change outside the IMDS bridge re-applied (applies %d)", r.nft.applies)
	}
}

func TestAMissingOrAlteredTableIsRestoredAndLoggedOnce(t *testing.T) {
	r := newRig(t, map[string]string{"350.conf": vm350})
	r.b.Tick()
	want := r.nft.table

	r.nft.exists = false // systemctl restart nftables without the block
	if !r.b.Tick() || r.nft.table != want {
		t.Fatal("a missing table was not restored")
	}
	r.nft.table = "table bridge vtpm_mds_bind { only the hand-written VM 350 block }"
	if !r.b.Tick() || r.nft.table != want {
		t.Fatal("an altered table was not restored")
	}
	r.b.Tick()
	if r.count("is missing") != 1 || r.count("changed outside") != 1 {
		t.Fatalf("logs: %q", r.logs)
	}
}

func TestAFailedApplyIsRetriedEveryTickAndLoggedOnce(t *testing.T) {
	r := newRig(t, map[string]string{"350.conf": vm350})
	r.nft.failing = true
	for i := 0; i < 4; i++ {
		if r.b.Tick() {
			t.Fatal("a failing apply reported success")
		}
	}
	if r.nft.attempts != 4 {
		t.Fatalf("apply attempted %d times over 4 ticks, want 4", r.nft.attempts)
	}
	if n := r.count("Failed to apply"); n != 1 {
		t.Fatalf("failure logged %d times: %q", n, r.logs)
	}
	if r.count("is missing") != 0 {
		t.Fatalf("a table that never existed was reported missing: %q", r.logs)
	}
	r.nft.failErr = "netlink: Device or resource busy"
	r.b.Tick()
	if n := r.count("Failed to apply"); n != 2 {
		t.Fatalf("a different error was not logged: %q", r.logs)
	}
	r.nft.failing = false
	if !r.b.Tick() || r.count("Applied") != 1 {
		t.Fatalf("not applied on the next tick after the error went away: %q", r.logs)
	}
	r.b.Tick()
	if r.nft.applies != 1 {
		t.Fatalf("applied %d times after recovery, want 1", r.nft.applies)
	}
}

func TestRunFailsWhenTheFirstApplyFails(t *testing.T) {
	r := newRig(t, map[string]string{"350.conf": vm350})
	r.nft.failing = true
	err := r.b.Run(context.Background(), time.Hour)
	if err == nil || !strings.Contains(err.Error(), "Operation not permitted") {
		t.Fatalf("Run = %v, want the apply error", err)
	}
}

func TestRunFailsWhenTheFirstReadFails(t *testing.T) {
	r := newRig(t, map[string]string{"350.conf": vm350})
	if err := os.Remove(filepath.Join(r.root, "local")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := r.b.Run(ctx, time.Hour)
	if err == nil || !strings.Contains(err.Error(), "cannot read guest NICs") {
		t.Fatalf("Run = %v, want the read error", err)
	}
	if r.nft.applies != 0 {
		t.Fatalf("applied %d times without configurations", r.nft.applies)
	}
}

func TestUnreadableConfigsKeepTheTableAndAreLoggedOnce(t *testing.T) {
	r := newRig(t, map[string]string{"350.conf": vm350})
	r.b.Tick()
	want := r.nft.table
	if err := os.Remove(filepath.Join(r.root, "local")); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if r.b.Tick() {
			t.Fatal("applied without configurations")
		}
	}
	if r.nft.table != want || r.count("Failed to read guest NICs") != 1 {
		t.Fatalf("table changed or logs wrong: %q", r.logs)
	}
	r.nft.exists = false
	if !r.b.Tick() || r.nft.table != want {
		t.Fatal("the last good table was not restored while the configurations are unreadable")
	}
}

func TestRunDirReceivesTheScript(t *testing.T) {
	r := newRig(t, map[string]string{"350.conf": vm350})
	r.b.RunDir = t.TempDir()
	r.b.Tick()
	got, err := os.ReadFile(filepath.Join(r.b.RunDir, "vtpm_mds_bind.nft"))
	if err != nil || string(got) != r.nft.table {
		t.Fatalf("run dir script %q, err %v", got, err)
	}
}

func TestRunChecksAtOnceAndUntilCancelled(t *testing.T) {
	r := newRig(t, map[string]string{"350.conf": vm350})
	applied := make(chan struct{})
	r.b.Logf = func(f string, a ...any) {
		if strings.HasPrefix(f, "Applied") {
			close(applied)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	// An interval far beyond the test: only the check at start can apply.
	go func() { _ = r.b.Run(ctx, time.Hour); close(done) }()
	select {
	case <-applied:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not check at start")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}
