package devid

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"

	"github.com/google/go-tpm/legacy/tpm2"
)

func TestEnrollAgainstSwtpm(t *testing.T) {
	sock := "/var/lib/mds-lab/run/swtpm.sock"
	if _, err := os.Stat(sock); err != nil {
		t.Skip(err)
	}
	rwc, err := tpm2.OpenTPM(sock)
	if err != nil {
		// Lab swtpm may expose only a QEMU ctrl socket, or the cmd socket may be stale.
		t.Skip(err)
	}
	defer rwc.Close()

	sr, res, err := CreateSigningRequest(rwc, DefaultKeyFactory(), "guest100")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Flush()

	if err := VerifyDevIDResidency(sr.AttestationKey, sr.DevIDKey, sr.CertifyData, sr.CertifySignature); err != nil {
		t.Fatalf("residency: %v", err)
	}

	ekPEM, err := os.ReadFile("/etc/vtpm-mds/ek-chain.pem")
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ekPEM) {
		t.Fatal("ek roots")
	}
	if err := VerifyEKCertificateBound(roots, sr.EndorsementKey, sr.EndorsementCertificate); err != nil {
		t.Fatalf("ek cert: %v", err)
	}

	ca, err := LoadCA("/etc/vtpm-mds/devid-ca.pem", "/etc/vtpm-mds/devid-ca-key.pem")
	if err != nil {
		t.Fatal(err)
	}
	enroller := NewEnroller(ca, roots, nil)
	data, err := sr.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	sig, err := HashAndSign(rwc, tpm2.HandleOwner, res.DevID.Handle, data)
	if err != nil {
		t.Fatal(err)
	}
	start, err := enroller.Start(data, sig, "100", EKFingerprint(sr.EndorsementCertificate))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	secret, err := res.Activate(start.CredentialBlob, start.Secret)
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	certPEM, err := enroller.Finish(start.SessionID, secret, "100", EKFingerprint(sr.EndorsementCertificate))
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	if len(certPEM) == 0 {
		t.Fatal("empty devid cert")
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("expected PEM certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if cert.Subject.CommonName != "100" {
		t.Fatalf("CN=%q want 100 (VM ID), not guest platform name", cert.Subject.CommonName)
	}
	t.Logf("DevID cert issued (%d bytes PEM) subject=%s", len(certPEM), cert.Subject.String())
}
