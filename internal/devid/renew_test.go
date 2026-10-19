package devid

import (
	"crypto/x509"
	"os"
	"path/filepath"
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
}
