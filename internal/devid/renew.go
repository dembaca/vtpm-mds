package devid

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// RenewDue reports whether the guest should enroll again for cert at now.
// Missing cert is handled by the caller (nil → renew). Renew when expired or
// remaining lifetime is strictly less than one third of the cert's validity.
func RenewDue(cert *x509.Certificate, now time.Time) bool {
	if cert == nil {
		return true
	}
	if !cert.NotAfter.After(now) {
		return true
	}
	total := cert.NotAfter.Sub(cert.NotBefore)
	if total <= 0 {
		return true
	}
	remaining := cert.NotAfter.Sub(now)
	return remaining < total/3
}

// LoadCertPEMFile parses the first CERTIFICATE PEM from path.
// Returns (nil, nil) when the file does not exist.
func LoadCertPEMFile(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("%s: no certificate PEM", path)
	}
	return x509.ParseCertificate(block.Bytes)
}

// ClearSPIREAgentSVIDCache removes the agent SVID cache under dataDir so the
// next agent start performs node attestation. Unknown paths are ignored.
// dataDir empty is a no-op.
func ClearSPIREAgentSVIDCache(dataDir string) error {
	if dataDir == "" {
		return nil
	}
	// SPIRE agent persists the attested SVID in agent-data.json (and related
	// key material). Removing the JSON is enough for a re-attest on start;
	// also drop common sibling key files if present.
	names := []string{
		"agent-data.json",
		"svid.key",
		"svid.pem",
		"bundle.crt",
		"agent-key.pem",
	}
	for _, name := range names {
		path := filepath.Join(dataDir, name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("clear SPIRE SVID cache %s: %w", path, err)
		}
	}
	return nil
}

// TryRestartSPIREAgent runs systemctl try-restart spire-agent.service.
// Missing systemctl or unit must not fail the renew.
func TryRestartSPIREAgent() {
	cmd := exec.Command("systemctl", "try-restart", "spire-agent.service")
	_ = cmd.Run()
}
