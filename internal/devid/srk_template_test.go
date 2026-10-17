package devid

import (
	"reflect"
	"testing"

	"github.com/google/go-tpm-tools/client"
	"github.com/google/go-tpm/legacy/tpm2"
)

// spireHighRSA rebuilds SPIRE's SRKTemplateHighRSA from its source rather than
// from our helper, so the comparison below is not a template tested against
// itself. Source: spiffe/spire, pkg/agent/plugin/nodeattestor/tpmdevid/tpmutil/
// signingkey.go, SRKTemplateHighRSA, identical at v1.14.0, v1.15.4 and main:
// client.SRKTemplateRSA() with RSAParameters.ModulusRaw = []byte{}.
func spireHighRSA() tpm2.Public {
	t := client.SRKTemplateRSA()
	t.RSAParameters.ModulusRaw = []byte{}
	return t
}

// TestSRKTemplateIsSPIREHighTemplate pins the parent key the client creates its
// DevID and attestation keys under to the one SPIRE derives, per task 2.1 of
// create-devid-under-the-srk-spire-loads. SPIRE only loads a DevID created
// under that primary.
func TestSRKTemplateIsSPIREHighTemplate(t *testing.T) {
	want := spireHighRSA()
	if m := want.RSAParameters.ModulusRaw; m == nil || len(m) != 0 {
		t.Fatalf("SPIRE's ModulusRaw must be an empty non-nil slice, got %#v", m)
	}

	if got := SRKTemplateHighRSA(); !reflect.DeepEqual(got, want) {
		t.Errorf("SRKTemplateHighRSA() differs from SPIRE's template:\n got %+v\nwant %+v", got, want)
	}
	if got := DefaultKeyFactory().SRKTemplate; !reflect.DeepEqual(got, want) {
		t.Errorf("DefaultKeyFactory().SRKTemplate is not SPIRE's template:\n got %+v\nwant %+v", got, want)
	}
}

// TestSRKTemplateIsNotTheGoTPMToolsDefault guards the defect itself: the
// go-tpm-tools template carries 256 zero bytes of unique, and a parent made from
// it is a different key from SPIRE's.
func TestSRKTemplateIsNotTheGoTPMToolsDefault(t *testing.T) {
	stock := client.SRKTemplateRSA()
	if len(stock.RSAParameters.ModulusRaw) != 256 {
		t.Fatalf("premise changed: go-tpm-tools' SRK template has %d bytes of unique, expected 256",
			len(stock.RSAParameters.ModulusRaw))
	}
	if got := DefaultKeyFactory().SRKTemplate; reflect.DeepEqual(got, stock) {
		t.Error("the client's SRK template equals go-tpm-tools' default, which SPIRE cannot load under")
	}
}
