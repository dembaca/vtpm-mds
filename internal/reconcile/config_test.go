package reconcile

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writePEM writes cert as a PEM file and returns its path.
func writePEM(t *testing.T, dir, name string, der []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// makePKI writes a self-signed root and two intermediates signed by it.
func makePKI(t *testing.T, dir string) (root, int1, int2 string, int1DER []byte) {
	t.Helper()
	rkey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := func(n int64, cn string, ca bool) *x509.Certificate {
		return &x509.Certificate{
			SerialNumber: big.NewInt(n), Subject: pkix.Name{CommonName: cn},
			NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
			IsCA: ca, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
		}
	}
	rt := tmpl(1, "root", true)
	rder, err := x509.CreateCertificate(rand.Reader, rt, rt, &rkey.PublicKey, rkey)
	if err != nil {
		t.Fatal(err)
	}
	rcert, _ := x509.ParseCertificate(rder)
	mk := func(n int64, cn string) []byte {
		k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		der, err := x509.CreateCertificate(rand.Reader, tmpl(n, cn, true), rcert, &k.PublicKey, rkey)
		if err != nil {
			t.Fatal(err)
		}
		return der
	}
	d1, d2 := mk(2, "int1"), mk(3, "int2")
	return writePEM(t, dir, "root.pem", rder), writePEM(t, dir, "int1.pem", d1), writePEM(t, dir, "int2.pem", d2), d1
}

func writeConfig(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	root, int1, int2, der := makePKI(t, dir)
	head := "trust_domain: example.org\nspire_socket: /run/s.sock\n"
	cl := func(name, url string, ints ...string) string {
		s := "  - name: " + name + "\n    api_url: " + url + "\n    token_file: /t\n    intermediates:\n"
		for _, i := range ints {
			s += "      - " + i + "\n"
		}
		return s
	}
	sum := sha1.Sum(der)
	tests := []struct {
		name    string
		body    string
		wantErr string // "" means success
	}{
		{"a good config", head + "clusters:\n" + cl("cluster-1", "https://pve:8006", int1, int1), ""},
		{"an unknown key", head + "bogus: 1\nclusters:\n" + cl("c", "https://pve", int1), "bogus"},
		{"an interval below the minimum", head + "interval: 5s\nclusters:\n" + cl("c", "https://pve", int1), "interval"},
		{"missing intermediates", head + "clusters:\n" + cl("c", "https://pve"), "no intermediates"},
		{"a root configured as intermediate", head + "clusters:\n" + cl("c", "https://pve", root), root},
		{"a fingerprint shared by two clusters", head + "clusters:\n" + cl("a", "https://pve", int1) + cl("b", "https://pve", int1), "clusters a and b"},
		{"duplicate cluster names", head + "clusters:\n" + cl("a", "https://pve", int1) + cl("a", "https://pve", int2), "duplicate"},
		{"a capital in the cluster name", head + "clusters:\n" + cl("Cluster_1", "https://pve", int1), "Cluster_1"},
		{"a dot-dot cluster name", head + "clusters:\n" + cl("..", "https://pve", int1), "cluster name"},
		{"an http api_url", head + "clusters:\n" + cl("c", "http://pve", int1), "https://"},
		{"a relative spire_socket", "trust_domain: example.org\nspire_socket: s.sock\nclusters:\n" + cl("c", "https://pve", int1), "must be an absolute path"},
		{"no clusters", head + "clusters: []\n", "no clusters"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := LoadConfig(writeConfig(t, dir, tt.body))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Interval != 60*time.Second {
				t.Errorf("interval = %s, want 60s", cfg.Interval)
			}
			fps := cfg.Clusters[0].Fingerprints
			if len(fps) != 1 || fps[0] != hex.EncodeToString(sum[:]) {
				t.Errorf("fingerprints = %v, want [%x]", fps, sum)
			}
		})
	}
}

func TestLoadConfigDedupesByFingerprint(t *testing.T) {
	dir := t.TempDir()
	_, int1, int2, der := makePKI(t, dir)
	copyOfInt1 := writePEM(t, dir, "int1-copy.pem", der)
	body := "trust_domain: example.org\nspire_socket: /run/s.sock\nclusters:\n  - name: c\n    api_url: https://pve\n    token_file: /t\n    intermediates:\n" +
		"      - " + int1 + "\n      - " + copyOfInt1 + "\n      - " + int2 + "\n"
	cfg, err := LoadConfig(writeConfig(t, dir, body))
	if err != nil {
		t.Fatal(err)
	}
	c := cfg.Clusters[0]
	if len(c.Fingerprints) != 2 || c.Fingerprints[0] == c.Fingerprints[1] || len(c.Intermediates) != 2 {
		t.Errorf("same cert under two file names: fingerprints %v, intermediates %v", c.Fingerprints, c.Intermediates)
	}
}

func TestClusterClient(t *testing.T) {
	dir := t.TempDir()
	t.Run("an empty token file", func(t *testing.T) {
		p := filepath.Join(dir, "empty")
		os.WriteFile(p, []byte(" \n"), 0o600)
		_, err := ClusterConfig{Name: "c", TokenFile: p}.Client()
		if err == nil || !strings.Contains(err.Error(), p) {
			t.Fatalf("err = %v, want one naming %s", err, p)
		}
	})
	t.Run("a token is trimmed", func(t *testing.T) {
		p := filepath.Join(dir, "tok")
		os.WriteFile(p, []byte("user@pve!id=sekrit\n"), 0o600)
		c, err := ClusterConfig{Name: "c", APIURL: "https://pve", TokenFile: p}.Client()
		if err != nil {
			t.Fatal(err)
		}
		if c.Token != "user@pve!id=sekrit" {
			t.Errorf("token = %q", c.Token)
		}
	})
}
