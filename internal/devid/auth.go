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
	"log"
	"net/http"
	"strings"

	"github.com/dembaca/vtpm-mds/internal/inventory"
	"github.com/google/go-tpm/legacy/tpm2"
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
// equal the CSR endorsement certificate and must certify endorsementKey.
func (e *Enroller) AuthenticateEnrollCaller(r *http.Request, csrEK *x509.Certificate, endorsementKey *tpm2.Public, expectedFP string) (vmID string, ekCert *x509.Certificate, err error) {
	vm := inventory.GetVMConfigFromRequest(r)
	if vm == nil || vm.VMID == "" {
		return "", nil, errUnauthorized("VM identity required (MAC not in inventory)")
	}

	hdr := r.Header.Get(HeaderEKCert)
	ekCert, err = ParseEKCertHeader(hdr)
	if err != nil {
		return "", nil, errUnauthorized(err.Error())
	}

	if csrEK != nil {
		// enroll/start: chain + bind the certificate to the endorsement key.
		if err := VerifyEKCertificateBound(e.EKRoots, endorsementKey, ekCert); err != nil {
			if errors.Is(err, ErrEKCertificateKeyMismatch) {
				return "", nil, errUnauthorized(ErrEKCertificateKeyMismatch.Error())
			}
			return "", nil, errUnauthorized(fmt.Sprintf("EK certificate not trusted: %v", err))
		}
	} else {
		// enroll/finish: chain only; the session fingerprint carries the binding.
		if err := VerifyEKCertificateChain(e.EKRoots, ekCert); err != nil {
			return "", nil, errUnauthorized(fmt.Sprintf("EK certificate not trusted: %v", err))
		}
	}

	fp := EKFingerprint(ekCert)
	if pin := strings.TrimSpace(vm.EKSHA256); pin != "" {
		if !strings.EqualFold(pin, fp) {
			return "", nil, errUnauthorized(errInventoryPinMismatch)
		}
	} else if err := e.enforceOwnPin(vm.VMID, fp); err != nil {
		return "", nil, err
	}
	if other, perr := e.pinnedToAnotherVM(vm.VMID, fp); perr != nil {
		return "", nil, perr
	} else if other != "" {
		return "", nil, errPinnedToAnotherVM(vm.VMID, other)
	}
	if expectedFP != "" && !strings.EqualFold(expectedFP, fp) {
		return "", nil, errUnauthorized("EK certificate does not match enroll session")
	}
	if csrEK != nil && !certsDEREqual(csrEK, ekCert) {
		return "", nil, errUnauthorized("EK certificate header does not match signing request")
	}
	return vm.VMID, ekCert, nil
}

// enforceOwnPin checks the caller's cluster pin, else its local pin, when
// the inventory has none (read-cluster-ek-pins-from-pmxcfs D-c, D-e).
func (e *Enroller) enforceOwnPin(vmid, fp string) error {
	cpin, dirExists, err := e.readClusterPin(vmid)
	if err != nil {
		return err
	}
	var stored string
	if e.PinDir != "" {
		if stored, err = readStoredPin(e.PinDir, vmid); err != nil {
			return err
		}
	}
	if cpin != "" {
		if !strings.EqualFold(cpin, fp) {
			return errUnauthorized(errClusterPinMismatch)
		}
		if stored != "" && !strings.EqualFold(stored, cpin) {
			log.Printf("devid enroll: local pin differs from cluster pin vm=%s", vmid)
		}
		return nil
	}
	if e.RequireClusterPin {
		return errUnauthorized(errNoClusterPin)
	}
	if dirExists {
		log.Printf("devid enroll: no cluster pin vm=%s, using local pin", vmid)
	}
	if stored != "" && !strings.EqualFold(stored, fp) {
		return errUnauthorized(errStoredPinMismatch)
	}
	return nil
}

// unauthorizedError is a 401. msg is the response body; detail is appended
// for the host log only (it may name another VM, which a guest must not see).
type unauthorizedError struct{ msg, detail string }

func (e unauthorizedError) Error() string {
	if e.detail == "" {
		return e.msg
	}
	return e.msg + " " + e.detail
}

func errUnauthorized(msg string) error { return unauthorizedError{msg: msg} }

// errPinnedToAnotherVM names both VMs in the log line, neither in the body
// and never the fingerprint.
func errPinnedToAnotherVM(vmid, other string) error {
	return unauthorizedError{msg: errPinnedElsewhere, detail: fmt.Sprintf("vm=%s pinned=%s", vmid, other)}
}

// responseBody is the text a guest gets for err: the bare message of a 401,
// without the log-only detail.
func responseBody(err error) string {
	var u unauthorizedError
	if errors.As(err, &u) {
		return u.msg
	}
	return err.Error()
}

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
