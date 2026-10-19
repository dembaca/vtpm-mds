package devid

import (
	"errors"
	"fmt"
	"io"

	"github.com/google/go-tpm-tools/client"
	"github.com/google/go-tpm/legacy/tpm2"
	"github.com/google/go-tpm/tpmutil"
)

// ErrDevIDLoad means existing DevID blobs could not be loaded under the
// SPIRE H-1 parent. Callers may fall back to creating a new DevID key.
var ErrDevIDLoad = errors.New("load existing DevID")

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

// DefaultKeyFactory returns RSA templates: the go-tpm-tools default for the EK,
// and for the SRK the H-1 template SPIRE derives its parent from, see
// SRKTemplateHighRSA.
func DefaultKeyFactory() *KeyFactory {
	return &KeyFactory{
		EKTemplate:    client.DefaultEKTemplateRSA(),
		SRKTemplate:   SRKTemplateHighRSA(),
		AKTemplate:    DefaultAKTemplateRSA(),
		DevIDTemplate: DefaultDevIDTemplateRSA(),
	}
}

// CreateEK derives the endorsement key as a transient primary key in the
// endorsement hierarchy. The caller flushes it. It never makes the key
// persistent and never reads, reuses, replaces or evicts a persistent object at
// client.EKReservedHandle or any other handle: the EK certificate certifies
// the key this template yields on this TPM, so a derived EK is the right one
// whatever else sits at the reserved handle, and deleting that object could
// remove a key that belongs to other software or to the platform.
func (f *KeyFactory) CreateEK(rw io.ReadWriter) (*KeyInfo, error) {
	return createPrimaryKey(rw, tpm2.HandleEndorsement, f.EKTemplate)
}

// CreateSRK derives the storage root key as a transient primary key in the
// owner hierarchy. The caller flushes it. It never makes the key persistent and
// never reads, reuses, replaces or evicts a persistent object at
// client.SRKReservedHandle or any other handle: SPIRE derives its parent afresh
// each time, so a persistent SRK is irrelevant to loading, and deleting one
// would remove a key that may belong to other software on the TPM.
func (f *KeyFactory) CreateSRK(rw io.ReadWriter) (*KeyInfo, error) {
	return createPrimaryKey(rw, tpm2.HandleOwner, f.SRKTemplate)
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
	// The parent is transient. Flushing it does not unload the child, which
	// the caller flushes with the rest of its resources.
	defer tpm2.FlushContext(rw, srk.Handle)
	return createLoadedKey(rw, srk.Handle, template)
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

// LoadDevID loads an existing DevID key under a freshly derived SRK parent.
func (f *KeyFactory) LoadDevID(rw io.ReadWriter, pubBlob, privBlob []byte) (*KeyInfo, error) {
	if len(pubBlob) == 0 || len(privBlob) == 0 {
		return nil, fmt.Errorf("%w: missing blobs", ErrDevIDLoad)
	}
	srk, err := f.CreateSRK(rw)
	if err != nil {
		return nil, err
	}
	defer tpm2.FlushContext(rw, srk.Handle)
	auth := tpm2.AuthCommand{
		Session:    tpm2.HandlePasswordSession,
		Attributes: tpm2.AttrContinueSession,
	}
	pub, err := tpm2.DecodePublic(pubBlob)
	if err != nil {
		return nil, fmt.Errorf("%w: decode public: %v", ErrDevIDLoad, err)
	}
	handle, _, err := tpm2.LoadUsingAuth(rw, srk.Handle, auth, pubBlob, privBlob)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDevIDLoad, err)
	}
	return &KeyInfo{
		Handle:      handle,
		Template:    f.DevIDTemplate,
		Public:      pub,
		PublicBlob:  pubBlob,
		PrivateBlob: privBlob,
	}, nil
}
