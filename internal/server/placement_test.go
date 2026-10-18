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

// The three tags paths are registered, so an unbound caller gets the empty 404
// of a tags path and never the metadata index, which answers every unregistered
// path under /latest/meta-data/ (serve-vm-tags-as-instance-metadata, task 2.4).
func TestTagsPathsAreRegisteredNotTheIndex(t *testing.T) {
	srv, token := signatureTestServer(t)
	for _, path := range []string{
		"/latest/meta-data/tags/",
		"/latest/meta-data/tags/instance",
		"/latest/meta-data/tags/instance/owner",
	} {
		req := httptest.NewRequest("GET", path, nil)
		req.RemoteAddr = "10.0.0.5:5000"
		req.Header.Set("X-Aws-Ec2-Metadata-Token", token)
		got := serve(srv, req)
		if got.status != http.StatusNotFound || got.body != "" || got.contentType != "text/plain" {
			t.Errorf("%s: got %d %q (%s), want 404 with an empty text/plain body", path, got.status, got.body, got.contentType)
		}
	}
	// A path under tags/ that is not registered is still the index's, like any
	// unknown key.
	req := httptest.NewRequest("GET", "/latest/meta-data/tags/other", nil)
	req.RemoteAddr = "10.0.0.5:5000"
	req.Header.Set("X-Aws-Ec2-Metadata-Token", token)
	if got := serve(srv, req); got.status != http.StatusOK || !strings.Contains(got.body, "instance-id") {
		t.Errorf("tags/other: got %d %q, want the index", got.status, got.body)
	}
}
