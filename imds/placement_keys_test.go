package imds

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dembaca/vtpm-mds/internal/inventory"
)

var rackTopology = inventory.Topology{
	"availability-zone": "room-a", "region": "region-1", "node": "node-a", "cluster": "lab",
	"rack": "rack-01", "host-id": "h-01",
}

func keyRequest(t *testing.T, key string) *http.Request {
	t.Helper()
	req := unboundRequest(t, "/latest/meta-data/placement/"+key)
	req.SetPathValue("key", key)
	return req
}

// A key of the node description is served with no trailing newline, as text/plain.
func TestAKeyOfTheNodeDescriptionIsServed(t *testing.T) {
	setupTestStore()
	withHostTopology(t, rackTopology)
	rec := get(HandlePlacementKey, keyRequest(t, "rack"))
	if rec.Code != http.StatusOK || rec.Body.String() != "rack-01" {
		t.Fatalf("got %d %q, want 200 \"rack-01\"", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain" {
		t.Fatalf("Content-Type %q, want text/plain", ct)
	}
}

// Node and cluster are keys like any other.
func TestNodeAndClusterAreServed(t *testing.T) {
	setupTestStore()
	withHostTopology(t, rackTopology)
	for key, want := range map[string]string{"node": "node-a", "cluster": "lab"} {
		if rec := get(HandlePlacementKey, keyRequest(t, key)); rec.Code != http.StatusOK || rec.Body.String() != want {
			t.Errorf("%s: got %d %q, want 200 %q", key, rec.Code, rec.Body.String(), want)
		}
	}
}

// An EC2 key can be overridden by the node: nothing refuses it.
func TestAnEC2KeyCanBeOverriddenByTheNode(t *testing.T) {
	setupTestStore()
	withHostTopology(t, rackTopology)
	if rec := get(HandlePlacementKey, keyRequest(t, "host-id")); rec.Code != http.StatusOK || rec.Body.String() != "h-01" {
		t.Fatalf("got %d %q, want 200 \"h-01\"", rec.Code, rec.Body.String())
	}
}

// A key the topology does not have is the empty 404, never the index, and keys
// are case-sensitive.
func TestAKeyWithNoValueIsAnEmpty404(t *testing.T) {
	setupTestStore()
	withHostTopology(t, inventory.DefaultTopology())
	for _, key := range []string{"rack", "node", "Region", "no-such-key"} {
		wantEmpty404(t, get(HandlePlacementKey, keyRequest(t, key)))
	}
}

// The directory lists the keys in ascending byte order, each with a newline.
func TestThePlacementDirectoryListsTheKeys(t *testing.T) {
	setupTestStore()
	withHostTopology(t, rackTopology)
	rec := get(HandlePlacementKeys, unboundRequest(t, "/latest/meta-data/placement/"))
	want := "availability-zone\ncluster\nhost-id\nnode\nrack\nregion\n"
	if rec.Code != http.StatusOK || rec.Body.String() != want {
		t.Fatalf("got %d %q, want 200 %q", rec.Code, rec.Body.String(), want)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain" {
		t.Fatalf("Content-Type %q, want text/plain", ct)
	}
}

// With the defaults only, availability-zone and region are the list.
func TestTheDefaultDirectoryHasZoneAndRegion(t *testing.T) {
	setupTestStore()
	withHostTopology(t, inventory.DefaultTopology())
	rec := get(HandlePlacementKeys, unboundRequest(t, "/latest/meta-data/placement/"))
	if rec.Body.String() != "availability-zone\nregion\n" {
		t.Fatalf("got %q", rec.Body.String())
	}
}

// An unbound caller reads the same topology as a bound one.
func TestAnUnboundCallerReadsTheSameTopology(t *testing.T) {
	setupTestStore()
	withHostTopology(t, rackTopology)
	req := keyRequest(t, "rack")
	bound := boundRequest(t, "/latest/meta-data/placement/rack", "100")
	bound.SetPathValue("key", "rack")
	if a, b := get(HandlePlacementKey, req).Body.String(), get(HandlePlacementKey, bound).Body.String(); a != "rack-01" || b != a {
		t.Fatalf("unbound %q, bound %q, want both rack-01", a, b)
	}
}

// Both paths need a session token.
func TestPlacementKeysNeedAToken(t *testing.T) {
	setupTestStore()
	for name, h := range map[string]http.HandlerFunc{"placement/": HandlePlacementKeys, "placement/rack": HandlePlacementKey} {
		if rec := get(h, httptest.NewRequest("GET", "/latest/meta-data/"+name, nil)); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: expected 401 without a token, got %d", name, rec.Code)
		}
	}
}

// The identity document keeps exactly its fixed keys: the other topology keys do
// not enter it.
func TestTheDocumentDoesNotCarryTheOtherKeys(t *testing.T) {
	setupTestStore()
	withHostTopology(t, rackTopology)
	rec := get(HandleInstanceIdentityDocument, boundRequest(t, "/latest/dynamic/instance-identity/document", "100"))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	for _, leak := range []string{"rack-01", "h-01", "node-a", "rack"} {
		if contains(rec.Body.String(), leak) {
			t.Errorf("the document mentions %q: %s", leak, rec.Body.String())
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
