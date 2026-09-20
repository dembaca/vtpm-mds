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
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dembaca/vtpm-mds/internal/inventory"
	"github.com/google/go-tpm/legacy/tpm2"
)

// tpmGeneratedValue is TPM_GENERATED_VALUE, the magic prefix of TPMS_ATTEST.
const tpmGeneratedValue = 0xff544347

// fakeTPM stands in for a guest TPM: an endorsement key, an attestation key
// and a DevID key, each with the TPM public area the enroll protocol carries.
type fakeTPM struct {
	ekKey  *rsa.PrivateKey
	akKey  *rsa.PrivateKey
	devKey *rsa.PrivateKey

	ekPub  tpm2.Public
	akPub  tpm2.Public
	devPub tpm2.Public
}

func mustRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func newFakeTPM(t *testing.T) *fakeTPM {
	t.Helper()
	f := &fakeTPM{
		ekKey:  mustRSAKey(t),
		akKey:  mustRSAKey(t),
		devKey: mustRSAKey(t),
	}
	f.ekPub = rsaPublicToTPM(f.ekKey, true)

	f.akPub = DefaultAKTemplateRSA()
	f.akPub.RSAParameters.ModulusRaw = f.akKey.PublicKey.N.Bytes()
	f.akPub.RSAParameters.ExponentRaw = uint32(f.akKey.PublicKey.E)

	f.devPub = DefaultDevIDTemplateRSA()
	f.devPub.RSAParameters.ModulusRaw = f.devKey.PublicKey.N.Bytes()
	f.devPub.RSAParameters.ExponentRaw = uint32(f.devKey.PublicKey.E)
	return f
}

func signPKCS1v15(t *testing.T, key *rsa.PrivateKey, data []byte) []byte {
	t.Helper()
	digest := sha256.Sum256(data)
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return sig
}

// signingRequest builds a signing request that passes every check other than
// the EK binding, together with the DevID signature over its exact bytes.
func (f *fakeTPM) signingRequest(t *testing.T, ekCert *x509.Certificate, platform pkix.RDNSequence, mutate ...func(*SigningRequest)) (*SigningRequest, []byte, []byte) {
	t.Helper()
	devName, err := f.devPub.Name()
	if err != nil {
		t.Fatal(err)
	}
	attest := tpm2.AttestationData{
		Magic:               tpmGeneratedValue,
		Type:                tpm2.TagAttestCertify,
		AttestedCertifyInfo: &tpm2.CertifyInfo{Name: devName},
	}
	certifyData, err := attest.Encode()
	if err != nil {
		t.Fatal(err)
	}

	sr := &SigningRequest{
		PlatformIdentity:       platform,
		EndorsementCertificate: ekCert,
		EndorsementKey:         &f.ekPub,
		AttestationKey:         &f.akPub,
		DevIDKey:               &f.devPub,
		CertifyData:            certifyData,
		CertifySignature:       signPKCS1v15(t, f.akKey, certifyData),
	}
	for _, m := range mutate {
		m(sr)
	}
	data, err := sr.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	return sr, data, signPKCS1v15(t, f.devKey, data)
}

// ekCA is a certificate authority standing in for `ek_ca_chain`.
type ekCA struct {
	cert  *x509.Certificate
	key   *rsa.PrivateKey
	roots *x509.CertPool
}

func newEKCA(t *testing.T) *ekCA {
	t.Helper()
	key := mustRSAKey(t)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ek-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
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
	return &ekCA{cert: cert, key: key, roots: roots}
}

// issue mints an EK certificate over pub. Nothing ties pub to the caller, which
// is exactly what the binding check has to catch.
func (c *ekCA) issue(t *testing.T, pub crypto.PublicKey, cn string) *x509.Certificate {
	t.Helper()
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, c.cert, pub, c.key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

func newTestEnroller(t *testing.T, roots *x509.CertPool) *Enroller {
	t.Helper()
	caCert, caKey := mustTestCA(t)
	return NewEnroller(&CA{cert: caCert, key: caKey}, roots, NewSessionStore(time.Minute))
}

func sessionCount(s *SessionStore) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sessions)
}

func enrollRequest(t *testing.T, path, vmID string, ekCert *x509.Certificate, body any) *http.Request {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set(HeaderEKCert, EncodeEKCertHeader(ekCert))
	vm := &inventory.VMConfig{VMID: vmID, MACs: []string{"52:54:00:a1:b2:c3"}}
	return req.WithContext(context.WithValue(req.Context(), inventory.VMConfigContextKey, vm))
}

// postStart runs the enroll/start handler and returns the recorded response.
func postStart(t *testing.T, e *Enroller, vmID string, headerCert *x509.Certificate, data, sig []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := enrollRequest(t, "/latest/devid/enroll/start", vmID, headerCert, EnrollStartRequest{
		RequestB64:   base64.StdEncoding.EncodeToString(data),
		SignatureB64: base64.StdEncoding.EncodeToString(sig),
	})
	rec := httptest.NewRecorder()
	e.HandleStart(rec, req)
	return rec
}

// postFinish runs the enroll/finish handler and returns the recorded response.
func postFinish(t *testing.T, e *Enroller, vmID string, headerCert *x509.Certificate, sessionID string, response []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := enrollRequest(t, "/latest/devid/enroll/finish", vmID, headerCert, EnrollFinishRequest{
		SessionID:            sessionID,
		ChallengeResponseB64: base64.StdEncoding.EncodeToString(response),
	})
	rec := httptest.NewRecorder()
	e.HandleFinish(rec, req)
	return rec
}
