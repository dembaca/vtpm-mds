package devid

import (
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
