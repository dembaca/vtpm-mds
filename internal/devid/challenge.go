package devid

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"

	"github.com/google/go-tpm/legacy/tpm2"
	"github.com/google/go-tpm/legacy/tpm2/credactivation"
)

// CreateChallenge wraps a random nonce for akName under ekPub using
// credential activation. The 2-byte TPM2B size prefixes are stripped from
// the returned credential blob and secret (SPIRE / ActivateCredential style).
func CreateChallenge(ekPub *tpm2.Public, akName tpm2.Name) (credentialBlob, secret, nonce []byte, err error) {
	if ekPub == nil {
		return nil, nil, nil, errors.New("missing EK public")
	}
	if akName.Digest == nil {
		return nil, nil, nil, errors.New("missing AK name digest")
	}

	hash, err := ekPub.NameAlg.Hash()
	if err != nil {
		return nil, nil, nil, err
	}
	nonce = make([]byte, hash.Size())
	if _, err = rand.Read(nonce); err != nil {
		return nil, nil, nil, err
	}

	encKey, err := ekPub.Key()
	if err != nil {
		return nil, nil, nil, err
	}
	rsaKey, ok := encKey.(*rsa.PublicKey)
	if !ok {
		return nil, nil, nil, errors.New("only RSA EK is supported")
	}
	if ekPub.RSAParameters == nil || ekPub.RSAParameters.Symmetric == nil {
		return nil, nil, nil, errors.New("EK missing symmetric parameters")
	}
	symBlockSize := int(ekPub.RSAParameters.Symmetric.KeyBits) / 8

	cred, sec, err := credactivation.Generate(akName.Digest, rsaKey, symBlockSize, nonce)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("credactivation.Generate: %w", err)
	}
	if len(cred) < 2 || len(sec) < 2 {
		return nil, nil, nil, errors.New("credential activation blobs too short")
	}
	return cred[2:], sec[2:], nonce, nil
}
