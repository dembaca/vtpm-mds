package devid

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"log"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dembaca/vtpm-mds/internal/inventory"
	"github.com/google/go-tpm/legacy/tpm2"
)

const ekMismatchBody = "EK certificate does not match the endorsement key in the signing request"

// testEKMaterial returns a self-signed EK certificate, its RSA private key,
// and a root pool that trusts it.
func testEKMaterial(t *testing.T) (*x509.Certificate, *rsa.PrivateKey, *x509.CertPool) {
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
	return cert, key, roots
}

func mustTestDevIDCA(t *testing.T) *CA {
	t.Helper()
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
	return ca
}

// fakeSignedEnrollRequest builds a signing request that passes residency,
// attribute and signature checks. ekCert is placed in the request; ekPub is
// the endorsement key public area (may intentionally disagree with ekCert).
func fakeSignedEnrollRequest(t *testing.T, ekCert *x509.Certificate, ekPub *tpm2.Public) (requestData, signature []byte, sr SigningRequest) {
	t.Helper()
	akPriv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	devPriv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	akTPM := rsaPublicToTPM(akPriv, false)
	akTPM.Attributes = AKAttributes
	akTPM.RSAParameters.Sign = &tpm2.SigScheme{Alg: tpm2.AlgRSASSA, Hash: tpm2.AlgSHA256}
	akTPM.RSAParameters.Symmetric = nil

	devTPM := rsaPublicToTPM(devPriv, false)
	devTPM.Attributes = DevIDAttributes
	devTPM.RSAParameters.Sign = &tpm2.SigScheme{Alg: tpm2.AlgRSASSA, Hash: tpm2.AlgSHA256}
	devTPM.RSAParameters.Symmetric = nil

	devName, err := devTPM.Name()
	if err != nil {
		t.Fatal(err)
	}
	ad := tpm2.AttestationData{
		Magic: 0xff544347,
		Type:  tpm2.TagAttestCertify,
		AttestedCertifyInfo: &tpm2.CertifyInfo{
			Name:          devName,
			QualifiedName: devName,
		},
	}
	certifyData, err := ad.Encode()
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(certifyData)
	certifySig, err := rsa.SignPKCS1v15(rand.Reader, akPriv, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}

	sr = SigningRequest{
		PlatformIdentity:       pkix.Name{CommonName: "100"}.ToRDNSequence(),
		EndorsementCertificate: ekCert,
		EndorsementKey:         ekPub,
		AttestationKey:         &akTPM,
		DevIDKey:               &devTPM,
		CertifyData:            certifyData,
		CertifySignature:       certifySig,
	}
	requestData, err = sr.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	reqSum := sha256.Sum256(requestData)
	signature, err = rsa.SignPKCS1v15(rand.Reader, devPriv, crypto.SHA256, reqSum[:])
	if err != nil {
		t.Fatal(err)
	}
	return requestData, signature, sr
}

func rsaKeyToEKPub(key *rsa.PrivateKey) *tpm2.Public {
	pub := rsaPublicToTPM(key, true)
	return &pub
}

func postEnrollStart(t *testing.T, enroller *Enroller, ekCert *x509.Certificate, requestData, signature []byte) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(EnrollStartRequest{
		RequestB64:   base64.StdEncoding.EncodeToString(requestData),
		SignatureB64: base64.StdEncoding.EncodeToString(signature),
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/latest/devid/enroll/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderEKCert, EncodeEKCertHeader(ekCert))
	vm := &inventory.VMConfig{VMID: "100", MACs: []string{"52:54:00:a1:b2:c3"}}
	req = req.WithContext(context.WithValue(req.Context(), inventory.VMConfigContextKey, vm))

	rr := httptest.NewRecorder()
	enroller.HandleStart(rr, req)
	return rr
}

func TestEnrollStartRejectsMismatchedEK(t *testing.T) {
	ekCert, _, roots := testEKMaterial(t)
	attackerEK, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	requestData, sig, _ := fakeSignedEnrollRequest(t, ekCert, rsaKeyToEKPub(attackerEK))
	store := NewSessionStore(time.Minute)
	enroller := NewEnroller(mustTestDevIDCA(t), roots, store)

	var logs string
	var rr *httptest.ResponseRecorder
	logs = captureLog(t, func() {
		rr = postEnrollStart(t, enroller, ekCert, requestData, sig)
	})

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("mismatched EK: expected 401, got %d body=%q", rr.Code, rr.Body.String())
	}
	gotBody := strings.TrimSpace(rr.Body.String())
	if gotBody != ekMismatchBody {
		t.Fatalf("body=%q want %q", gotBody, ekMismatchBody)
	}
	if !strings.Contains(logs, "devid enroll auth:") {
		t.Fatalf("expected auth log prefix, got %q", logs)
	}
	if !strings.Contains(logs, ekMismatchBody) {
		t.Fatalf("expected mismatch reason in log, got %q", logs)
	}
	store.mu.Lock()
	n := len(store.sessions)
	store.mu.Unlock()
	if n != 0 {
		t.Fatalf("session store not empty after rejected start: %d sessions", n)
	}
}

func TestEnrollStartAcceptsMatchingEK(t *testing.T) {
	ekCert, ekKey, roots := testEKMaterial(t)
	requestData, sig, _ := fakeSignedEnrollRequest(t, ekCert, rsaKeyToEKPub(ekKey))
	enroller := NewEnroller(mustTestDevIDCA(t), roots, NewSessionStore(time.Minute))

	rr := postEnrollStart(t, enroller, ekCert, requestData, sig)
	if rr.Code != http.StatusOK {
		t.Fatalf("matching EK: expected 200, got %d body=%q", rr.Code, rr.Body.String())
	}
}

func TestEnrollStartRejectsMissingEndorsementKey(t *testing.T) {
	ekCert, _, roots := testEKMaterial(t)
	requestData, sig, _ := fakeSignedEnrollRequest(t, ekCert, nil)
	enroller := NewEnroller(mustTestDevIDCA(t), roots, NewSessionStore(time.Minute))

	rr := postEnrollStart(t, enroller, ekCert, requestData, sig)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("missing EK public: expected 401, got %d body=%q", rr.Code, rr.Body.String())
	}
	gotBody := strings.TrimSpace(rr.Body.String())
	if gotBody != ekMismatchBody {
		t.Fatalf("body=%q want %q", gotBody, ekMismatchBody)
	}
	if strings.Contains(gotBody, "create challenge: missing EK public") {
		t.Fatal("should not reach challenge creation")
	}
}

func TestVerifyEKCertificateBoundKeys(t *testing.T) {
	cert, key, roots := testEKMaterial(t)
	if err := VerifyEKCertificateBound(roots, rsaKeyToEKPub(key), cert); err != nil {
		t.Fatalf("matching key: %v", err)
	}

	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyEKCertificateBound(roots, rsaKeyToEKPub(other), cert); !errors.Is(err, ErrEKCertificateKeyMismatch) {
		t.Fatalf("different key: got %v", err)
	}
	if err := VerifyEKCertificateBound(roots, nil, cert); !errors.Is(err, ErrEKCertificateKeyMismatch) {
		t.Fatalf("nil key: got %v", err)
	}
}

func TestEnrollFinishStillChainOnlyAndSessionBound(t *testing.T) {
	ekCertA, _, rootsA := testEKMaterial(t)
	ekCertB, _, rootsB := testEKMaterial(t)
	// Trust both certificates so finish's chain-only check accepts B.
	roots := x509.NewCertPool()
	roots.AddCert(ekCertA)
	roots.AddCert(ekCertB)
	_ = rootsA
	_ = rootsB

	enroller := NewEnroller(mustTestDevIDCA(t), roots, NewSessionStore(time.Minute))
	devidKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pub := rsaPublicToTPM(devidKey, false)
	pub.Attributes = DevIDAttributes
	pub.RSAParameters.Sign = &tpm2.SigScheme{Alg: tpm2.AlgRSASSA, Hash: tpm2.AlgSHA256}
	pub.RSAParameters.Symmetric = nil
	sr := SigningRequest{DevIDKey: &pub}

	fpA := EKFingerprint(ekCertA)
	sid, err := enroller.Sessions.Put([]byte("nonce"), sr, "100", fpA)
	if err != nil {
		t.Fatal(err)
	}

	// Finish-style auth with a different trusted EK certificate succeeds at chain
	// verification, then Finish rejects on the session fingerprint.
	req := httptest.NewRequest(http.MethodPost, "/latest/devid/enroll/finish", nil)
	req.Header.Set(HeaderEKCert, EncodeEKCertHeader(ekCertB))
	vm := &inventory.VMConfig{VMID: "100", MACs: []string{"52:54:00:a1:b2:c3"}}
	req = req.WithContext(context.WithValue(req.Context(), inventory.VMConfigContextKey, vm))

	vmid, ek, err := enroller.AuthenticateEnrollCaller(req, nil, nil, "")
	if err != nil {
		t.Fatalf("finish auth (chain-only) should accept other trusted EK: %v", err)
	}
	if _, err := enroller.Finish(sid, []byte("nonce"), vmid, EKFingerprint(ek)); err == nil ||
		err.Error() != "EK certificate does not match enroll session" {
		t.Fatalf("expected session EK mismatch, got %v", err)
	}
}

// captureLog runs fn while capturing the default log destination.
func captureLog(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	prev := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(prev)
		log.SetFlags(prevFlags)
	}()
	fn()
	return buf.String()
}
