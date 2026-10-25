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
// SPIRE H-1 parent for a reason that says nothing about whose they are: the
// parent could not be derived, a TPM warning such as TPM_RC_RETRY or
// TPM_RC_OBJECT_MEMORY, another TPM error, or the transport. Callers must not
// replace the key on it; the next run tries again.
var ErrDevIDLoad = errors.New("load existing DevID")

// ErrDevIDForeign means existing DevID blobs are not this TPM's: a blob does
// not decode, or TPM2_Load under the derived H-1 parent answers with
// TPM_RC_INTEGRITY, TPM_RC_BINDING or TPM_RC_SENSITIVE. Only then may a caller
// create a new DevID key in their place
// (renew-when-the-devid-key-is-not-this-tpms D-c).
var ErrDevIDForeign = errors.New("DevID blobs are foreign to this TPM")

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
// Blobs that are not this TPM's fail with ErrDevIDForeign, every other failure
// with ErrDevIDLoad or the parent's error; both wrap the cause.
func (f *KeyFactory) LoadDevID(rw io.ReadWriter, pubBlob, privBlob []byte) (*KeyInfo, error) {
	if len(pubBlob) == 0 || len(privBlob) == 0 {
		return nil, fmt.Errorf("%w: missing blobs", ErrDevIDLoad)
	}
	// Decoded before the parent is derived: a blob that does not decode is
	// foreign without asking the TPM.
	pub, err := tpm2.DecodePublic(pubBlob)
	if err != nil {
		return nil, fmt.Errorf("%w: decode public: %w", ErrDevIDForeign, err)
	}
	srk, err := f.CreateSRK(rw)
	if err != nil {
		return nil, fmt.Errorf("%w: derive parent: %w", ErrDevIDLoad, err)
	}
	defer tpm2.FlushContext(rw, srk.Handle)
	auth := tpm2.AuthCommand{
		Session:    tpm2.HandlePasswordSession,
		Attributes: tpm2.AttrContinueSession,
	}
	handle, _, err := tpm2.LoadUsingAuth(rw, srk.Handle, auth, pubBlob, privBlob)
	if err != nil {
		if isForeignLoadError(err) {
			return nil, fmt.Errorf("%w: %w", ErrDevIDForeign, err)
		}
		return nil, fmt.Errorf("%w: %w", ErrDevIDLoad, err)
	}
	return &KeyInfo{
		Handle:      handle,
		Template:    f.DevIDTemplate,
		Public:      pub,
		PublicBlob:  pubBlob,
		PrivateBlob: privBlob,
	}, nil
}

// isForeignLoadError reports whether a TPM2_Load error says the blobs were not
// made under this parent: the private area's HMAC fails (TPM_RC_INTEGRITY), the
// public and sensitive areas are not bound (TPM_RC_BINDING), both format-1
// parameter errors, or the sensitive area does not unmarshal after decryption
// (TPM_RC_SENSITIVE, a format-0 error). go-tpm's legacy package returns these
// as values; pointers are matched too, so a wrapper that returns one is not
// silently read as "not foreign".
func isForeignLoadError(err error) bool {
	var pv tpm2.ParameterError
	var pp *tpm2.ParameterError
	var code tpm2.RCFmt1
	switch {
	case errors.As(err, &pv):
		code = pv.Code
	case errors.As(err, &pp) && pp != nil:
		code = pp.Code
	default:
		var ev tpm2.Error
		var ep *tpm2.Error
		switch {
		case errors.As(err, &ev):
			return ev.Code == tpm2.RCSensitive
		case errors.As(err, &ep) && ep != nil:
			return ep.Code == tpm2.RCSensitive
		}
		return false
	}
	return code == tpm2.RCIntegrity || code == tpm2.RCBinding
}
