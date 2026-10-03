package devid

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/google/go-tpm/legacy/tpm2"
)

func TestTemplateAttributes(t *testing.T) {
	ak := DefaultAKTemplateRSA()
	if err := CheckAKProp(ak.Attributes); err != nil {
		t.Fatalf("AK template: %v", err)
	}
	dev := DefaultDevIDTemplateRSA()
	if err := CheckDevIDProp(dev.Attributes); err != nil {
		t.Fatalf("DevID template: %v", err)
	}

	// Negative cases
	if err := CheckDevIDProp(ak.Attributes); err == nil {
		t.Fatal("expected DevID check to reject restricted AK attributes")
	}
	bad := DevIDAttributes | tpm2.FlagDecrypt
	if err := CheckDevIDProp(bad); err == nil {
		t.Fatal("expected DevID check to reject decrypt")
	}
	if err := CheckAKProp(DevIDAttributes); err == nil {
		t.Fatal("expected AK check to reject non-restricted DevID attributes")
	}
}

func TestCreateChallengeRSA(t *testing.T) {
	ekPriv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ekPub := &tpm2.Public{
		Type:       tpm2.AlgRSA,
		NameAlg:    tpm2.AlgSHA256,
		Attributes: tpm2.FlagStorageDefault,
		RSAParameters: &tpm2.RSAParams{
			Symmetric: &tpm2.SymScheme{
				Alg:     tpm2.AlgAES,
				KeyBits: 128,
				Mode:    tpm2.AlgCFB,
			},
			KeyBits:     2048,
			ExponentRaw: uint32(ekPriv.PublicKey.E),
			ModulusRaw:  ekPriv.PublicKey.N.Bytes(),
		},
	}

	akPub := DefaultAKTemplateRSA()
	// Give AK a distinct modulus so Name() is well-formed.
	akKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	akPub.RSAParameters.ModulusRaw = akKey.PublicKey.N.Bytes()
	akPub.RSAParameters.ExponentRaw = uint32(akKey.PublicKey.E)

	name, err := akPub.Name()
	if err != nil {
		t.Fatal(err)
	}
	cred, secret, nonce, err := CreateChallenge(ekPub, name)
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	if len(cred) == 0 || len(secret) == 0 || len(nonce) == 0 {
		t.Fatalf("empty challenge outputs: cred=%d secret=%d nonce=%d", len(cred), len(secret), len(nonce))
	}
	if len(nonce) != 32 {
		t.Fatalf("nonce length=%d want 32", len(nonce))
	}
}

func TestCASignAndParse(t *testing.T) {
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

	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		Subject:               pkix.Name{CommonName: "test-devid"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              NoExpiration,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		PublicKey:             &leafKey.PublicKey,
	}
	der, err := ca.Sign(template)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Subject.CommonName != "test-devid" {
		t.Fatalf("CN=%q", parsed.Subject.CommonName)
	}
	if !parsed.NotAfter.Equal(NoExpiration) {
		t.Fatalf("NotAfter=%v want %v", parsed.NotAfter, NoExpiration)
	}
}

func TestIssueDevIDAndSessionNonceMismatch(t *testing.T) {
	caCert, caKey := mustTestCA(t)
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw})
	keyDER, _ := x509.MarshalPKCS8PrivateKey(caKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	ca, err := ParseCA(caPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}

	ekKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	devKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	ekTPM := rsaPublicToTPM(ekKey, true)
	devTPM := rsaPublicToTPM(devKey, false)
	devTPM.Attributes = DevIDAttributes
	devTPM.RSAParameters.Sign = &tpm2.SigScheme{Alg: tpm2.AlgRSASSA, Hash: tpm2.AlgSHA256}
	devTPM.RSAParameters.Symmetric = nil

	sr := &SigningRequest{
		EndorsementKey: &ekTPM,
		DevIDKey:       &devTPM,
		PlatformIdentity: pkix.Name{CommonName: "vm-100"}.ToRDNSequence(),
	}

	cert, err := ca.IssueDevID(sr, "100", time.Now().UTC())
	if err != nil {
		t.Fatalf("IssueDevID: %v", err)
	}
	if cert.Subject.CommonName != "100" {
		t.Fatalf("CN=%q want 100 (authenticated VM ID)", cert.Subject.CommonName)
	}
	if cert.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		t.Fatal("missing digitalSignature key usage")
	}
	foundEKU := false
	for _, oid := range cert.UnknownExtKeyUsage {
		if oid.Equal(OIDVerifiedTPMFixed) {
			foundEKU = true
		}
	}
	if !foundEKU {
		t.Fatal("missing verifiedTPMFixed EKU")
	}

	enroller := NewEnroller(ca, x509.NewCertPool(), NewSessionStore(time.Minute))
	sid, err := enroller.Sessions.Put([]byte("correct-nonce"), *sr, "100", "ekfp")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := enroller.Finish(sid, []byte("wrong-nonce"), "100", "ekfp"); err == nil {
		t.Fatal("expected nonce mismatch error")
	}

	sid2, err := enroller.Sessions.Put([]byte("correct-nonce"), *sr, "100", "ekfp")
	if err != nil {
		t.Fatal(err)
	}
	pemBytes, err := enroller.Finish(sid2, []byte("correct-nonce"), "100", "ekfp")
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatal("expected PEM certificate")
	}
}

// TestGuestSuppliedCommonNameIsIgnored: platform identity CN other-name is
// ignored; the authenticated VM ID becomes the certificate CN.
func TestGuestSuppliedCommonNameIsIgnored(t *testing.T) {
	ca := mustParseTestCA(t)
	sr := mustMinimalSigningRequest(t, pkix.Name{CommonName: "other-name"})

	cert, err := ca.IssueDevID(sr, "100", time.Now().UTC())
	if err != nil {
		t.Fatalf("IssueDevID: %v", err)
	}
	if cert.Subject.CommonName != "100" {
		t.Fatalf("CN=%q want 100", cert.Subject.CommonName)
	}
	if cert.Subject.String() != "CN=100" {
		t.Fatalf("subject=%q", cert.Subject.String())
	}

	enroller := NewEnroller(ca, x509.NewCertPool(), NewSessionStore(time.Minute))
	sid, err := enroller.Sessions.Put([]byte("nonce"), *sr, "100", "ekfp")
	if err != nil {
		t.Fatal(err)
	}
	pemBytes, err := enroller.Finish(sid, []byte("nonce"), "100", "ekfp")
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		t.Fatal("expected PEM certificate")
	}
	issued, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if issued.Subject.CommonName != "100" {
		t.Fatalf("Finish CN=%q want 100", issued.Subject.CommonName)
	}
	if issued.Subject.String() != "CN=100" {
		t.Fatalf("Finish subject=%q contains guest name", issued.Subject.String())
	}
}

func TestIssueDevIDVMIDWhenNoPlatformIdentity(t *testing.T) {
	ca := mustParseTestCA(t)
	sr := mustMinimalSigningRequest(t, pkix.Name{})

	cert, err := ca.IssueDevID(sr, "100", time.Now().UTC())
	if err != nil {
		t.Fatalf("IssueDevID: %v", err)
	}
	if cert.Subject.CommonName != "100" {
		t.Fatalf("CN=%q want 100", cert.Subject.CommonName)
	}
	if cert.Subject.String() != "CN=100" {
		t.Fatalf("subject=%q", cert.Subject.String())
	}
}

func TestIssueDevIDPreservesOrganizationAndOU(t *testing.T) {
	ca := mustParseTestCA(t)
	sr := mustMinimalSigningRequest(t, pkix.Name{
		CommonName:         "other-name",
		Organization:       []string{"Example GmbH"},
		OrganizationalUnit: []string{"lab"},
	})

	cert, err := ca.IssueDevID(sr, "100", time.Now().UTC())
	if err != nil {
		t.Fatalf("IssueDevID: %v", err)
	}
	if cert.Subject.CommonName != "100" {
		t.Fatalf("CN=%q want 100", cert.Subject.CommonName)
	}
	if len(cert.Subject.Organization) != 1 || cert.Subject.Organization[0] != "Example GmbH" {
		t.Fatalf("Organization=%v", cert.Subject.Organization)
	}
	if len(cert.Subject.OrganizationalUnit) != 1 || cert.Subject.OrganizationalUnit[0] != "lab" {
		t.Fatalf("OrganizationalUnit=%v", cert.Subject.OrganizationalUnit)
	}
}

func TestIssueDevIDSANNotCritical(t *testing.T) {
	ca := mustParseTestCA(t)
	sr := mustMinimalSigningRequest(t, pkix.Name{CommonName: "other-name"})

	cert, err := ca.IssueDevID(sr, "100", time.Now().UTC())
	if err != nil {
		t.Fatalf("IssueDevID: %v", err)
	}
	found := false
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(oidSubjectAltName) {
			found = true
			if ext.Critical {
				t.Fatal("SAN extension marked critical; subject is never empty")
			}
		}
	}
	if !found {
		t.Fatal("missing SAN extension")
	}
}

func TestIssueDevIDCertificateShape(t *testing.T) {
	ca := mustParseTestCA(t)
	sr := mustMinimalSigningRequest(t, pkix.Name{Organization: []string{"Example GmbH"}})

	cert, err := ca.IssueDevID(sr, "100", time.Now().UTC())
	if err != nil {
		t.Fatalf("IssueDevID: %v", err)
	}
	if !cert.NotAfter.Equal(NoExpiration) {
		t.Fatalf("NotAfter=%v want %v", cert.NotAfter, NoExpiration)
	}
	if cert.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		t.Fatal("missing digitalSignature key usage")
	}
	if cert.IsCA {
		t.Fatal("IsCA true")
	}
	foundEKU := false
	for _, oid := range cert.UnknownExtKeyUsage {
		if oid.Equal(OIDVerifiedTPMFixed) {
			foundEKU = true
		}
	}
	if !foundEKU {
		t.Fatal("missing verifiedTPMFixed EKU")
	}
	foundSAN := false
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(oidSubjectAltName) {
			foundSAN = true
			if err := assertTCGDevIDSAN(ext.Value); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !foundSAN {
		t.Fatal("missing SAN extension")
	}
}

func TestIssueDevIDSerialDeterministic(t *testing.T) {
	ca := mustParseTestCA(t)
	sr := mustMinimalSigningRequest(t, pkix.Name{CommonName: "other-name"})
	now := time.Now().UTC()

	cert1, err := ca.IssueDevID(sr, "100", now)
	if err != nil {
		t.Fatalf("IssueDevID 1: %v", err)
	}
	cert2, err := ca.IssueDevID(sr, "100", now)
	if err != nil {
		t.Fatalf("IssueDevID 2: %v", err)
	}
	if cert1.SerialNumber.Cmp(cert2.SerialNumber) != 0 {
		t.Fatalf("serials differ: %s vs %s", cert1.SerialNumber, cert2.SerialNumber)
	}
}

func TestSigningRequestRoundTrip(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pub := rsaPublicToTPM(key, false)
	pub.Attributes = DevIDAttributes
	pub.RSAParameters.Sign = &tpm2.SigScheme{Alg: tpm2.AlgRSASSA, Hash: tpm2.AlgSHA256}
	pub.RSAParameters.Symmetric = nil

	sr := &SigningRequest{
		PlatformIdentity: pkix.Name{CommonName: "host1"}.ToRDNSequence(),
		DevIDKey:         &pub,
		CertifyData:      []byte{1, 2, 3},
		CertifySignature: []byte{4, 5, 6},
	}
	data, err := sr.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	var out SigningRequest
	if err := out.UnmarshalBinary(data); err != nil {
		t.Fatal(err)
	}
	if string(out.CertifyData) != string(sr.CertifyData) {
		t.Fatal("certify data mismatch")
	}
	var name pkix.Name
	name.FillFromRDNSequence(&out.PlatformIdentity)
	if name.CommonName != "host1" {
		t.Fatalf("CN=%q", name.CommonName)
	}
}

func rsaPublicToTPM(key *rsa.PrivateKey, storage bool) tpm2.Public {
	attrs := DevIDAttributes
	params := &tpm2.RSAParams{
		KeyBits:     2048,
		ExponentRaw: uint32(key.PublicKey.E),
		ModulusRaw:  key.PublicKey.N.Bytes(),
	}
	if storage {
		attrs = tpm2.FlagStorageDefault
		params.Symmetric = &tpm2.SymScheme{Alg: tpm2.AlgAES, KeyBits: 128, Mode: tpm2.AlgCFB}
	}
	return tpm2.Public{
		Type:          tpm2.AlgRSA,
		NameAlg:       tpm2.AlgSHA256,
		Attributes:    attrs,
		RSAParameters: params,
	}
}

func mustTestCA(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-devid-ca"},
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
	return cert, key
}

func assertTCGDevIDSAN(der []byte) error {
	// OtherName encoding is awkward to round-trip; verify the TCG otherName
	// type OIDs are present in the SAN SEQUENCE.
	hwOID, err := asn1.Marshal(oidOnHardwareModuleName)
	if err != nil {
		return err
	}
	permOID, err := asn1.Marshal(oidOnPermanentIdentifier)
	if err != nil {
		return err
	}
	if !bytes.Contains(der, hwOID) {
		return fmt.Errorf("SAN missing HardwareModuleName OID")
	}
	if !bytes.Contains(der, permOID) {
		return fmt.Errorf("SAN missing PermanentIdentifier OID")
	}
	tpmTypeOID, err := asn1.Marshal(oidTCGHardwareTypeTPM2)
	if err != nil {
		return err
	}
	if !bytes.Contains(der, tpmTypeOID) {
		return fmt.Errorf("SAN missing TPM hardware type OID")
	}
	return nil
}

func mustParseTestCA(t *testing.T) *CA {
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

func mustMinimalSigningRequest(t *testing.T, platform pkix.Name) *SigningRequest {
	t.Helper()
	ekKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	devKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ekTPM := rsaPublicToTPM(ekKey, true)
	devTPM := rsaPublicToTPM(devKey, false)
	devTPM.Attributes = DevIDAttributes
	devTPM.RSAParameters.Sign = &tpm2.SigScheme{Alg: tpm2.AlgRSASSA, Hash: tpm2.AlgSHA256}
	devTPM.RSAParameters.Symmetric = nil
	return &SigningRequest{
		EndorsementKey:   &ekTPM,
		DevIDKey:         &devTPM,
		PlatformIdentity: platform.ToRDNSequence(),
	}
}
