package reconcile

import (
	"reflect"
	"testing"

	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
)

func TestEntryFromProto(t *testing.T) {
	base := func() *types.Entry {
		return &types.Entry{
			Id:        "e1",
			ParentId:  &types.SPIFFEID{TrustDomain: "example.org", Path: "/spire/server"},
			SpiffeId:  &types.SPIFFEID{TrustDomain: "example.org", Path: "/pve/c/vm/350"},
			Selectors: []*types.Selector{{Type: "tpm_devid", Value: "subject:cn:350"}},
		}
	}
	t.Run("only id, parent, spiffe_id and selectors is default", func(t *testing.T) {
		got := entryFromProto(base())
		want := Entry{
			ID:        "e1",
			ParentID:  "spiffe://example.org/spire/server",
			SPIFFEID:  "spiffe://example.org/pve/c/vm/350",
			Selectors: []string{"tpm_devid:subject:cn:350"},
			Default:   true,
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})
	nonDefault := map[string]func(*types.Entry){
		"Admin":         func(e *types.Entry) { e.Admin = true },
		"Downstream":    func(e *types.Entry) { e.Downstream = true },
		"DnsNames":      func(e *types.Entry) { e.DnsNames = []string{"a.example.org"} },
		"FederatesWith": func(e *types.Entry) { e.FederatesWith = []string{"other.org"} },
		"X509SvidTtl":   func(e *types.Entry) { e.X509SvidTtl = 60 },
		"JwtSvidTtl":    func(e *types.Entry) { e.JwtSvidTtl = 60 },
		"Hint":          func(e *types.Entry) { e.Hint = "h" },
		"ExpiresAt":     func(e *types.Entry) { e.ExpiresAt = 1 },
		"StoreSvid":     func(e *types.Entry) { e.StoreSvid = true },
	}
	for name, set := range nonDefault {
		t.Run(name+" alone is not default", func(t *testing.T) {
			e := base()
			set(e)
			if entryFromProto(e).Default {
				t.Errorf("%s set: Default = true", name)
			}
		})
	}
	managed := map[string]func(*types.Entry){
		"CreatedAt":      func(e *types.Entry) { e.CreatedAt = 1 },
		"RevisionNumber": func(e *types.Entry) { e.RevisionNumber = 3 },
	}
	for name, set := range managed {
		t.Run(name+" alone stays default", func(t *testing.T) {
			e := base()
			set(e)
			if !entryFromProto(e).Default {
				t.Errorf("%s set: Default = false", name)
			}
		})
	}
}
