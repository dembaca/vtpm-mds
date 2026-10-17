package devid

import (
	"github.com/google/go-tpm-tools/client"
	"github.com/google/go-tpm/legacy/tpm2"
)

// TPM object attribute sets matching TCG DevID guidance (AK restricted+sign;
// DevID sign-only, not restricted/decrypt).
const (
	AKAttributes = tpm2.FlagSign |
		tpm2.FlagRestricted |
		tpm2.FlagFixedTPM |
		tpm2.FlagFixedParent |
		tpm2.FlagSensitiveDataOrigin |
		tpm2.FlagUserWithAuth

	DevIDAttributes = tpm2.FlagSign |
		tpm2.FlagFixedTPM |
		tpm2.FlagFixedParent |
		tpm2.FlagSensitiveDataOrigin |
		tpm2.FlagUserWithAuth
)

// DefaultAKTemplateRSA returns an RSA attestation key template.
func DefaultAKTemplateRSA() tpm2.Public {
	return tpm2.Public{
		Type:       tpm2.AlgRSA,
		NameAlg:    tpm2.AlgSHA256,
		Attributes: AKAttributes,
		RSAParameters: &tpm2.RSAParams{
			Sign: &tpm2.SigScheme{
				Alg:  tpm2.AlgRSASSA,
				Hash: tpm2.AlgSHA256,
			},
			KeyBits:    2048,
			ModulusRaw: make([]byte, 256),
		},
	}
}

// DefaultDevIDTemplateRSA returns an RSA DevID signing key template.
func DefaultDevIDTemplateRSA() tpm2.Public {
	return tpm2.Public{
		Type:       tpm2.AlgRSA,
		NameAlg:    tpm2.AlgSHA256,
		Attributes: DevIDAttributes,
		RSAParameters: &tpm2.RSAParams{
			Sign: &tpm2.SigScheme{
				Alg:  tpm2.AlgRSASSA,
				Hash: tpm2.AlgSHA256,
			},
			KeyBits:    2048,
			ModulusRaw: make([]byte, 256),
		},
	}
}

// SRKTemplateHighRSA returns the storage root key template that SPIRE's
// tpm_devid attestor derives its parent key from: the TCG "H-1" template, which
// is go-tpm-tools' RSA storage-key template with an empty unique field
// (ModulusRaw), not the 256 zero bytes client.SRKTemplateRSA carries.
//
// A DevID key only loads under the primary it was created under, and SPIRE
// creates this primary afresh every time it loads (pkg/agent/plugin/
// nodeattestor/tpmdevid/tpmutil, SRKTemplateHighRSA, at SPIRE v1.14.0,
// v1.15.4 and main). The client therefore has to create its DevID and
// attestation keys under the same one.
//
// ModulusRaw is []byte{}, not nil, exactly as SPIRE writes it.
func SRKTemplateHighRSA() tpm2.Public {
	template := client.SRKTemplateRSA()
	template.RSAParameters.ModulusRaw = []byte{}
	return template
}
