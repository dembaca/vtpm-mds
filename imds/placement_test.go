package imds

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dembaca/vtpm-mds/internal/inventory"
)

// withHostTopology sets the host's topology for one test and restores the
// default afterwards.
func withHostTopology(t *testing.T, p inventory.Topology) {
	t.Helper()
	inventory.SetHostTopology(p)
	t.Cleanup(func() { inventory.SetHostTopology(inventory.DefaultTopology()) })
}

// declaring is a tokened request from a connection bound to a VM record whose
// raw configuration carries the given placement keys.
func declaring(t *testing.T, path string, raw map[string]string) *http.Request {
	t.Helper()
	req := unboundRequest(t, path)
	return req.WithContext(context.WithValue(req.Context(),
		inventory.VMConfigContextKey, &inventory.VMConfig{VMID: "100", RawConfig: raw}))
}

func placementOfRequest(t *testing.T, req *http.Request) (zone, region string) {
	t.Helper()
	zrec, rrec := httptest.NewRecorder(), httptest.NewRecorder()
	zreq := req.Clone(req.Context())
	rreq := req.Clone(req.Context())
	HandleAvailabilityZone(zrec, zreq)
	HandleRegion(rrec, rreq)
	for name, rec := range map[string]*httptest.ResponseRecorder{"zone": zrec, "region": rrec} {
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", name, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "text/plain" {
			t.Fatalf("%s: Content-Type %q, want text/plain", name, ct)
		}
	}
	return zrec.Body.String(), rrec.Body.String()
}

// The host's zone and region are served, with no trailing newline, to a caller
// bound to any VM record.
func TestTheHostsZoneAndRegionAreServed(t *testing.T) {
	setupTestStore()
	withHostTopology(t, inventory.Topology{"availability-zone": "DUS4", "region": "DUS"})
	zone, region := placementOfRequest(t, declaring(t, "/latest/meta-data/placement/region", map[string]string{"name": "x"}))
	if zone != "DUS4" || region != "DUS" {
		t.Fatalf("got zone %q, region %q; want DUS4 and DUS", zone, region)
	}
}

// A zone or a region in the VM record no longer places it: the record's raw
// configuration and its tags are never read for a placement.
func TestAVMLevelPlacementIsNotServed(t *testing.T) {
	setupTestStore()
	withHostTopology(t, inventory.Topology{"availability-zone": "hogan", "region": "bgl-cluster"})
	req := unboundRequest(t, "/latest/meta-data/placement/region")
	req = req.WithContext(context.WithValue(req.Context(), inventory.VMConfigContextKey,
		&inventory.VMConfig{VMID: "100", RawConfig: map[string]string{"zone": "rack-b", "region": "bgl-1"},
			Tags: map[string]string{"zone": "rack-b"}}))
	zone, region := placementOfRequest(t, req)
	if zone != "hogan" || region != "bgl-cluster" {
		t.Fatalf("got zone %q, region %q; want hogan and bgl-cluster", zone, region)
	}
}

// An unbound caller is served the host's placement.
func TestAnUnboundCallerIsServedTheHostsPlacement(t *testing.T) {
	setupTestStore()
	withHostTopology(t, inventory.Topology{"availability-zone": "hogan", "region": "bgl-cluster"})
	zone, region := placementOfRequest(t, unboundRequest(t, "/latest/meta-data/placement/region"))
	if zone != "hogan" || region != "bgl-cluster" {
		t.Fatalf("got zone %q, region %q; want hogan and bgl-cluster", zone, region)
	}
}

// The identity document agrees with the placement keys.
func TestTheDocumentAndThePlacementKeysAgree(t *testing.T) {
	setupTestStore()
	withHostTopology(t, inventory.Topology{"availability-zone": "hogan", "region": "bgl-cluster"})
	withHostTopology(t, inventory.Topology{"availability-zone": "DUS4", "region": "DUS"})
	req := declaring(t, "/latest/dynamic/instance-identity/document", map[string]string{"name": "x"})
	rec := httptest.NewRecorder()
	HandleInstanceIdentityDocument(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["region"] != "DUS" || doc["availabilityZone"] != "DUS4" {
		t.Fatalf("got region %v, availabilityZone %v; want DUS and DUS4", doc["region"], doc["availabilityZone"])
	}
}

// With nothing declared and the YAML backend's host placement, the document keeps
// the values it served before.
func TestTheDocumentKeepsTheOldConstantsByDefault(t *testing.T) {
	setupTestStore()
	req := declaring(t, "/latest/dynamic/instance-identity/document", map[string]string{"name": "x"})
	rec := httptest.NewRecorder()
	HandleInstanceIdentityDocument(rec, req)
	var doc map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["region"] != "local" || doc["availabilityZone"] != "proxmox" {
		t.Fatalf("got region %v, availabilityZone %v; want local and proxmox", doc["region"], doc["availabilityZone"])
	}
}

// placement/region needs a session token, like every metadata read.
func TestRegionNeedsAToken(t *testing.T) {
	setupTestStore()
	req := httptest.NewRequest("GET", "/latest/meta-data/placement/region", nil)
	rec := httptest.NewRecorder()
	HandleRegion(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a token, got %d", rec.Code)
	}
}
