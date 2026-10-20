package devid

import (
	"crypto"
	"crypto/rand"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"time"
)

// OIDVerifiedTPMFixed is tcg-cap-verifiedTPMFixed (2.23.133.11.1.2).
var OIDVerifiedTPMFixed = asn1.ObjectIdentifier{2, 23, 133, 11, 1, 2}

// DefaultDevIDLifetime is ninety days — the built-in mds.devid_lifetime.
const DefaultDevIDLifetime = 2160 * time.Hour

// CA issues LDevID certificates.
type CA struct {
	cert *x509.Certificate
	key  crypto.PrivateKey
	// Lifetime is notAfter - notBefore for IssueDevID. Zero means DefaultDevIDLifetime.
	Lifetime time.Duration
}

// LoadCA loads a PEM certificate and PKCS#8 (or PKCS#1) private key from disk.
func LoadCA(certPath, keyPath string) (*CA, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("read DevID CA cert: %w", err)
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read DevID CA key: %w", err)
	}
	return ParseCA(certPEM, keyPEM)
}

// ParseCA parses PEM-encoded CA certificate and private key material.
func ParseCA(certPEM, keyPEM []byte) (*CA, error) {
	var cert *x509.Certificate
	var key crypto.PrivateKey

	rest := certPEM
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		cert = c
	}
	if cert == nil {
		return nil, errors.New("no certificate in CA PEM")
	}

	rest = keyPEM
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		switch block.Type {
		case "PRIVATE KEY":
			k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return nil, err
			}
			key = k
		case "RSA PRIVATE KEY":
			k, err := x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				return nil, err
			}
			key = k
		}
	}
	if key == nil {
		return nil, errors.New("no private key in CA PEM")
	}
	return &CA{cert: cert, key: key}, nil
}

// Certificate returns the CA certificate.
func (ca *CA) Certificate() *x509.Certificate {
	return ca.cert
}

// Sign issues a certificate from template (serial generated if unset).
func (ca *CA) Sign(template *x509.Certificate) ([]byte, error) {
	if template.PublicKey == nil {
		return nil, errors.New("missing public key")
	}
	sn, err := ca.uniqueSerial(template)
	if err != nil {
		return nil, err
	}
	template.SerialNumber = sn
	return x509.CreateCertificate(rand.Reader, template, ca.cert, template.PublicKey, ca.key)
}

func (ca *CA) uniqueSerial(template *x509.Certificate) (*big.Int, error) {
	pubBytes, err := x509.MarshalPKIXPublicKey(template.PublicKey)
	if err != nil {
		return nil, err
	}
	subjData, err := asn1.Marshal(template.Subject.ToRDNSequence())
	if err != nil {
		return nil, err
	}
	h := sha1.New()
	h.Write(ca.cert.Raw)
	h.Write(subjData)
	h.Write(pubBytes)
	h.Write([]byte(template.NotBefore.UTC().Format(time.RFC3339)))
	h.Write([]byte(template.NotAfter.UTC().Format(time.RFC3339)))
	if template.SerialNumber != nil {
		h.Write(template.SerialNumber.Bytes())
	}
	return new(big.Int).SetBytes(h.Sum(nil)), nil
}

// IssueDevID builds and signs an LDevID certificate for sr.
// subjectCN is the authenticated VM ID and is the whole subject; nothing from
// the platform identity in sr reaches the certificate.
func (ca *CA) IssueDevID(sr *SigningRequest, subjectCN string, notBefore time.Time) (*x509.Certificate, error) {
	if sr == nil || sr.DevIDKey == nil {
		return nil, errors.New("missing DevID key")
	}
	pub, err := sr.DevIDKey.Key()
	if err != nil {
		return nil, err
	}

	subj := pkix.Name{CommonName: subjectCN}

	subjectIsEmpty := len(subj.ToRDNSequence()) == 0
	sanExt, err := buildDevIDSAN(subjectIsEmpty, sr)
	if err != nil {
		return nil, err
	}

	if notBefore.IsZero() {
		notBefore = time.Now().UTC()
	}
	notBefore = notBefore.UTC().Truncate(time.Second)
	lifetime := ca.Lifetime
	if lifetime <= 0 {
		lifetime = DefaultDevIDLifetime
	}
	notAfter := notBefore.Add(lifetime)

	template := &x509.Certificate{
		PublicKey:             pub,
		Subject:               subj,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  false,
		UnknownExtKeyUsage:    []asn1.ObjectIdentifier{OIDVerifiedTPMFixed},
		ExtraExtensions:       []pkix.Extension{sanExt},
	}

	der, err := ca.Sign(template)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(der)
}

func buildDevIDSAN(subjectIsEmpty bool, sr *SigningRequest) (pkix.Extension, error) {
	if sr.EndorsementKey != nil {
		ekPub, err := sr.EndorsementKey.Key()
		if err == nil {
			ext, err := DevIDSANFromEKPublicKey(subjectIsEmpty, ekPub)
			if err == nil {
				return ext, nil
			}
		}
	}
	if sr.EndorsementCertificate != nil {
		return DevIDSANFromEKCertificate(subjectIsEmpty, sr.EndorsementCertificate)
	}
	return pkix.Extension{}, errors.New("unable to build DevID SAN")
}
