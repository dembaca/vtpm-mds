package server

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dembaca/vtpm-mds/internal/config"
)

// writeTestCA writes a self-signed CA certificate and its PKCS#8 key into dir.
func writeTestCA(t *testing.T, dir string) (certPath, keyPEM string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test DevID CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPath = filepath.Join(dir, "ca.crt")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return certPath, string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}))
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return &buf
}

func TestCredentialReplacesConfiguredKeyPath(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPEM := writeTestCA(t, dir)
	credDir := filepath.Join(dir, "creds")
	if err := os.Mkdir(credDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(credDir, credentialCAKeyName), []byte(keyPEM), 0o400); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CREDENTIALS_DIRECTORY", credDir)
	logs := captureLog(t)

	cfg := &config.Config{}
	cfg.MDS.DevIDCACert = certPath
	cfg.MDS.DevIDCAKey = filepath.Join(dir, "no-such-dir", "unreachable.key")
	enroller, err := loadDevIDEnroller(cfg)
	if err != nil {
		t.Fatalf("loadDevIDEnroller with a credential: %v", err)
	}
	if enroller == nil {
		t.Fatal("no enroller although the credential holds a valid key")
	}
	if !strings.Contains(logs.String(), "DevID CA key from systemd credential") {
		t.Errorf("log does not say the key came from the credential: %q", logs.String())
	}
}

func TestCredentialAcceptsEmptyConfiguredKey(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPEM := writeTestCA(t, dir)
	if err := os.WriteFile(filepath.Join(dir, credentialCAKeyName), []byte(keyPEM), 0o400); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CREDENTIALS_DIRECTORY", dir)
	captureLog(t)

	cfg := &config.Config{}
	cfg.MDS.DevIDCACert = certPath
	enroller, err := loadDevIDEnroller(cfg)
	if err != nil || enroller == nil {
		t.Fatalf("empty devid_ca_key with a credential: enroller=%v err=%v", enroller, err)
	}
}

func TestNoCredentialKeepsConfiguredKeyPath(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, dir string)
	}{
		{"no CREDENTIALS_DIRECTORY", func(t *testing.T, dir string) { t.Setenv("CREDENTIALS_DIRECTORY", "") }},
		{"directory without the credential", func(t *testing.T, dir string) { t.Setenv("CREDENTIALS_DIRECTORY", dir) }},
		{"empty credential", func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, credentialCAKeyName), nil, 0o400); err != nil {
				t.Fatal(err)
			}
			t.Setenv("CREDENTIALS_DIRECTORY", dir)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			certPath, keyPEM := writeTestCA(t, dir)
			keyPath := filepath.Join(dir, "ca.key")
			if err := os.WriteFile(keyPath, []byte(keyPEM), 0o600); err != nil {
				t.Fatal(err)
			}
			credDir := filepath.Join(dir, "creds")
			if err := os.Mkdir(credDir, 0o700); err != nil {
				t.Fatal(err)
			}
			tc.setup(t, credDir)
			logs := captureLog(t)

			cfg := &config.Config{}
			cfg.MDS.DevIDCACert = certPath
			cfg.MDS.DevIDCAKey = keyPath
			enroller, err := loadDevIDEnroller(cfg)
			if err != nil || enroller == nil {
				t.Fatalf("configured key: enroller=%v err=%v", enroller, err)
			}
			if strings.Contains(logs.String(), "systemd credential") {
				t.Errorf("claims a credential although none is usable: %q", logs.String())
			}
		})
	}
}
