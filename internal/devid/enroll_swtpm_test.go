package devid

import (
	"crypto/x509"
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

	ekPEM, err := os.ReadFile("/etc/prox-mds/ek-chain.pem")
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ekPEM) {
		t.Fatal("ek roots")
	}
	if err := VerifyEKCertificate(roots, sr.EndorsementKey, sr.EndorsementCertificate); err != nil {
		t.Fatalf("ek cert: %v", err)
	}

	ca, err := LoadCA("/etc/prox-mds/devid-ca.pem", "/etc/prox-mds/devid-ca-key.pem")
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
	start, err := enroller.Start(data, sig, "guest100", "100", EKFingerprint(sr.EndorsementCertificate))
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
	t.Logf("DevID cert issued (%d bytes PEM)", len(certPEM))
}
