package devid

import (
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"fmt"
)

var (
	oidSubjectAltName        = asn1.ObjectIdentifier{2, 5, 29, 17}
	oidOnHardwareModuleName  = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 8, 4}
	oidOnPermanentIdentifier = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 8, 3}
	oidTCGHardwareTypeTPM2   = asn1.ObjectIdentifier{2, 23, 133, 1, 2}
	oidTCGOnEKPermIDSha256   = asn1.ObjectIdentifier{2, 23, 133, 12, 1}
)

type generalName struct {
	OtherName otherName `asn1:"tag:0"`
}

type otherName struct {
	TypeID asn1.ObjectIdentifier
	Value  any `asn1:"tag:0"`
}

type hardwareModuleName struct {
	Type         asn1.ObjectIdentifier
	SerialNumber []byte
}

type permanentIdentifier struct {
	IdentifierValue string                `asn1:"optional,utf8"`
	Assigner        asn1.ObjectIdentifier `asn1:"optional"`
}

func buildSAN(subjectIsEmpty bool, hwSerialNum []byte, permanentID string) (pkix.Extension, error) {
	names := []generalName{
		{
			OtherName: otherName{
				TypeID: oidOnHardwareModuleName,
				Value: hardwareModuleName{
					Type:         oidTCGHardwareTypeTPM2,
					SerialNumber: hwSerialNum,
				},
			},
		},
		{
			OtherName: otherName{
				TypeID: oidOnPermanentIdentifier,
				Value: permanentIdentifier{
					IdentifierValue: permanentID,
					Assigner:        oidTCGOnEKPermIDSha256,
				},
			},
		},
	}
	data, err := asn1.Marshal(names)
	if err != nil {
		return pkix.Extension{}, err
	}
	return pkix.Extension{
		Id:       oidSubjectAltName,
		Critical: subjectIsEmpty,
		Value:    data,
	}, nil
}

// DevIDSANFromEKPublicKey builds a TCG SAN using SHA-256 of the EK public key
// as hwSerialNum (avoids brittle EK certificate SAN parsing).
func DevIDSANFromEKPublicKey(subjectIsEmpty bool, pub crypto.PublicKey) (pkix.Extension, error) {
	keyBytes, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return pkix.Extension{}, err
	}
	hwSerial := sha256.Sum256(keyBytes)

	h := sha256.New()
	h.Write([]byte("EkPubkey"))
	h.Write(keyBytes)
	permanentID := fmt.Sprintf("%X", h.Sum(nil))

	return buildSAN(subjectIsEmpty, hwSerial[:], permanentID)
}

// DevIDSANFromEKCertificate builds a TCG SAN from an EK certificate digest.
// Prefer DevIDSANFromEKPublicKey when the EK public key is available.
func DevIDSANFromEKCertificate(subjectIsEmpty bool, cert *x509.Certificate) (pkix.Extension, error) {
	if cert == nil {
		return pkix.Extension{}, fmt.Errorf("nil EK certificate")
	}
	h := sha256.New()
	h.Write(cert.Raw)
	permanentID := fmt.Sprintf("%X", h.Sum(nil))

	// Without manufacturer fields, use a digest of the cert as hwSerialNum.
	hwSerial := sha256.Sum256(cert.Raw)
	return buildSAN(subjectIsEmpty, hwSerial[:], permanentID)
}
