package devid

import (
	"crypto/x509"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"
)

func TestRenewDue(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	total := 90 * 24 * time.Hour

	fresh := &x509.Certificate{
		NotBefore: now.Add(-10 * 24 * time.Hour),
		NotAfter:  now.Add(-10*24*time.Hour).Add(total),
	}
	if RenewDue(fresh, now) {
		t.Fatal("fresh cert (remaining > 1/3) should not renew")
	}

	// remaining == total/3 → not strictly less → no renew
	notBefore := now.Add(-60 * 24 * time.Hour)
	notAfter := notBefore.Add(total)
	boundary := &x509.Certificate{NotBefore: notBefore, NotAfter: notAfter}
	if remaining := notAfter.Sub(now); remaining != total/3 {
		t.Fatalf("test setup: remaining=%v want %v", remaining, total/3)
	}
	if RenewDue(boundary, now) {
		t.Fatal("remaining == 1/3 must not renew")
	}

	nb := now.Add(-61 * 24 * time.Hour)
	na := nb.Add(total)
	lastThird := &x509.Certificate{NotBefore: nb, NotAfter: na}
	if !RenewDue(lastThird, now) {
		t.Fatalf("last third should renew; remaining=%v total/3=%v", na.Sub(now), total/3)
	}

	expired := &x509.Certificate{
		NotBefore: now.Add(-total),
		NotAfter:  now.Add(-time.Hour),
	}
	if !RenewDue(expired, now) {
		t.Fatal("expired cert should renew")
	}

	if !RenewDue(nil, now) {
		t.Fatal("nil cert should renew")
	}
}

func TestLoadCertPEMFileCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "devid.crt.pem")
	if err := os.WriteFile(path, []byte("not-a-cert"), 0o600); err != nil {
		t.Fatal(err)
	}
	cert, err := LoadCertPEMFile(path)
	if err == nil || cert != nil {
		t.Fatalf("want error for corrupt PEM, got cert=%v err=%v", cert, err)
	}
	if _, err := LoadCertPEMFile(filepath.Join(dir, "missing.pem")); err != nil {
		t.Fatalf("missing file: %v", err)
	}
}

func TestClearSPIREAgentSVIDCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-data.json")
	bundle := filepath.Join(dir, "bundle.crt")
	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bundle, []byte("BUNDLE"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ClearSPIREAgentSVIDCache(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("agent-data.json still present: %v", err)
	}
	if _, err := os.Stat(bundle); err != nil {
		t.Fatalf("bundle.crt must not be removed: %v", err)
	}
	if err := ClearSPIREAgentSVIDCache(""); err != nil {
		t.Fatalf("empty dataDir: %v", err)
	}
	if err := ClearSPIREAgentSVIDCache(dir); err != nil {
		t.Fatalf("idempotent clear: %v", err)
	}
}

func TestTryRestartSPIREAgentDoesNotWait(t *testing.T) {
	want := []string{"systemctl", "--no-block", "try-restart", "spire-agent.service"}
	if !reflect.DeepEqual(restartSPIREAgentArgv, want) {
		t.Fatalf("restart argv = %q, want %q", restartSPIREAgentArgv, want)
	}
}

// stubOwner makes keepOwner believe it runs as euid and records the owner it
// sets instead of calling chown, which only root may do.
func stubOwner(t *testing.T, euid int) *[][2]int {
	t.Helper()
	var got [][2]int
	oldEuid, oldChown := geteuid, chownFile
	geteuid = func() int { return euid }
	chownFile = func(_ *os.File, uid, gid int) error {
		got = append(got, [2]int{uid, gid})
		return nil
	}
	t.Cleanup(func() { geteuid, chownFile = oldEuid, oldChown })
	return &got
}

func ownerOf(t *testing.T, path string) [2]int {
	t.Helper()
	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	st := fi.Sys().(*syscall.Stat_t)
	return [2]int{int(st.Uid), int(st.Gid)}
}

func TestWriteFileAtomicKeepsOwnerAsRoot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "devid.crt.pem")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := stubOwner(t, 0)
	if err := writeFileAtomic(path, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if want := ownerOf(t, dir); len(*got) != 1 || (*got)[0] != want {
		t.Fatalf("chown calls = %v, want one with the replaced file's owner %v", *got, want)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "new" {
		t.Fatalf("content = %q, %v", data, err)
	}

	// A new file takes the directory's owner.
	*got = nil
	if err := writeFileAtomic(filepath.Join(dir, "devid.pub.blob"), []byte("pub"), 0o600); err != nil {
		t.Fatal(err)
	}
	if want := ownerOf(t, dir); len(*got) != 1 || (*got)[0] != want {
		t.Fatalf("chown calls = %v, want one with the directory's owner %v", *got, want)
	}
}

func TestWriteFileAtomicLeavesOwnerWhenNotRoot(t *testing.T) {
	dir := t.TempDir()
	got := stubOwner(t, 1000)
	if err := writeFileAtomic(filepath.Join(dir, "devid.crt.pem"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 0 {
		t.Fatalf("chown called without root: %v", *got)
	}
}

func TestWriteFileAtomicFailsWhenOwnerCannotBeKept(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "devid.crt.pem")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	stubOwner(t, 0)
	chownFile = func(*os.File, int, int) error { return syscall.EPERM }
	if err := writeFileAtomic(path, []byte("new"), 0o600); err == nil {
		t.Fatal("want an error when chown fails as root")
	}
	if data, _ := os.ReadFile(path); string(data) != "old" {
		t.Fatalf("file replaced despite the failed chown: %q", data)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".devid.crt.pem.tmp-*")); len(left) != 0 {
		t.Fatalf("temporary files left: %v", left)
	}
}
