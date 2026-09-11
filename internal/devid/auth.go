package devid

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/dembaca/vtpm-mds/internal/inventory"
)

// HeaderEKCert is sent by the guest enroll client. Value is base64(DER) of the
// TPM Endorsement Key certificate (or a PEM block base64-encoded as a whole).
// Combined with MAC→inventory identity, this authenticates the enroll caller.
const HeaderEKCert = "X-vtpm-mds-ek-cert"

// EKFingerprint returns lowercase hex SHA-256 over the certificate DER.
func EKFingerprint(cert *x509.Certificate) string {
	if cert == nil {
		return ""
	}
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:])
}

// ParseEKCertHeader decodes an EK certificate from the enroll auth header.
func ParseEKCertHeader(v string) (*x509.Certificate, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, errors.New("missing EK certificate header")
	}
	raw, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return nil, fmt.Errorf("ek cert header: base64: %w", err)
	}
	// Accept raw DER or PEM.
	if block, _ := pem.Decode(raw); block != nil && block.Type == "CERTIFICATE" {
		raw = block.Bytes
	}
	cert, err := parseCertificateLoose(raw)
	if err != nil {
		return nil, fmt.Errorf("ek cert header: %w", err)
	}
	return cert, nil
}

// EncodeEKCertHeader base64-encodes certificate DER for HeaderEKCert.
func EncodeEKCertHeader(cert *x509.Certificate) string {
	if cert == nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(cert.Raw)
}

// AuthenticateEnrollCaller requires:
//  1. VM identity from MAC (inventory via request context)
//  2. A trusted EK certificate in HeaderEKCert
//
// If expectedFP is non-empty (session- or inventory-pinned), the header EK
// fingerprint must match. If csrEK is non-nil (enroll/start), header EK must
// equal the CSR endorsement certificate.
func (e *Enroller) AuthenticateEnrollCaller(r *http.Request, csrEK *x509.Certificate, expectedFP string) (vmID string, ekCert *x509.Certificate, err error) {
	vm := inventory.GetVMConfigFromRequest(r)
	if vm == nil || vm.VMID == "" {
		return "", nil, errUnauthorized("VM identity required (MAC not in inventory)")
	}

	hdr := r.Header.Get(HeaderEKCert)
	ekCert, err = ParseEKCertHeader(hdr)
	if err != nil {
		return "", nil, errUnauthorized(err.Error())
	}
	if err := VerifyEKCertificate(e.EKRoots, nil, ekCert); err != nil {
		return "", nil, errUnauthorized(fmt.Sprintf("EK certificate not trusted: %v", err))
	}

	fp := EKFingerprint(ekCert)
	if pin := strings.TrimSpace(vm.EKSHA256); pin != "" {
		if !strings.EqualFold(pin, fp) {
			return "", nil, errUnauthorized("EK certificate does not match inventory pin")
		}
	}
	if expectedFP != "" && !strings.EqualFold(expectedFP, fp) {
		return "", nil, errUnauthorized("EK certificate does not match enroll session")
	}
	if csrEK != nil && !certsDEREqual(csrEK, ekCert) {
		return "", nil, errUnauthorized("EK certificate header does not match signing request")
	}
	return vm.VMID, ekCert, nil
}

type unauthorizedError struct{ msg string }

func (e unauthorizedError) Error() string { return e.msg }

func errUnauthorized(msg string) error { return unauthorizedError{msg: msg} }

func isUnauthorized(err error) bool {
	var u unauthorizedError
	return errors.As(err, &u)
}

func certsDEREqual(a, b *x509.Certificate) bool {
	if a == nil || b == nil {
		return a == b
	}
	return bytes.Equal(a.Raw, b.Raw)
}
