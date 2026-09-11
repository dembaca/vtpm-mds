package devid

import (
	"bytes"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Enroller is the server-side DevID enrollment orchestrator.
type Enroller struct {
	CA       *CA
	EKRoots  *x509.CertPool
	Sessions *SessionStore
}

// NewEnroller constructs an Enroller. sessions may be nil (a default is created).
func NewEnroller(ca *CA, ekRoots *x509.CertPool, sessions *SessionStore) *Enroller {
	if sessions == nil {
		sessions = NewSessionStore(5 * time.Minute)
	}
	return &Enroller{CA: ca, EKRoots: ekRoots, Sessions: sessions}
}

// StartResult is returned after verifying a signing request and creating a challenge.
type StartResult struct {
	SessionID      string
	CredentialBlob []byte
	Secret         []byte
}

// Start verifies the signing request + signature, creates a credential challenge,
// and stores an enroll session. subjectCN is used when the CSR has no CN.
// vmid and ekFP bind the session to the MAC-identified VM and EK certificate.
func (e *Enroller) Start(requestData, signature []byte, subjectCN, vmid, ekFP string) (*StartResult, error) {
	if e == nil || e.CA == nil {
		return nil, errors.New("enroller not configured")
	}
	var sr SigningRequest
	if err := sr.UnmarshalBinary(requestData); err != nil {
		return nil, fmt.Errorf("decode signing request: %w", err)
	}
	if err := CheckSignature(sr.DevIDKey, requestData, signature); err != nil {
		return nil, fmt.Errorf("invalid request signature: %w", err)
	}
	if err := VerifyEKCertificate(e.EKRoots, sr.EndorsementKey, sr.EndorsementCertificate); err != nil {
		return nil, err
	}
	if err := VerifyDevIDResidency(sr.AttestationKey, sr.DevIDKey, sr.CertifyData, sr.CertifySignature); err != nil {
		return nil, fmt.Errorf("DevID residency: %w", err)
	}
	if err := CheckDevIDProp(sr.DevIDKey.Attributes); err != nil {
		return nil, err
	}
	if err := CheckAKProp(sr.AttestationKey.Attributes); err != nil {
		return nil, err
	}
	if ekFP != "" && sr.EndorsementCertificate != nil {
		if fp := EKFingerprint(sr.EndorsementCertificate); !strings.EqualFold(fp, ekFP) {
			return nil, errors.New("EK fingerprint does not match authenticated certificate")
		}
	}

	akName, err := sr.AttestationKey.Name()
	if err != nil {
		return nil, err
	}
	cred, secret, nonce, err := CreateChallenge(sr.EndorsementKey, akName)
	if err != nil {
		return nil, fmt.Errorf("create challenge: %w", err)
	}

	cn := subjectCNFromRequest(&sr, subjectCN)
	sid, err := e.Sessions.Put(nonce, sr, cn, vmid, ekFP)
	if err != nil {
		return nil, err
	}
	return &StartResult{
		SessionID:      sid,
		CredentialBlob: cred,
		Secret:         secret,
	}, nil
}

func subjectCNFromRequest(sr *SigningRequest, fallback string) string {
	if len(sr.PlatformIdentity) > 0 {
		var name pkix.Name
		name.FillFromRDNSequence(&sr.PlatformIdentity)
		if name.CommonName != "" {
			return name.CommonName
		}
	}
	return fallback
}

// Finish completes enrollment if challengeResponse matches the stored nonce.
// vmid and ekFP must match the values bound at Start (MAC + EK auth).
func (e *Enroller) Finish(sessionID string, challengeResponse []byte, vmid, ekFP string) ([]byte, error) {
	if e == nil || e.CA == nil {
		return nil, errors.New("enroller not configured")
	}
	sess, err := e.Sessions.Take(sessionID)
	if err != nil {
		return nil, err
	}
	if sess.VMID != "" && sess.VMID != vmid {
		return nil, errors.New("VM identity does not match enroll session")
	}
	if sess.EKFingerprint != "" && !strings.EqualFold(sess.EKFingerprint, ekFP) {
		return nil, errors.New("EK certificate does not match enroll session")
	}
	if !bytes.Equal(sess.Nonce, challengeResponse) {
		return nil, errors.New("challenge verification failed")
	}
	cert, err := e.CA.IssueDevID(&sess.Request, sess.SubjectCN, time.Now().UTC())
	if err != nil {
		return nil, fmt.Errorf("issue DevID: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}), nil
}
