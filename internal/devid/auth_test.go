package devid

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dembaca/vtpm-mds/internal/inventory"
	"github.com/google/go-tpm/legacy/tpm2"
)

func testEKCert(t *testing.T) (*x509.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-ek"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	return cert, roots
}

func TestParseEKCertHeader(t *testing.T) {
	cert, _ := testEKCert(t)
	hdr := EncodeEKCertHeader(cert)
	got, err := ParseEKCertHeader(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if EKFingerprint(got) != EKFingerprint(cert) {
		t.Fatal("fingerprint mismatch")
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	got2, err := ParseEKCertHeader(EncodeEKCertHeader(&x509.Certificate{Raw: pemBytes}))
	if err != nil {
		t.Fatal(err)
	}
	if !certsDEREqual(got2, cert) {
		t.Fatal("PEM round-trip failed")
	}
}

func TestAuthenticateEnrollCaller(t *testing.T) {
	cert, key, roots := testEKMaterial(t)
	other, _, _ := testEKMaterial(t)
	enroller := NewEnroller(&CA{}, roots, nil)
	ekPub := rsaKeyToEKPub(key)

	req := httptest.NewRequest(http.MethodPost, "/latest/devid/enroll/start", nil)
	req.Header.Set(HeaderEKCert, EncodeEKCertHeader(cert))

	if _, _, err := enroller.AuthenticateEnrollCaller(req, cert, ekPub, ""); !isUnauthorized(err) {
		t.Fatalf("expected unauthorized without VM, got %v", err)
	}

	vm := &inventory.VMConfig{VMID: "100", MACs: []string{"52:54:00:a1:b2:c3"}}
	req = req.WithContext(context.WithValue(req.Context(), inventory.VMConfigContextKey, vm))

	// Start-style: chain + bind.
	vmid, ek, err := enroller.AuthenticateEnrollCaller(req, cert, ekPub, "")
	if err != nil || vmid != "100" || ek == nil {
		t.Fatalf("auth failed: vmid=%q err=%v", vmid, err)
	}

	// Finish-style call: chain only (no signing-request EK).
	if _, _, err := enroller.AuthenticateEnrollCaller(req, nil, nil, ""); err != nil {
		t.Fatalf("finish-style auth: %v", err)
	}

	if _, _, err := enroller.AuthenticateEnrollCaller(req, other, ekPub, ""); !isUnauthorized(err) {
		t.Fatalf("expected CSR mismatch unauthorized, got %v", err)
	}

	vm.EKSHA256 = EKFingerprint(other)
	if _, _, err := enroller.AuthenticateEnrollCaller(req, cert, ekPub, ""); !isUnauthorized(err) {
		t.Fatalf("expected pin mismatch, got %v", err)
	}
	vm.EKSHA256 = EKFingerprint(cert)
	if _, _, err := enroller.AuthenticateEnrollCaller(req, cert, ekPub, ""); err != nil {
		t.Fatal(err)
	}
}

func TestFinishRejectsIdentityMismatch(t *testing.T) {
	caCert, caKey := mustTestCA(t)
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw})
	keyDER, err := x509.MarshalPKCS8PrivateKey(caKey)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	ca, err := ParseCA(caPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	enroller := NewEnroller(ca, x509.NewCertPool(), NewSessionStore(time.Minute))

	devidKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pub := rsaPublicToTPM(devidKey, false)
	pub.Attributes = DevIDAttributes
	pub.RSAParameters.Sign = &tpm2.SigScheme{Alg: tpm2.AlgRSASSA, Hash: tpm2.AlgSHA256}
	pub.RSAParameters.Symmetric = nil
	sr := SigningRequest{DevIDKey: &pub}

	sid, err := enroller.Sessions.Put([]byte("nonce"), sr, "100", "fp-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := enroller.Finish(sid, []byte("nonce"), "999", "fp-a"); err == nil {
		t.Fatal("expected VM mismatch")
	}

	sid, err = enroller.Sessions.Put([]byte("nonce"), sr, "100", "fp-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := enroller.Finish(sid, []byte("nonce"), "100", "fp-b"); err == nil {
		t.Fatal("expected EK mismatch")
	}
}
