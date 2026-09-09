package devid

import (
	"fmt"
	"io"

	"github.com/google/go-tpm/legacy/tpm2"
	"github.com/google/go-tpm/tpmutil"
)

// ActivateCredential decrypts a credential blob using AK + EK with an
// endorsement policy session (standard EK authorization).
func ActivateCredential(rw io.ReadWriter, akHandle, ekHandle tpmutil.Handle, credentialBlob, secret []byte) ([]byte, error) {
	session, err := endorsementPolicySession(rw)
	if err != nil {
		return nil, err
	}
	defer tpm2.FlushContext(rw, session)

	out, err := tpm2.ActivateCredentialUsingAuth(
		rw,
		[]tpm2.AuthCommand{
			{Session: tpm2.HandlePasswordSession, Attributes: tpm2.AttrContinueSession},
			{Session: session, Attributes: tpm2.AttrContinueSession},
		},
		akHandle,
		ekHandle,
		credentialBlob,
		secret,
	)
	if err != nil {
		return nil, fmt.Errorf("ActivateCredential: %w", err)
	}
	return out, nil
}

func endorsementPolicySession(rw io.ReadWriter) (tpmutil.Handle, error) {
	var nonceCaller [32]byte
	session, _, err := tpm2.StartAuthSession(
		rw,
		tpm2.HandleNull,
		tpm2.HandleNull,
		nonceCaller[:],
		nil,
		tpm2.SessionPolicy,
		tpm2.AlgNull,
		tpm2.AlgSHA256,
	)
	if err != nil {
		return 0, fmt.Errorf("StartAuthSession: %w", err)
	}
	_, _, err = tpm2.PolicySecret(
		rw,
		tpm2.HandleEndorsement,
		tpm2.AuthCommand{Session: tpm2.HandlePasswordSession, Attributes: tpm2.AttrContinueSession},
		session,
		nil,
		nil,
		nil,
		0,
	)
	if err != nil {
		tpm2.FlushContext(rw, session)
		return 0, fmt.Errorf("PolicySecret: %w", err)
	}
	return session, nil
}
