package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dembaca/vtpm-mds/internal/inventory"
)

// placement/region is a registered key served with its value, not answered by
// the index handler as an unknown key under /latest/meta-data/ was before
// (serve-zone-and-region-per-vm, task 2.4). The YAML backend's host placement
// is the old constants.
func TestPlacementRegionIsARegisteredKey(t *testing.T) {
	srv, token := signatureTestServer(t)
	req := httptest.NewRequest("GET", "/latest/meta-data/placement/region", nil)
	req.RemoteAddr = "10.0.0.5:5000"
	req.Header.Set("X-Aws-Ec2-Metadata-Token", token)

	got := serve(srv, req)
	if got.status != http.StatusOK || got.body != "local" {
		t.Fatalf("got %d %q, want 200 \"local\"", got.status, got.body)
	}
	if strings.Contains(got.body, "\n") || got.contentType != "text/plain" {
		t.Fatalf("got body %q, Content-Type %q; want the bare value as text/plain", got.body, got.contentType)
	}
}

// A successful load sets the host placement for its backend, so a SIGHUP picks
// up a changed one; the YAML backend's is the default.
func TestASuccessfulLoadSetsTheHostPlacement(t *testing.T) {
	srv, _ := signatureTestServer(t)
	inventory.SetHostPlacement(inventory.Placement{Zone: "stale", Region: "stale"})
	t.Cleanup(func() { inventory.SetHostPlacement(inventory.DefaultPlacement) })

	if err := srv.RefreshVMConfigCache(); err != nil {
		t.Fatalf("RefreshVMConfigCache: %v", err)
	}
	if got := inventory.HostPlacement(); got != inventory.DefaultPlacement {
		t.Fatalf("got %+v after a YAML load, want %+v", got, inventory.DefaultPlacement)
	}
}
