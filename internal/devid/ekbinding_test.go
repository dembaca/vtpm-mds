package devid

import (
	"bytes"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"testing"
)

// wantEKBindingBody is the exact body an unbound EK certificate is answered
// with. http.Error appends a newline, so comparisons trim it.
const wantEKBindingBody = "EK certificate does not match the endorsement key in the signing request"

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	flags := log.Flags()
	out := log.Writer()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(out)
		log.SetFlags(flags)
	})
	return &buf
}

// TestEnrollStartRejectsBorrowedEKCertificate is the regression test for the
// EK binding defect: an EK certificate that chains to `ek_ca_chain` but was
// minted over another TPM's endorsement key must not authenticate a caller
// whose signing request carries its own endorsement key.
func TestEnrollStartRejectsBorrowedEKCertificate(t *testing.T) {
	ca := newEKCA(t)
	victim := newFakeTPM(t)
	attacker := newFakeTPM(t)

	// The certificate is the victim's: it chains to ek_ca_chain and certifies
	// the victim's endorsement key. EK certificates are not secret.
	borrowed := ca.issue(t, &victim.ekKey.PublicKey, "victim-ek")

	// The attacker presents it in the header and in its own signing request,
	// while the signing request carries the attacker's endorsement key, so the
	// challenge would be wrapped to a TPM the CA never certified.
	_, data, sig := attacker.signingRequest(t, borrowed, nil)

	logs := captureLog(t)
	e := newTestEnroller(t, ca.roots)
	rec := postStart(t, e, "100", borrowed, data, sig)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("enroll/start code=%d body=%q, want 401", rec.Code, rec.Body.String())
	}
	if got := strings.TrimSuffix(rec.Body.String(), "\n"); got != wantEKBindingBody {
		t.Fatalf("body=%q, want %q", got, wantEKBindingBody)
	}
	if n := sessionCount(e.Sessions); n != 0 {
		t.Fatalf("session count=%d, want 0: a rejected request must not create a session", n)
	}
	if want := "devid enroll auth: " + wantEKBindingBody; !strings.Contains(logs.String(), want) {
		t.Fatalf("log=%q, want it to contain %q", logs.String(), want)
	}
}

// TestEnrollStartAcceptsOwnEKCertificate is the counterpart: a guest whose EK
// certificate was issued over its own endorsement key still enrolls.
func TestEnrollStartAcceptsOwnEKCertificate(t *testing.T) {
	ca := newEKCA(t)
	guest := newFakeTPM(t)
	ekCert := ca.issue(t, &guest.ekKey.PublicKey, "guest-ek")

	_, data, sig := guest.signingRequest(t, ekCert, pkix.Name{CommonName: "guest100"}.ToRDNSequence())

	e := newTestEnroller(t, ca.roots)
	rec := postStart(t, e, "100", ekCert, data, sig)

	if rec.Code != http.StatusOK {
		t.Fatalf("enroll/start code=%d body=%q, want 200", rec.Code, rec.Body.String())
	}
	if n := sessionCount(e.Sessions); n != 1 {
		t.Fatalf("session count=%d, want 1", n)
	}
}

// TestEnrollStartRejectsRequestWithoutEndorsementKey covers a signing request
// that carries an endorsement certificate but no endorsement key public area:
// the binding cannot be established, so it is an authentication failure and
// not a late challenge-creation error.
func TestEnrollStartRejectsRequestWithoutEndorsementKey(t *testing.T) {
	ca := newEKCA(t)
	guest := newFakeTPM(t)
	ekCert := ca.issue(t, &guest.ekKey.PublicKey, "guest-ek")

	_, data, sig := guest.signingRequest(t, ekCert, nil, func(sr *SigningRequest) {
		sr.EndorsementKey = nil
	})

	e := newTestEnroller(t, ca.roots)
	rec := postStart(t, e, "100", ekCert, data, sig)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("enroll/start code=%d body=%q, want 401", rec.Code, rec.Body.String())
	}
	got := strings.TrimSuffix(rec.Body.String(), "\n")
	if got != wantEKBindingBody {
		t.Fatalf("body=%q, want %q", got, wantEKBindingBody)
	}
	if strings.Contains(got, "create challenge") {
		t.Fatalf("body=%q: must not reach challenge creation", got)
	}
	if n := sessionCount(e.Sessions); n != 0 {
		t.Fatalf("session count=%d, want 0", n)
	}
}

// TestEnrollFinishVerifiesChainOnly checks that the finish leg still verifies
// the chain only — it has no signing request — and that the session's EK
// fingerprint is what stops a caller substituting another trusted certificate.
func TestEnrollFinishVerifiesChainOnly(t *testing.T) {
	ca := newEKCA(t)
	guest := newFakeTPM(t)
	other := newFakeTPM(t)
	ekCert := ca.issue(t, &guest.ekKey.PublicKey, "guest-ek")
	otherCert := ca.issue(t, &other.ekKey.PublicKey, "other-ek")

	_, data, sig := guest.signingRequest(t, ekCert, nil)
	e := newTestEnroller(t, ca.roots)
	rec := postStart(t, e, "100", ekCert, data, sig)
	if rec.Code != http.StatusOK {
		t.Fatalf("enroll/start code=%d body=%q, want 200", rec.Code, rec.Body.String())
	}
	var start EnrollStartResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &start); err != nil {
		t.Fatal(err)
	}

	// otherCert is trusted, so chain-only verification accepts it; the session
	// binding is what rejects the request.
	if err := VerifyEKCertificateChain(ca.roots, otherCert); err != nil {
		t.Fatalf("chain verification of the substituted certificate: %v", err)
	}
	fin := postFinish(t, e, "100", otherCert, start.SessionID, []byte("whatever"))
	if fin.Code != http.StatusUnauthorized {
		t.Fatalf("enroll/finish code=%d body=%q, want 401", fin.Code, fin.Body.String())
	}
	if got, want := strings.TrimSuffix(fin.Body.String(), "\n"), "EK certificate does not match enroll session"; got != want {
		t.Fatalf("body=%q, want %q", got, want)
	}
}

// TestVerifyEKCertificateBinding covers the comparison itself.
func TestVerifyEKCertificateBinding(t *testing.T) {
	ca := newEKCA(t)
	guest := newFakeTPM(t)
	other := newFakeTPM(t)
	ekCert := ca.issue(t, &guest.ekKey.PublicKey, "guest-ek")

	if err := VerifyEKCertificateBinding(ca.roots, ekCert, &guest.ekPub); err != nil {
		t.Fatalf("matching endorsement key: %v", err)
	}
	if err := VerifyEKCertificateBinding(ca.roots, ekCert, &other.ekPub); err == nil {
		t.Fatal("expected a different endorsement key to be rejected")
	}
	if err := VerifyEKCertificateBinding(ca.roots, ekCert, nil); err == nil {
		t.Fatal("expected a nil endorsement key public area to be rejected")
	}

	// An untrusted certificate fails chain building, not the binding.
	untrusted := newEKCA(t).issue(t, &guest.ekKey.PublicKey, "guest-ek")
	err := VerifyEKCertificateBinding(x509.NewCertPool(), untrusted, &guest.ekPub)
	if err == nil || !strings.Contains(err.Error(), "EK certificate verification failed") {
		t.Fatalf("untrusted certificate: err=%v, want a chain failure", err)
	}
}
