package devid

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
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
// An unreadable or corrupt certificate returns (nil, err) so the caller can
// treat it as renew-due after logging.
func LoadCertPEMFile(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("%s: empty certificate file", path)
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("%s: no certificate PEM", path)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return cert, nil
}

// ClearSPIREAgentSVIDCache removes the SPIRE 1.14 agent SVID cache under
// dataDir so the next agent start performs node attestation. Only
// agent-data.json and the legacy agent_svid.der are removed — never the
// bootstrap trust bundle. dataDir empty is a no-op.
func ClearSPIREAgentSVIDCache(dataDir string) error {
	if dataDir == "" {
		return nil
	}
	names := []string{
		"agent-data.json",
		"agent_svid.der",
	}
	for _, name := range names {
		path := filepath.Join(dataDir, name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("clear SPIRE SVID cache %s: %w", path, err)
		}
	}
	return nil
}

// restartSPIREAgentArgv is the command TryRestartSPIREAgent runs; a seam for tests.
var restartSPIREAgentArgv = []string{"systemctl", "--no-block", "try-restart", "spire-agent.service"}

// TryRestartSPIREAgent queues systemctl try-restart spire-agent.service without
// waiting for it: the packaged oneshot is ordered Before=spire-agent.service,
// so a blocking restart from inside it waits on itself until the start timeout
// kills it (make-guest-renewal-work-with-a-non-root-spire-agent D-b).
// Missing systemctl or unit must not fail the renew.
func TryRestartSPIREAgent() {
	cmd := exec.Command(restartSPIREAgentArgv[0], restartSPIREAgentArgv[1:]...)
	_ = cmd.Run()
}

// LogRenewReason records once per renewal why the materials in dir are
// renewed: "no certificate", "unreadable certificate", "due", "certificate is
// for another VM", "missing blob", "certificate does not match key" or "key
// does not load in this TPM" (renew-when-the-devid-key-is-not-this-tpms D-d).
// detail, when set, is the error behind the reason.
func LogRenewReason(dir, reason, detail string) {
	if detail != "" {
		log.Printf("devid-enroll: renewing %s: %s: %s", dir, reason, detail)
		return
	}
	log.Printf("devid-enroll: renewing %s: %s", dir, reason)
}
