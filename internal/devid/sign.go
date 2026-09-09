package devid

import (
	"fmt"
	"io"

	"github.com/google/go-tpm/legacy/tpm2"
	"github.com/google/go-tpm/tpmutil"
	"golang.org/x/crypto/cryptobyte"
	"golang.org/x/crypto/cryptobyte/asn1"
)

const maxDigestBuffer = 1024

// SignatureScheme returns the signing scheme for a TPM public area.
func SignatureScheme(pub tpm2.Public) (*tpm2.SigScheme, error) {
	if pub.Attributes&tpm2.FlagSign == 0 {
		return nil, fmt.Errorf("not a signing key")
	}
	switch pub.Type {
	case tpm2.AlgRSA:
		if pub.RSAParameters == nil {
			return nil, fmt.Errorf("malformed RSA key")
		}
		return pub.RSAParameters.Sign, nil
	case tpm2.AlgECC:
		if pub.ECCParameters == nil {
			return nil, fmt.Errorf("malformed ECC key")
		}
		return pub.ECCParameters.Sign, nil
	default:
		return nil, fmt.Errorf("unsupported key type 0x%04x", pub.Type)
	}
}

// HashAndSign hashes data in the TPM and signs with keyHandle.
func HashAndSign(rw io.ReadWriter, hierarchy, keyHandle tpmutil.Handle, data []byte) ([]byte, error) {
	pub, _, _, err := tpm2.ReadPublic(rw, keyHandle)
	if err != nil {
		return nil, fmt.Errorf("ReadPublic: %w", err)
	}
	scheme, err := SignatureScheme(pub)
	if err != nil {
		return nil, err
	}
	if scheme == nil {
		return nil, fmt.Errorf("missing signature scheme")
	}
	digest, ticket, err := tpmHash(rw, hierarchy, scheme.Hash, data)
	if err != nil {
		return nil, err
	}
	sig, err := tpm2.Sign(rw, keyHandle, "", digest, ticket, scheme)
	if err != nil {
		return nil, fmt.Errorf("Sign: %w", err)
	}
	return signatureBytes(sig)
}

func tpmHash(rw io.ReadWriter, hierarchy tpmutil.Handle, hashAlg tpm2.Algorithm, data []byte) ([]byte, *tpm2.Ticket, error) {
	if len(data) <= maxDigestBuffer {
		digest, validation, err := tpm2.Hash(rw, hashAlg, data, hierarchy)
		if err != nil {
			return nil, nil, fmt.Errorf("Hash: %w", err)
		}
		return digest, validation, nil
	}

	seq, err := tpm2.HashSequenceStart(rw, "", hashAlg)
	if err != nil {
		return nil, nil, fmt.Errorf("HashSequenceStart: %w", err)
	}
	defer tpm2.FlushContext(rw, seq)

	for len(data) > maxDigestBuffer {
		if err := tpm2.SequenceUpdate(rw, "", seq, data[:maxDigestBuffer]); err != nil {
			return nil, nil, fmt.Errorf("SequenceUpdate: %w", err)
		}
		data = data[maxDigestBuffer:]
	}
	digest, validation, err := tpm2.SequenceComplete(rw, "", seq, hierarchy, data)
	if err != nil {
		return nil, nil, fmt.Errorf("SequenceComplete: %w", err)
	}
	return digest, validation, nil
}

func signatureBytes(sig *tpm2.Signature) ([]byte, error) {
	switch sig.Alg {
	case tpm2.AlgRSASSA, tpm2.AlgRSAPSS:
		return sig.RSA.Signature, nil
	case tpm2.AlgECDSA:
		var b cryptobyte.Builder
		b.AddASN1(asn1.SEQUENCE, func(b *cryptobyte.Builder) {
			b.AddASN1BigInt(sig.ECC.R)
			b.AddASN1BigInt(sig.ECC.S)
		})
		return b.Bytes()
	default:
		return nil, fmt.Errorf("unsupported signature algorithm 0x%04x", sig.Alg)
	}
}
