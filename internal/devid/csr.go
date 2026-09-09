package devid

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/go-tpm/legacy/tpm2"
	"github.com/google/go-tpm/tpmutil"
)

// EKRSACertificateHandle is the standard NV index for the RSA EK certificate.
const EKRSACertificateHandle = tpmutil.Handle(0x01c00002)

// SigningRequest is the DevID enrollment CSR payload.
type SigningRequest struct {
	PlatformIdentity       pkix.RDNSequence
	EndorsementCertificate *x509.Certificate
	EndorsementKey         *tpm2.Public
	AttestationKey         *tpm2.Public
	DevIDKey               *tpm2.Public
	CertifyData            []byte
	CertifySignature       []byte
}

// signingRequestWire is the JSON/base64 encoding of SigningRequest.
type signingRequestWire struct {
	PlatformIdentityB64       string `json:"platform_identity_b64,omitempty"`
	EndorsementCertificateB64 string `json:"endorsement_certificate_b64,omitempty"`
	EndorsementKeyB64         string `json:"endorsement_key_b64,omitempty"`
	AttestationKeyB64         string `json:"attestation_key_b64,omitempty"`
	DevIDKeyB64               string `json:"devid_key_b64,omitempty"`
	CertifyDataB64            string `json:"certify_data_b64,omitempty"`
	CertifySignatureB64       string `json:"certify_signature_b64,omitempty"`
}

// MarshalBinary encodes the signing request as canonical JSON (signed bytes).
func (sr *SigningRequest) MarshalBinary() ([]byte, error) {
	w := signingRequestWire{}
	if len(sr.PlatformIdentity) > 0 {
		b, err := asn1.Marshal(sr.PlatformIdentity)
		if err != nil {
			return nil, err
		}
		w.PlatformIdentityB64 = base64.StdEncoding.EncodeToString(b)
	}
	if sr.EndorsementCertificate != nil {
		w.EndorsementCertificateB64 = base64.StdEncoding.EncodeToString(sr.EndorsementCertificate.Raw)
	}
	if sr.EndorsementKey != nil {
		b, err := sr.EndorsementKey.Encode()
		if err != nil {
			return nil, err
		}
		w.EndorsementKeyB64 = base64.StdEncoding.EncodeToString(b)
	}
	if sr.AttestationKey != nil {
		b, err := sr.AttestationKey.Encode()
		if err != nil {
			return nil, err
		}
		w.AttestationKeyB64 = base64.StdEncoding.EncodeToString(b)
	}
	if sr.DevIDKey != nil {
		b, err := sr.DevIDKey.Encode()
		if err != nil {
			return nil, err
		}
		w.DevIDKeyB64 = base64.StdEncoding.EncodeToString(b)
	}
	if len(sr.CertifyData) > 0 {
		w.CertifyDataB64 = base64.StdEncoding.EncodeToString(sr.CertifyData)
	}
	if len(sr.CertifySignature) > 0 {
		w.CertifySignatureB64 = base64.StdEncoding.EncodeToString(sr.CertifySignature)
	}
	return json.Marshal(w)
}

// UnmarshalBinary decodes a canonical JSON signing request.
func (sr *SigningRequest) UnmarshalBinary(data []byte) error {
	var w signingRequestWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	*sr = SigningRequest{}
	if w.PlatformIdentityB64 != "" {
		b, err := base64.StdEncoding.DecodeString(w.PlatformIdentityB64)
		if err != nil {
			return err
		}
		if rest, err := asn1.Unmarshal(b, &sr.PlatformIdentity); err != nil {
			return err
		} else if len(rest) > 0 {
			return errors.New("trailing data in platform identity")
		}
	}
	if w.EndorsementCertificateB64 != "" {
		b, err := base64.StdEncoding.DecodeString(w.EndorsementCertificateB64)
		if err != nil {
			return err
		}
		cert, err := parseCertificateLoose(b)
		if err != nil {
			return fmt.Errorf("parse EK certificate: %w", err)
		}
		sr.EndorsementCertificate = cert
	}
	if w.EndorsementKeyB64 != "" {
		b, err := base64.StdEncoding.DecodeString(w.EndorsementKeyB64)
		if err != nil {
			return err
		}
		pub, err := tpm2.DecodePublic(b)
		if err != nil {
			return fmt.Errorf("decode EK: %w", err)
		}
		sr.EndorsementKey = &pub
	}
	if w.AttestationKeyB64 != "" {
		b, err := base64.StdEncoding.DecodeString(w.AttestationKeyB64)
		if err != nil {
			return err
		}
		pub, err := tpm2.DecodePublic(b)
		if err != nil {
			return fmt.Errorf("decode AK: %w", err)
		}
		sr.AttestationKey = &pub
	}
	if w.DevIDKeyB64 != "" {
		b, err := base64.StdEncoding.DecodeString(w.DevIDKeyB64)
		if err != nil {
			return err
		}
		pub, err := tpm2.DecodePublic(b)
		if err != nil {
			return fmt.Errorf("decode DevID: %w", err)
		}
		sr.DevIDKey = &pub
	}
	if w.CertifyDataB64 != "" {
		b, err := base64.StdEncoding.DecodeString(w.CertifyDataB64)
		if err != nil {
			return err
		}
		sr.CertifyData = b
	}
	if w.CertifySignatureB64 != "" {
		b, err := base64.StdEncoding.DecodeString(w.CertifySignatureB64)
		if err != nil {
			return err
		}
		sr.CertifySignature = b
	}
	return nil
}

func parseCertificateLoose(asn1Data []byte) (*x509.Certificate, error) {
	var value asn1.RawValue
	if _, err := asn1.Unmarshal(asn1Data, &value); err != nil {
		return x509.ParseCertificate(asn1Data)
	}
	return x509.ParseCertificate(value.FullBytes)
}

// RequestResources holds live TPM handles created while building a CSR.
type RequestResources struct {
	Attestation *KeyInfo
	Endorsement *KeyInfo
	DevID       *KeyInfo
	rw          io.ReadWriter
}

// Flush releases transient key contexts.
func (r *RequestResources) Flush() {
	if r == nil {
		return
	}
	if r.Attestation != nil {
		FlushTransient(r.rw, r.Attestation.Handle)
		r.Attestation.Handle = 0
	}
	if r.DevID != nil {
		FlushTransient(r.rw, r.DevID.Handle)
		r.DevID.Handle = 0
	}
	// EK is typically persistent; leave it loaded.
}

// Activate solves a credential challenge with the CSR keys.
func (r *RequestResources) Activate(credentialBlob, secret []byte) ([]byte, error) {
	if r.Attestation == nil || r.Endorsement == nil {
		return nil, errors.New("missing AK or EK")
	}
	return ActivateCredential(r.rw, r.Attestation.Handle, r.Endorsement.Handle, credentialBlob, secret)
}

// CreateSigningRequest builds a DevID signing request using the TPM.
func CreateSigningRequest(rw io.ReadWriter, factory *KeyFactory, platformCN string) (*SigningRequest, *RequestResources, error) {
	if factory == nil {
		factory = DefaultKeyFactory()
	}
	res := &RequestResources{rw: rw}
	var err error
	defer func() {
		if err != nil {
			res.Flush()
		}
	}()

	ekCertData, err := tpm2.NVRead(rw, EKRSACertificateHandle)
	if err != nil {
		return nil, nil, fmt.Errorf("read EK certificate NV: %w", err)
	}
	ekCert, err := parseCertificateLoose(ekCertData)
	if err != nil {
		return nil, nil, fmt.Errorf("parse EK certificate: %w", err)
	}

	ek, err := factory.CreateEK(rw)
	if err != nil {
		return nil, nil, fmt.Errorf("create EK: %w", err)
	}
	res.Endorsement = ek

	ak, err := factory.CreateAK(rw)
	if err != nil {
		return nil, nil, fmt.Errorf("create AK: %w", err)
	}
	res.Attestation = ak

	devID, err := factory.CreateDevID(rw)
	if err != nil {
		return nil, nil, fmt.Errorf("create DevID: %w", err)
	}
	res.DevID = devID

	certifyData, certifySig, err := certifyWithRetry(rw, devID.Handle, ak.Handle)
	if err != nil {
		return nil, nil, err
	}

	sr := &SigningRequest{
		EndorsementCertificate: ekCert,
		EndorsementKey:         &ek.Public,
		AttestationKey:         &ak.Public,
		DevIDKey:               &devID.Public,
		CertifyData:            certifyData,
		CertifySignature:       certifySig,
	}
	if platformCN != "" {
		name := pkix.Name{CommonName: platformCN}
		sr.PlatformIdentity = name.ToRDNSequence()
	}
	return sr, res, nil
}

func certifyWithRetry(rw io.ReadWriter, object, signer tpmutil.Handle) ([]byte, []byte, error) {
	const maxAttempts = 5
	var last error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		data, sig, err := tpm2.Certify(rw, "", "", object, signer, nil)
		if err == nil {
			return data, sig, nil
		}
		last = err
		if !isTPMRetry(err) {
			return nil, nil, fmt.Errorf("Certify: %w", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil, nil, fmt.Errorf("Certify: max retries exceeded: %w", last)
}

func isTPMRetry(err error) bool {
	var warn *tpm2.Warning
	return errors.As(err, &warn) && warn.Code == tpm2.RCRetry
}
