//go:build tpmsim

// These tests run the real TPM commands against go-tpm-tools' in-process TPM
// simulator, behind the same build tag as srk_sim_test.go:
//
//	CGO_CFLAGS=-I<openssl>/include CGO_LDFLAGS=-L<openssl>/lib \
//	  go test -tags tpmsim ./internal/devid/ -run EK -v
//
// They check how the client treats the endorsement key
// (stop-evicting-foreign-objects-at-the-ek-handle, tasks 2.1 and 2.2). The
// simulator is a third TPM implementation beside swtpm/libtpms on a Proxmox host and in
// the lab, so a pass here shows the logic, not those.
package devid

import (
	"bytes"
	"io"
	"testing"

	"github.com/google/go-tpm-tools/client"
	"github.com/google/go-tpm/legacy/tpm2"
	"github.com/google/go-tpm/tpmutil"
)

// persistAt makes a primary key from template in the given hierarchy
// persistent at handle and returns its public area.
func persistAt(t *testing.T, rw io.ReadWriter, hierarchy tpmutil.Handle, template tpm2.Public, at tpmutil.Handle) tpm2.Public {
	t.Helper()
	h, pubBlob, _, _, _, _, err := tpm2.CreatePrimaryEx(rw, hierarchy, tpm2.PCRSelection{}, "", "", template)
	if err != nil {
		t.Fatalf("create primary: %v", err)
	}
	defer tpm2.FlushContext(rw, h)
	if err := tpm2.EvictControl(rw, "", tpm2.HandleOwner, h, at); err != nil {
		t.Fatalf("persist at %#x: %v", at, err)
	}
	pub, err := tpm2.DecodePublic(pubBlob)
	if err != nil {
		t.Fatalf("decode public: %v", err)
	}
	return pub
}

func publicAt(t *testing.T, rw io.ReadWriter, h tpmutil.Handle) []byte {
	t.Helper()
	pub, _, _, err := tpm2.ReadPublic(rw, h)
	if err != nil {
		t.Fatalf("read public at %#x: %v", h, err)
	}
	b, err := pub.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// ekWithUnique is the default EK template with a unique field of 256 bytes of v.
func ekWithUnique(v byte) tpm2.Public {
	tpl := client.DefaultEKTemplateRSA()
	tpl.RSAParameters.ModulusRaw = bytes.Repeat([]byte{v}, 256)
	return tpl
}

// Task 2.1: whatever sits at the reserved EK handle, it is still there
// afterwards, and the EK the client returns is the one the default template
// yields, not the foreign object.
func TestForeignObjectAtEKHandleIsLeftAlone(t *testing.T) {
	cases := []struct {
		name     string
		hier     tpmutil.Handle
		template tpm2.Public
	}{
		{"a storage key from another template", tpm2.HandleOwner, client.SRKTemplateRSA()},
		{"an EK-template key with another unique", tpm2.HandleEndorsement, ekWithUnique(1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sim := newSim(t)
			persistAt(t, sim, tc.hier, tc.template, client.EKReservedHandle)
			before := publicAt(t, sim, client.EKReservedHandle)

			ek, err := DefaultKeyFactory().CreateEK(sim)
			if err != nil {
				t.Fatalf("CreateEK: %v", err)
			}
			defer tpm2.FlushContext(sim, ek.Handle)

			if got := publicAt(t, sim, client.EKReservedHandle); !bytes.Equal(got, before) {
				t.Fatal("the object at 0x81010001 changed")
			}
			if ek.Handle == client.EKReservedHandle {
				t.Fatal("CreateEK returned the persistent handle, not a transient EK")
			}

			want, err := createPrimaryKey(sim, tpm2.HandleEndorsement, client.DefaultEKTemplateRSA())
			if err != nil {
				t.Fatal(err)
			}
			defer tpm2.FlushContext(sim, want.Handle)
			if !bytes.Equal(ek.PublicBlob, want.PublicBlob) {
				t.Fatal("the EK is not the key the default EK template yields")
			}
		})
	}
}

// The derived EK is the same key as a persistent one made from the same
// template, which is how a platform's provisioned EK relates to its certificate.
func TestDerivedEKEqualsPersistentEK(t *testing.T) {
	sim := newSim(t)
	persistent := persistAt(t, sim, tpm2.HandleEndorsement, client.DefaultEKTemplateRSA(), client.EKReservedHandle)
	ek, err := DefaultKeyFactory().CreateEK(sim)
	if err != nil {
		t.Fatal(err)
	}
	defer tpm2.FlushContext(sim, ek.Handle)
	if !bytes.Equal(ek.Public.RSAParameters.ModulusRaw, persistent.RSAParameters.ModulusRaw) {
		t.Fatal("derived EK differs from the persistent EK of the same template")
	}
}

// Task 2.2: the whole flow, the EK for the signing request, the AK and DevID
// key under a transient parent, then credential activation, never needs a
// fourth object slot, and leaves nothing loaded but the keys it created.
func TestEnrollmentFlowFitsThreeObjectSlots(t *testing.T) {
	sim := newSim(t)
	f := DefaultKeyFactory()

	ek, err := f.CreateEK(sim)
	if err != nil {
		t.Fatalf("CreateEK: %v", err)
	}
	tpm2.FlushContext(sim, ek.Handle)
	ek.Handle = 0

	ak, err := f.CreateAK(sim)
	if err != nil {
		t.Fatalf("CreateAK: %v", err)
	}
	devID, err := f.CreateDevID(sim)
	if err != nil {
		t.Fatalf("CreateDevID: %v", err)
	}
	res := &RequestResources{rw: sim, factory: f, Attestation: ak, Endorsement: ek, DevID: devID}
	if got := len(transientHandles(t, sim)); got != 2 {
		t.Fatalf("%d transient objects loaded before activation, want 2 (AK, DevID)", got)
	}

	akName, err := ak.Public.Name()
	if err != nil {
		t.Fatalf("AK name: %v", err)
	}
	blob, secret, nonce, err := CreateChallenge(&ek.Public, akName)
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	got, err := res.Activate(blob, secret)
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if !bytes.Equal(got, nonce) {
		t.Fatal("Activate returned another nonce than the challenge carried")
	}
	if got := len(transientHandles(t, sim)); got != 2 {
		t.Fatalf("%d transient objects loaded after activation, want 2 (AK, DevID)", got)
	}
	res.Flush()
	if got := len(transientHandles(t, sim)); got != 0 {
		t.Fatalf("%d transient objects left after Flush", got)
	}
}
