//go:build tpmsim

// These tests run the real TPM commands against go-tpm-tools' in-process TPM
// simulator (the Microsoft reference implementation). They need cgo and OpenSSL
// headers, so they sit behind a build tag and plain `go test ./...` skips them:
//
//	CGO_CFLAGS=-I<openssl>/include CGO_LDFLAGS=-L<openssl>/lib \
//	  go test -tags tpmsim ./internal/devid/ -run SRK -v
//
// They check the parent key the client creates its keys under
// (create-devid-under-the-srk-spire-loads, tasks 2.1 to 2.3). The simulator is a
// third TPM implementation beside swtpm/libtpms on hogan and in the lab, so a
// pass here shows nothing about those; it shows the logic.
package devid

import (
	"bytes"
	"testing"

	"github.com/google/go-tpm-tools/client"
	"github.com/google/go-tpm-tools/simulator"
	"github.com/google/go-tpm/legacy/tpm2"
	"github.com/google/go-tpm/tpmutil"
)

func newSim(t *testing.T) *simulator.Simulator {
	t.Helper()
	sim, err := simulator.Get()
	if err != nil {
		t.Fatalf("start TPM simulator: %v", err)
	}
	t.Cleanup(func() { sim.Close() })
	return sim
}

// loadsUnder reports whether the DevID blobs load under a primary key created
// in the owner hierarchy from the given template, the way SPIRE's tpm_devid
// attestor loads them (tpmutil.Session.loadKey: CreatePrimaryEx, then Load).
func loadsUnder(t *testing.T, rw *simulator.Simulator, template tpm2.Public, key *KeyInfo) error {
	t.Helper()
	parent, _, _, _, _, _, err := tpm2.CreatePrimaryEx(rw, tpm2.HandleOwner, tpm2.PCRSelection{}, "", "", template)
	if err != nil {
		t.Fatalf("create primary: %v", err)
	}
	defer tpm2.FlushContext(rw, parent)
	handle, _, err := tpm2.Load(rw, parent, "", key.PublicBlob, key.PrivateBlob)
	if err == nil {
		tpm2.FlushContext(rw, handle)
	}
	return err
}

// uniqueTemplate is the H-1 template with a unique field of n bytes of value v.
func uniqueTemplate(n int, v byte) tpm2.Public {
	tpl := SRKTemplateHighRSA()
	tpl.RSAParameters.ModulusRaw = bytes.Repeat([]byte{v}, n)
	return tpl
}

func transientHandles(t *testing.T, rw *simulator.Simulator) []tpmutil.Handle {
	t.Helper()
	vals, _, err := tpm2.GetCapability(rw, tpm2.CapabilityHandles, 32, uint32(tpm2.HandleTypeTransient)<<24)
	if err != nil {
		t.Fatalf("get handles: %v", err)
	}
	var out []tpmutil.Handle
	for _, v := range vals {
		out = append(out, v.(tpmutil.Handle))
	}
	return out
}

// Task 2.3: the DevID loads under the H-1 parent, as SPIRE loads it, and the
// negative control, a parent whose unique certainly differs, does not.
func TestDevIDLoadsUnderSPIREParent(t *testing.T) {
	rw := newSim(t)
	f := DefaultKeyFactory()

	devID, err := f.CreateDevID(rw)
	if err != nil {
		t.Fatal(err)
	}
	ak, err := f.CreateAK(rw)
	if err != nil {
		t.Fatal(err)
	}
	// The simulator holds few objects at once. Only the blobs matter from here,
	// so unload the keys before loading others, as SPIRE does on a fresh TPM.
	FlushTransient(rw, devID.Handle)
	FlushTransient(rw, ak.Handle)

	if err := loadsUnder(t, rw, SRKTemplateHighRSA(), devID); err != nil {
		t.Errorf("DevID does not load under SPIRE's H-1 parent: %v", err)
	}
	if err := loadsUnder(t, rw, SRKTemplateHighRSA(), ak); err != nil {
		t.Errorf("AK does not load under SPIRE's H-1 parent: %v", err)
	}

	// Negative control: 256 bytes of 0x01 is a unique that differs on every TPM.
	if err := loadsUnder(t, rw, uniqueTemplate(256, 0x01), devID); err == nil {
		t.Error("DevID also loads under a parent with a different unique: the test proves nothing")
	}
}

// A data point, not an assertion: whether this simulator tells the go-tpm-tools
// 256-zero-byte template from SPIRE's empty one. On the cloud lab's TPM and on
// hogan's, through go-tpm, it is a different parent.
func TestStockTemplateParentLogged(t *testing.T) {
	rw := newSim(t)
	f := DefaultKeyFactory()
	devID, err := f.CreateDevID(rw)
	if err != nil {
		t.Fatal(err)
	}
	FlushTransient(rw, devID.Handle)

	err = loadsUnder(t, rw, client.SRKTemplateRSA(), devID)
	t.Logf("DevID created under SPIRE's parent, loaded under go-tpm-tools' 256-zero-byte template: err=%v", err)
}

// Task 2.2: a persistent object at the reserved SRK handle is left alone, and
// the DevID still loads under the H-1 parent.
func TestPersistentObjectAtSRKHandleIsLeftAlone(t *testing.T) {
	rw := newSim(t)
	f := DefaultKeyFactory()

	// Persist a storage key whose unique differs from SPIRE's. The old code
	// treated it as a match, because MatchesTemplate ignores ModulusRaw, reused it
	// as the parent, and made a DevID SPIRE cannot load.
	other, err := createPrimaryKey(rw, tpm2.HandleOwner, uniqueTemplate(256, 0x01))
	if err != nil {
		t.Fatal(err)
	}
	if err := tpm2.EvictControl(rw, "", tpm2.HandleOwner, other.Handle, client.SRKReservedHandle); err != nil {
		t.Fatalf("persist at 0x%x: %v", uint32(client.SRKReservedHandle), err)
	}
	tpm2.FlushContext(rw, other.Handle)
	before, _, _, err := tpm2.ReadPublic(rw, client.SRKReservedHandle)
	if err != nil {
		t.Fatal(err)
	}
	beforeBlob, err := before.Encode()
	if err != nil {
		t.Fatal(err)
	}

	devID, err := f.CreateDevID(rw)
	if err != nil {
		t.Fatal(err)
	}
	ak, err := f.CreateAK(rw)
	if err != nil {
		t.Fatal(err)
	}
	FlushTransient(rw, devID.Handle)
	FlushTransient(rw, ak.Handle)

	after, _, _, err := tpm2.ReadPublic(rw, client.SRKReservedHandle)
	if err != nil {
		t.Fatalf("the persistent object at 0x%x is gone: %v", uint32(client.SRKReservedHandle), err)
	}
	afterBlob, err := after.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(beforeBlob, afterBlob) {
		t.Error("the persistent object at the SRK handle changed")
	}
	if err := loadsUnder(t, rw, SRKTemplateHighRSA(), devID); err != nil {
		t.Errorf("DevID does not load under SPIRE's parent when a foreign object sits at the SRK handle: %v", err)
	}
}

// The parent is transient and flushed: only the loaded keys remain, and they go
// when the caller flushes them.
func TestParentKeyIsFlushed(t *testing.T) {
	rw := newSim(t)
	f := DefaultKeyFactory()
	devID, err := f.CreateDevID(rw)
	if err != nil {
		t.Fatal(err)
	}
	ak, err := f.CreateAK(rw)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(transientHandles(t, rw)); got != 2 {
		t.Errorf("expected only the DevID and the AK to stay loaded, found %d transient handles", got)
	}
	FlushTransient(rw, devID.Handle)
	FlushTransient(rw, ak.Handle)
	if got := transientHandles(t, rw); len(got) != 0 {
		t.Errorf("transient handles left after flushing the keys: %v", got)
	}
}
