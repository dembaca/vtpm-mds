package devid

import (
	"bytes"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"strings"
	"testing"
)

// TestEnrollSubjectIsAuthenticatedVMID is the regression test for the
// guest-controlled subject defect: the common name of an issued LDevID must be
// the authenticated VM id, never the one the guest put in its signing request.
func TestEnrollSubjectIsAuthenticatedVMID(t *testing.T) {
	ca := newEKCA(t)
	guest := newFakeTPM(t)
	ekCert := ca.issue(t, &guest.ekKey.PublicKey, "guest-ek")
	e := newTestEnroller(t, ca.roots)

	platform := pkix.Name{CommonName: "other-name"}.ToRDNSequence()
	cert := enrollOnce(t, e, guest, ekCert, "100", platform)

	if cert.Subject.CommonName != "100" {
		t.Fatalf("CN=%q, want %q", cert.Subject.CommonName, "100")
	}
	if strings.Contains(cert.Subject.String(), "other-name") {
		t.Fatalf("subject=%q still carries the guest-supplied name", cert.Subject.String())
	}
}

// TestEnrollSubjectWithoutPlatformIdentity covers the path that already used
// the VM id, which must be unchanged.
func TestEnrollSubjectWithoutPlatformIdentity(t *testing.T) {
	ca := newEKCA(t)
	guest := newFakeTPM(t)
	ekCert := ca.issue(t, &guest.ekKey.PublicKey, "guest-ek")
	e := newTestEnroller(t, ca.roots)

	cert := enrollOnce(t, e, guest, ekCert, "100", nil)

	if got := cert.Subject.String(); got != "CN=100" {
		t.Fatalf("subject=%q, want %q", got, "CN=100")
	}
}

// TestEnrollSubjectKeepsOtherPlatformAttributes checks that the relative
// distinguished names which assert no identity survive.
func TestEnrollSubjectKeepsOtherPlatformAttributes(t *testing.T) {
	ca := newEKCA(t)
	guest := newFakeTPM(t)
	ekCert := ca.issue(t, &guest.ekKey.PublicKey, "guest-ek")
	e := newTestEnroller(t, ca.roots)

	platform := pkix.Name{
		CommonName:         "other-name",
		Organization:       []string{"Example GmbH"},
		OrganizationalUnit: []string{"lab"},
	}.ToRDNSequence()
	cert := enrollOnce(t, e, guest, ekCert, "100", platform)

	if cert.Subject.CommonName != "100" {
		t.Fatalf("CN=%q, want %q", cert.Subject.CommonName, "100")
	}
	if got := cert.Subject.Organization; len(got) != 1 || got[0] != "Example GmbH" {
		t.Fatalf("O=%v, want [Example GmbH]", got)
	}
	if got := cert.Subject.OrganizationalUnit; len(got) != 1 || got[0] != "lab" {
		t.Fatalf("OU=%v, want [lab]", got)
	}
	if strings.Contains(cert.Subject.String(), "other-name") {
		t.Fatalf("subject=%q still carries the guest-supplied name", cert.Subject.String())
	}
}

// TestIssuedCertificateShape covers the derived values the subject rule could
// disturb: the TCG SAN is never critical because the subject is never empty,
// and nothing else about the certificate changes.
func TestIssuedCertificateShape(t *testing.T) {
	ca := newEKCA(t)
	guest := newFakeTPM(t)
	ekCert := ca.issue(t, &guest.ekKey.PublicKey, "guest-ek")
	e := newTestEnroller(t, ca.roots)

	cert := enrollOnce(t, e, guest, ekCert, "100", pkix.Name{CommonName: "other-name"}.ToRDNSequence())

	san, ok := findExtension(cert, oidSubjectAltName)
	if !ok {
		t.Fatal("missing subjectAltName extension")
	}
	if san.Critical {
		t.Fatal("subjectAltName is critical, but the subject always carries the VM id")
	}
	for _, oid := range []asn1.ObjectIdentifier{
		oidOnHardwareModuleName,
		oidTCGHardwareTypeTPM2,
		oidOnPermanentIdentifier,
		oidTCGOnEKPermIDSha256,
	} {
		der, err := asn1.Marshal(oid)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(san.Value, der) {
			t.Fatalf("TCG SAN does not carry %v", oid)
		}
	}

	if !cert.NotAfter.Equal(NoExpiration) {
		t.Fatalf("NotAfter=%v, want %v", cert.NotAfter, NoExpiration)
	}
	if cert.KeyUsage != x509.KeyUsageDigitalSignature {
		t.Fatalf("key usage=%v, want digitalSignature only", cert.KeyUsage)
	}
	if !cert.BasicConstraintsValid || cert.IsCA {
		t.Fatalf("basic constraints: valid=%v isCA=%v", cert.BasicConstraintsValid, cert.IsCA)
	}
	foundEKU := false
	for _, oid := range cert.UnknownExtKeyUsage {
		if oid.Equal(OIDVerifiedTPMFixed) {
			foundEKU = true
		}
	}
	if !foundEKU {
		t.Fatalf("extended key usage=%v, want %v", cert.UnknownExtKeyUsage, OIDVerifiedTPMFixed)
	}
}

// TestIssuedSerialIsDeterministic checks that the serial derivation is
// unchanged: the same DevID key enrolled twice by the same VM keeps its serial.
func TestIssuedSerialIsDeterministic(t *testing.T) {
	ca := newEKCA(t)
	guest := newFakeTPM(t)
	ekCert := ca.issue(t, &guest.ekKey.PublicKey, "guest-ek")
	e := newTestEnroller(t, ca.roots)

	first := enrollOnce(t, e, guest, ekCert, "100", nil)
	second := enrollOnce(t, e, guest, ekCert, "100", nil)

	if first.SerialNumber.Cmp(second.SerialNumber) != 0 {
		t.Fatalf("serials differ: %s vs %s", first.SerialNumber, second.SerialNumber)
	}
}
