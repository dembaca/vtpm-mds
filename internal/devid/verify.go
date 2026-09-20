package devid

import (
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"

	"github.com/google/go-tpm/legacy/tpm2"
)

// KeyAttributeError describes invalid TPM key properties.
type KeyAttributeError struct {
	Reason string
}

func (e KeyAttributeError) Error() string {
	return "key attribute error: " + e.Reason
}

// CheckDevIDProp validates DevID object attributes.
func CheckDevIDProp(prop tpm2.KeyProp) error {
	if prop&tpm2.FlagDecrypt != 0 {
		return KeyAttributeError{Reason: "DevID should not be a decryption key"}
	}
	if prop&tpm2.FlagRestricted != 0 {
		return KeyAttributeError{Reason: "DevID should not be a restricted key"}
	}
	if prop&tpm2.FlagSign == 0 {
		return KeyAttributeError{Reason: "DevID should be a signing key"}
	}
	if prop&tpm2.FlagFixedTPM == 0 {
		return KeyAttributeError{Reason: "DevID should be fixedTPM"}
	}
	return nil
}

// CheckAKProp validates attestation key object attributes.
func CheckAKProp(prop tpm2.KeyProp) error {
	if prop&tpm2.FlagDecrypt != 0 {
		return KeyAttributeError{Reason: "AK should not be a decryption key"}
	}
	if prop&tpm2.FlagRestricted == 0 {
		return KeyAttributeError{Reason: "AK should be a restricted key"}
	}
	if prop&tpm2.FlagSign == 0 {
		return KeyAttributeError{Reason: "AK should be a signing key"}
	}
	if prop&tpm2.FlagFixedTPM == 0 {
		return KeyAttributeError{Reason: "AK should be fixedTPM"}
	}
	return nil
}

// CheckSignature verifies an RSA PKCS#1 v1.5 signature over data with pub.
func CheckSignature(pub *tpm2.Public, data, sig []byte) error {
	if pub == nil {
		return errors.New("missing public key")
	}
	key, err := pub.Key()
	if err != nil {
		return err
	}
	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return errors.New("only RSA keys are supported")
	}
	scheme, err := SignatureScheme(*pub)
	if err != nil {
		return err
	}
	if scheme == nil {
		return errors.New("missing signature scheme")
	}
	hash, err := scheme.Hash.Hash()
	if err != nil {
		return err
	}
	h := hash.New()
	h.Write(data)
	return rsa.VerifyPKCS1v15(rsaKey, hash, h.Sum(nil), sig)
}

// VerifyDevIDResidency checks that attestData/attestSig prove pubDevID under pubAK.
func VerifyDevIDResidency(pubAK, pubDevID *tpm2.Public, attestData, attestSig []byte) error {
	if err := CheckSignature(pubAK, attestData, attestSig); err != nil {
		return fmt.Errorf("certify signature: %w", err)
	}
	data, err := tpm2.DecodeAttestationData(attestData)
	if err != nil {
		return fmt.Errorf("decode attest data: %w", err)
	}
	if data.AttestedCertifyInfo == nil {
		return errors.New("missing certify info")
	}
	ok, err := data.AttestedCertifyInfo.Name.MatchesPublic(*pubDevID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("certify name does not match DevID public")
	}
	return nil
}

// errEKKeyMismatch reports that an EK certificate does not certify the
// endorsement key it was presented with.
var errEKKeyMismatch = errors.New("EK certificate does not certify the endorsement key")

// VerifyEKCertificateChain verifies cert against roots, tolerating critical SAN
// extensions that Go's x509 verifier does not handle for EK certs. It says
// nothing about which TPM holds the certified key: use
// VerifyEKCertificateBinding wherever the endorsement key is available.
func VerifyEKCertificateChain(roots *x509.CertPool, cert *x509.Certificate) error {
	if cert == nil {
		return errors.New("missing EK certificate")
	}
	if roots == nil {
		return errors.New("missing EK trust roots")
	}

	if len(cert.UnhandledCriticalExtensions) > 0 {
		kept := make([]asn1.ObjectIdentifier, 0, len(cert.UnhandledCriticalExtensions))
		for _, oid := range cert.UnhandledCriticalExtensions {
			if oid.Equal(oidSubjectAltName) {
				continue
			}
			kept = append(kept, oid)
		}
		cert.UnhandledCriticalExtensions = kept
	}

	_, err := cert.Verify(x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	})
	if err != nil {
		return fmt.Errorf("EK certificate verification failed: %w", err)
	}
	return nil
}

// VerifyEKCertificateBinding verifies cert against roots and additionally
// requires that cert certifies pub, the endorsement key public area the caller
// is enrolling with. Without this the EK factor proves possession of a public
// certificate rather than of the TPM that certificate describes. A binding
// failure is reported as errEKKeyMismatch.
func VerifyEKCertificateBinding(roots *x509.CertPool, cert *x509.Certificate, pub *tpm2.Public) error {
	if err := VerifyEKCertificateChain(roots, cert); err != nil {
		return err
	}
	if pub == nil {
		return errEKKeyMismatch
	}
	ekKey, err := pub.Key()
	if err != nil {
		return fmt.Errorf("%w: %v", errEKKeyMismatch, err)
	}
	// crypto.PublicKey.Equal reports false for a key of another type, so an
	// endorsement key that is not the certified one never matches.
	certKey, ok := cert.PublicKey.(interface{ Equal(crypto.PublicKey) bool })
	if !ok || !certKey.Equal(ekKey) {
		return errEKKeyMismatch
	}
	return nil
}
