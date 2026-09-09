package devid

import (
	"fmt"
	"io"

	"github.com/google/go-tpm-tools/client"
	"github.com/google/go-tpm/legacy/tpm2"
	"github.com/google/go-tpm/tpmutil"
)

// KeyInfo describes a TPM key (transient or persistent).
type KeyInfo struct {
	Handle      tpmutil.Handle
	Public      tpm2.Public
	PublicBlob  []byte
	PrivateBlob []byte
	Template    tpm2.Public
}

// KeyFactory creates EK/SRK/AK/DevID objects.
type KeyFactory struct {
	EKTemplate    tpm2.Public
	SRKTemplate   tpm2.Public
	AKTemplate    tpm2.Public
	DevIDTemplate tpm2.Public
}

// DefaultKeyFactory returns RSA templates using go-tpm-tools EK/SRK defaults.
func DefaultKeyFactory() *KeyFactory {
	return &KeyFactory{
		EKTemplate:    client.DefaultEKTemplateRSA(),
		SRKTemplate:   client.SRKTemplateRSA(),
		AKTemplate:    DefaultAKTemplateRSA(),
		DevIDTemplate: DefaultDevIDTemplateRSA(),
	}
}

// CreateEK loads or creates a cached EK under the endorsement hierarchy.
func (f *KeyFactory) CreateEK(rw io.ReadWriter) (*KeyInfo, error) {
	return cachedPrimary(rw, f.EKTemplate, tpm2.HandleOwner, tpm2.HandleEndorsement, client.EKReservedHandle)
}

// CreateSRK loads or creates a cached SRK under the owner hierarchy.
func (f *KeyFactory) CreateSRK(rw io.ReadWriter) (*KeyInfo, error) {
	return cachedPrimary(rw, f.SRKTemplate, tpm2.HandleOwner, tpm2.HandleOwner, client.SRKReservedHandle)
}

// CreateAK creates an attestation key under the SRK.
func (f *KeyFactory) CreateAK(rw io.ReadWriter) (*KeyInfo, error) {
	return createUnderSRK(rw, f, f.AKTemplate)
}

// CreateDevID creates a DevID signing key under the SRK.
func (f *KeyFactory) CreateDevID(rw io.ReadWriter) (*KeyInfo, error) {
	return createUnderSRK(rw, f, f.DevIDTemplate)
}

func createUnderSRK(rw io.ReadWriter, f *KeyFactory, template tpm2.Public) (*KeyInfo, error) {
	srk, err := f.CreateSRK(rw)
	if err != nil {
		return nil, err
	}
	// SRK is persistent; do not flush it.
	return createLoadedKey(rw, srk.Handle, template)
}

func cachedPrimary(rw io.ReadWriter, template tpm2.Public, owner, parent, cachedHandle tpmutil.Handle) (*KeyInfo, error) {
	cachedPub, _, _, err := tpm2.ReadPublic(rw, cachedHandle)
	if err == nil {
		if cachedPub.MatchesTemplate(template) {
			pubBlob, err := cachedPub.Encode()
			if err != nil {
				return nil, err
			}
			return &KeyInfo{
				Handle:     cachedHandle,
				Template:   template,
				Public:     cachedPub,
				PublicBlob: pubBlob,
			}, nil
		}
		if err := tpm2.EvictControl(rw, "", owner, cachedHandle, cachedHandle); err != nil {
			return nil, fmt.Errorf("evict stale cached key: %w", err)
		}
	}

	info, err := createPrimaryKey(rw, parent, template)
	if err != nil {
		return nil, err
	}
	defer tpm2.FlushContext(rw, info.Handle)

	if err := tpm2.EvictControl(rw, "", owner, info.Handle, cachedHandle); err != nil {
		return nil, fmt.Errorf("persist primary key: %w", err)
	}
	info.Handle = cachedHandle
	return info, nil
}

func createPrimaryKey(rw io.ReadWriter, hierarchy tpmutil.Handle, template tpm2.Public) (*KeyInfo, error) {
	handle, pubBlob, _, _, _, _, err := tpm2.CreatePrimaryEx(
		rw,
		hierarchy,
		tpm2.PCRSelection{},
		"",
		"",
		template,
	)
	if err != nil {
		return nil, fmt.Errorf("CreatePrimary: %w", err)
	}
	pub, err := tpm2.DecodePublic(pubBlob)
	if err != nil {
		tpm2.FlushContext(rw, handle)
		return nil, fmt.Errorf("decode primary public: %w", err)
	}
	return &KeyInfo{
		Handle:     handle,
		Template:   template,
		Public:     pub,
		PublicBlob: pubBlob,
	}, nil
}

func createLoadedKey(rw io.ReadWriter, parent tpmutil.Handle, template tpm2.Public) (*KeyInfo, error) {
	auth := tpm2.AuthCommand{
		Session:    tpm2.HandlePasswordSession,
		Attributes: tpm2.AttrContinueSession,
	}
	privBlob, pubBlob, _, _, _, err := tpm2.CreateKeyUsingAuth(
		rw,
		parent,
		tpm2.PCRSelection{},
		auth,
		"",
		template,
	)
	if err != nil {
		return nil, fmt.Errorf("CreateKey: %w", err)
	}
	pub, err := tpm2.DecodePublic(pubBlob)
	if err != nil {
		return nil, fmt.Errorf("decode public: %w", err)
	}
	handle, _, err := tpm2.LoadUsingAuth(rw, parent, auth, pubBlob, privBlob)
	if err != nil {
		return nil, fmt.Errorf("Load: %w", err)
	}
	return &KeyInfo{
		Handle:      handle,
		Template:    template,
		Public:      pub,
		PublicBlob:  pubBlob,
		PrivateBlob: privBlob,
	}, nil
}

// FlushTransient flushes a handle if it looks transient (ignores errors).
func FlushTransient(rw io.ReadWriter, h tpmutil.Handle) {
	if h == 0 {
		return
	}
	// Persistent handles live in 0x81xxxxxx; skip them.
	if h >= 0x81000000 && h <= 0x81FFFFFF {
		return
	}
	_ = tpm2.FlushContext(rw, h)
}
