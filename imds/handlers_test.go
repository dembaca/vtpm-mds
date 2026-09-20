package imds

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/dembaca/vtpm-mds/internal/config"
	"github.com/dembaca/vtpm-mds/internal/inventory"
)

func TestHandleInstanceID(t *testing.T) {
	// Setup
	setupTestStore()
	req := httptest.NewRequest("GET", "/latest/meta-data/instance-id", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	
	// Get token first
	token, _ := store.GenerateToken()
	req.Header.Set("X-Aws-Ec2-Metadata-Token", token.Token)
	
	rec := httptest.NewRecorder()
	HandleInstanceID(rec, req)
	
	// Verify
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	
	body := rec.Body.String()
	if !strings.HasPrefix(body, "i-") {
		t.Errorf("Expected instance ID to start with 'i-', got '%s'", body)
	}
}

func TestHandleHostname(t *testing.T) {
	setupTestStore()
	req := httptest.NewRequest("GET", "/latest/meta-data/local-hostname", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	
	token, _ := store.GenerateToken()
	req.Header.Set("X-Aws-Ec2-Metadata-Token", token.Token)
	
	rec := httptest.NewRecorder()
	HandleHostname(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	
	body := rec.Body.String()
	if body == "" {
		t.Error("Expected hostname, got empty string")
	}
}

func TestHandleLocalIPv4(t *testing.T) {
	setupTestStore()
	req := httptest.NewRequest("GET", "/latest/meta-data/local-ipv4", nil)
	req.RemoteAddr = "10.0.0.5:12345"
	
	token, _ := store.GenerateToken()
	req.Header.Set("X-Aws-Ec2-Metadata-Token", token.Token)
	
	rec := httptest.NewRecorder()
	HandleLocalIPv4(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	
	body := rec.Body.String()
	if body != "10.0.0.5" {
		t.Errorf("Expected IP '10.0.0.5', got '%s'", body)
	}
}

// TestPeerIP covers the address shapes the peer-address helper must parse
// correctly: an IPv4 host:port, a bracketed IPv6 host:port, a bracketed
// IPv4-mapped IPv6 host:port, and a bare host with no port (task 2.1).
func TestPeerIP(t *testing.T) {
	cases := []struct {
		name       string
		remoteAddr string
		want       string // "" means PeerIP should return nil
	}{
		{"ipv4 host:port", "10.0.0.5:1234", "10.0.0.5"},
		{"bracketed ipv6 host:port", "[fe80::1]:5000", "fe80::1"},
		// net.IP.String() renders an IPv4-mapped IPv6 address as the dotted
		// quad, which is what "To4() decides the address family" in
		// design.md relies on; PeerIP itself does not restrict the family.
		{"bracketed ipv4-mapped ipv6 host:port", "[::ffff:10.0.0.5]:5000", "10.0.0.5"},
		{"bare host, no port", "10.0.0.5", "10.0.0.5"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ip := PeerIP(c.remoteAddr)
			if c.want == "" {
				if ip != nil {
					t.Errorf("expected nil, got %v", ip)
				}
				return
			}
			if ip == nil {
				t.Fatalf("expected %s, got nil", c.want)
			}
			if ip.String() != c.want {
				t.Errorf("expected %s, got %s", c.want, ip.String())
			}
		})
	}
}

// TestHandleLocalIPv4_IPv6Peer verifies the corrected behaviour for a peer
// with no IPv4 address: the fix for the defect reproduced in
// openspec/changes/parse-peer-address-correctly/proposal.md. Baseline,
// recorded before this fix was applied: status 200, body "[fe80" (RemoteAddr
// "[fe80::1]:5000" split on the first colon). This test now asserts the
// corrected 404 response (task 3.1).
func TestHandleLocalIPv4_IPv6Peer(t *testing.T) {
	setupTestStore()
	req := httptest.NewRequest("GET", "/latest/meta-data/local-ipv4", nil)
	req.RemoteAddr = "[fe80::1]:5000"

	token, _ := store.GenerateToken()
	req.Header.Set("X-Aws-Ec2-Metadata-Token", token.Token)

	rec := httptest.NewRecorder()
	HandleLocalIPv4(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != "" {
		t.Errorf("expected empty body, got %q", body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain" {
		t.Errorf("expected Content-Type text/plain, got %q", ct)
	}
}

// TestHandleLocalIPv4_IPv4MappedPeer verifies an IPv4-mapped IPv6 peer is
// served the dotted-quad IPv4 form (task 3.3).
func TestHandleLocalIPv4_IPv4MappedPeer(t *testing.T) {
	setupTestStore()
	req := httptest.NewRequest("GET", "/latest/meta-data/local-ipv4", nil)
	req.RemoteAddr = "[::ffff:10.0.0.5]:5000"

	token, _ := store.GenerateToken()
	req.Header.Set("X-Aws-Ec2-Metadata-Token", token.Token)

	rec := httptest.NewRecorder()
	HandleLocalIPv4(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != "10.0.0.5" {
		t.Errorf("expected body '10.0.0.5', got %q", body)
	}
}

// TestHandleInstanceIdentityDocument_IPv6Peer verifies the corrected
// privateIp behaviour for a peer with no IPv4 address (task 1.2, 3.2):
// baseline, recorded before this fix, was privateIp == "[fe80". The key
// must remain present with the empty string, so the document keeps its ten
// keys.
func TestHandleInstanceIdentityDocument_IPv6Peer(t *testing.T) {
	setupTestStore()
	req := httptest.NewRequest("GET", "/latest/dynamic/instance-identity/document", nil)
	req.RemoteAddr = "[fe80::1]:5000"

	token, _ := store.GenerateToken()
	req.Header.Set("X-Aws-Ec2-Metadata-Token", token.Token)

	rec := httptest.NewRecorder()
	HandleInstanceIdentityDocument(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	ip, present := doc["privateIp"]
	if !present {
		t.Fatal("expected privateIp key to remain present")
	}
	if ip != "" {
		t.Errorf("expected privateIp to be the empty string, got %q", ip)
	}
	if len(doc) != 10 {
		t.Errorf("expected exactly 10 keys in the identity document, got %d: %v", len(doc), doc)
	}
}

// TestHandleInstanceIdentityDocument_IPv4MappedPeer verifies privateIp for
// an IPv4-mapped IPv6 peer is the dotted-quad IPv4 form (task 3.3).
func TestHandleInstanceIdentityDocument_IPv4MappedPeer(t *testing.T) {
	setupTestStore()
	req := httptest.NewRequest("GET", "/latest/dynamic/instance-identity/document", nil)
	req.RemoteAddr = "[::ffff:10.0.0.5]:5000"

	token, _ := store.GenerateToken()
	req.Header.Set("X-Aws-Ec2-Metadata-Token", token.Token)

	rec := httptest.NewRecorder()
	HandleInstanceIdentityDocument(rec, req)

	var doc map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if doc["privateIp"] != "10.0.0.5" {
		t.Errorf("expected privateIp '10.0.0.5', got %q", doc["privateIp"])
	}
}

func TestHandleAvailabilityZone(t *testing.T) {
	setupTestStore()
	req := httptest.NewRequest("GET", "/latest/meta-data/placement/availability-zone", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	
	token, _ := store.GenerateToken()
	req.Header.Set("X-Aws-Ec2-Metadata-Token", token.Token)
	
	rec := httptest.NewRecorder()
	HandleAvailabilityZone(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	
	body := rec.Body.String()
	if body == "" {
		t.Error("Expected availability zone, got empty string")
	}
}

func TestHandleInstanceIdentityDocument(t *testing.T) {
	setupTestStore()
	req := httptest.NewRequest("GET", "/latest/dynamic/instance-identity/document", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	
	token, _ := store.GenerateToken()
	req.Header.Set("X-Aws-Ec2-Metadata-Token", token.Token)
	
	rec := httptest.NewRecorder()
	HandleInstanceIdentityDocument(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	
	var doc map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}
	
	// Verify expected fields
	if doc["instanceId"] == nil {
		t.Error("Expected instanceId field")
	}
	if doc["region"] == nil {
		t.Error("Expected region field")
	}
	if doc["availabilityZone"] == nil {
		t.Error("Expected availabilityZone field")
	}
	if doc["privateIp"] == nil {
		t.Error("Expected privateIp field")
	}
	
	if doc["imageId"] != "proxmox-unknown" {
		t.Errorf("Expected imageId 'proxmox-unknown', got '%v'", doc["imageId"])
	}
	if doc["instanceType"] != "vm" {
		t.Errorf("Expected instanceType 'vm', got '%v'", doc["instanceType"])
	}
}

func TestHandleMetaDataIndex(t *testing.T) {
	setupTestStore()
	req := httptest.NewRequest("GET", "/latest/meta-data/", nil)
	
	token, _ := store.GenerateToken()
	req.Header.Set("X-Aws-Ec2-Metadata-Token", token.Token)
	
	rec := httptest.NewRecorder()
	HandleMetaDataIndex(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	
	body := rec.Body.String()
	if !strings.Contains(body, "instance-id") {
		t.Error("Expected 'instance-id' in metadata index")
	}
	if !strings.Contains(body, "local-hostname") {
		t.Error("Expected 'local-hostname' in metadata index")
	}
}

func TestHandleCreateToken(t *testing.T) {
	cfg := config.DefaultConfig()
	store := NewTokenStore(cfg)
	
	req := httptest.NewRequest("PUT", "/latest/api/token", nil)
	rec := httptest.NewRecorder()
	
	handler := HandleCreateToken(store)
	handler(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	
	body := rec.Body.String()
	if len(body) < 32 { // Base64 encoded token should be at least 32 chars
		t.Errorf("Expected token of length >= 32, got %d", len(body))
	}
	
	// Verify Content-Type
	contentType := rec.Header().Get("Content-Type")
	if contentType != "text/plain" {
		t.Errorf("Expected Content-Type 'text/plain', got '%s'", contentType)
	}
}

func TestMissingToken(t *testing.T) {
	setupTestStore()
	req := httptest.NewRequest("GET", "/latest/meta-data/instance-id", nil)
	rec := httptest.NewRecorder()
	
	HandleInstanceID(rec, req)
	
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
}

func TestInvalidToken(t *testing.T) {
	setupTestStore()
	req := httptest.NewRequest("GET", "/latest/meta-data/instance-id", nil)
	req.Header.Set("X-Aws-Ec2-Metadata-Token", "invalid-token")
	rec := httptest.NewRecorder()
	
	HandleInstanceID(rec, req)
	
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
}

// TestHandleInstanceID_UnboundCallerForwardedFor covers the defect that
// refuse-unbound-metadata-callers fixes (task 1.1). Baseline, recorded before
// the fix: an unbound caller from 192.168.1.100 sending
// "X-Forwarded-For: 10.9.9.9" was answered 200 with the body "i-10-9-9-9", so
// it chose the id it was served. It now gets neither: the header is not
// consulted anywhere, and the route refuses it outright.
func TestHandleInstanceID_UnboundCallerForwardedFor(t *testing.T) {
	setupTestStore()

	// The header no longer reaches the derivation. Calling the handler
	// directly bypasses the route wrapper, which is what makes the id
	// visible at all.
	req := httptest.NewRequest("GET", "/latest/meta-data/instance-id", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	req.Header.Set("X-Forwarded-For", "10.9.9.9")
	req.Header.Set("X-Aws-Ec2-Metadata-Token", newToken(t))

	rec := httptest.NewRecorder()
	HandleInstanceID(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 from the bare handler, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != "i-192-168-1-100" {
		t.Errorf("expected the peer-derived 'i-192-168-1-100', got %q", body)
	}

	// Through the route as the server registers it, with the built-in
	// default, the unbound caller is refused instead.
	rec = httptest.NewRecorder()
	newTestMux(true).ServeHTTP(rec, unboundRequest(t, "/latest/meta-data/instance-id"))

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404 through the guarded route, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != notFoundBody() {
		t.Errorf("expected the catch-all body %q, got %q", notFoundBody(), body)
	}
	if strings.Contains(rec.Body.String(), "10.9.9.9") {
		t.Error("the forwarded-for address must not appear in the response")
	}
}

// metadataRoutes pins both sides of the line the change draws: which paths
// refuse an unbound caller and which keep serving one (tasks 3.4 and 3.5).
// The refused side must match imds.IdentityBearingPaths exactly, so a handler
// added to the wrong side of it fails this test rather than shipping.
var metadataRoutes = []struct {
	path string
	// refused is the expected side of the line for an unbound caller.
	refused bool
	// servedStatus is the status an unbound caller gets when the path is
	// not refused. Every path serves 200 except public-ipv4, which is
	// always unavailable and answers 404 with an empty body — for a bound
	// caller just as much as for an unbound one, which is what makes it a
	// different thing from a refusal.
	servedStatus int
	// mustContain is what an unbound caller must still be served, where the
	// value is worth pinning: the index still lists instance-id even though
	// reading it is refused, and local-ipv4 reports the caller's own peer
	// address.
	mustContain string
}{
	{"/latest/meta-data/instance-id", true, 0, ""},
	{"/latest/dynamic/instance-identity/document", true, 0, ""},
	{"/latest/dynamic/instance-identity/signature", true, 0, ""},
	{"/latest/meta-data/", false, http.StatusOK, "instance-id"},
	{"/latest/meta-data/local-hostname", false, http.StatusOK, "example.com"},
	{"/latest/meta-data/local-ipv4", false, http.StatusOK, "192.168.1.100"},
	{"/latest/meta-data/placement/availability-zone", false, http.StatusOK, "proxmox"},
	{"/latest/meta-data/services/domain", false, http.StatusOK, "localdomain"},
	{"/latest/meta-data/public-ipv4", false, http.StatusNotFound, ""},
}

// TestRefusedAndServedPaths drives every registered metadata path from an
// unbound caller and from a bound one (tasks 3.4, 3.5, 3.7).
// GET /latest/identity is the fourth refused path; it lives in the identity
// package, which imports this one, and is covered by
// identity.TestHandleIdentity_UnboundCallerRefused.
func TestRefusedAndServedPaths(t *testing.T) {
	setupTestStore()
	mux := newTestMux(true)

	for _, rt := range metadataRoutes {
		t.Run(rt.path, func(t *testing.T) {
			if got := RequiresVMIdentity(rt.path); got != rt.refused {
				t.Errorf("RequiresVMIdentity(%q) = %v, want %v", rt.path, got, rt.refused)
			}

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, unboundRequest(t, rt.path))

			if rt.refused {
				if rec.Code != http.StatusNotFound {
					t.Errorf("unbound caller: expected status 404, got %d", rec.Code)
				}
				if body := rec.Body.String(); body != notFoundBody() {
					t.Errorf("unbound caller: expected the catch-all body %q, got %q", notFoundBody(), body)
				}
			} else {
				if rec.Code != rt.servedStatus {
					t.Errorf("unbound caller: expected status %d, got %d", rt.servedStatus, rec.Code)
				}
				if body := rec.Body.String(); body == notFoundBody() {
					t.Error("unbound caller: this path must not be refused, but got the refusal body")
				}
				if rt.mustContain != "" && !strings.Contains(rec.Body.String(), rt.mustContain) {
					t.Errorf("unbound caller: expected the response to contain %q, got %q",
						rt.mustContain, rec.Body.String())
				}
			}

			// A bound caller is served every path, refused or not.
			rec = httptest.NewRecorder()
			mux.ServeHTTP(rec, boundRequest(t, rt.path, "100"))

			wantBound := http.StatusOK
			if !rt.refused && rt.servedStatus != http.StatusOK {
				wantBound = rt.servedStatus
			}
			if rec.Code != wantBound {
				t.Errorf("bound caller: expected status %d, got %d", wantBound, rec.Code)
			}
			if body := rec.Body.String(); body == notFoundBody() {
				t.Error("bound caller: must never be refused")
			}
		})
	}
}

// TestIdentityBearingPathsIsExactlyTheSpecifiedSet pins the enumeration the
// registration consults, so the refused set cannot grow or shrink silently
// (task 3.1).
func TestIdentityBearingPathsIsExactlyTheSpecifiedSet(t *testing.T) {
	want := []string{
		"/latest/meta-data/instance-id",
		"/latest/dynamic/instance-identity/document",
		"/latest/dynamic/instance-identity/signature",
		"/latest/identity",
	}
	if len(IdentityBearingPaths) != len(want) {
		t.Fatalf("expected exactly %d identity-bearing paths, got %d: %v",
			len(want), len(IdentityBearingPaths), IdentityBearingPaths)
	}
	for i, p := range want {
		if IdentityBearingPaths[i] != p {
			t.Errorf("IdentityBearingPaths[%d] = %q, want %q", i, IdentityBearingPaths[i], p)
		}
	}
	for _, p := range []string{"/latest/meta-data/", "/latest/meta-data/local-ipv4", "/health", "/latest/api/token"} {
		if RequiresVMIdentity(p) {
			t.Errorf("%q must not be on the refusing side of the line", p)
		}
	}
}

// TestRefusalRunsAfterTokenValidation verifies the two failures compose in
// the specified order: no token is still answered as a missing token, and
// only a caller that passed the token check is refused (task 3.2).
func TestRefusalRunsAfterTokenValidation(t *testing.T) {
	setupTestStore()
	mux := newTestMux(true)

	req := httptest.NewRequest("GET", "/latest/meta-data/instance-id", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no token: expected status 401, got %d", rec.Code)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != "IMDSv2 token required" {
		t.Errorf("no token: expected body 'IMDSv2 token required', got %q", body)
	}
	if got := rec.Header().Get("X-Aws-Ec2-Metadata-Token"); got != "required" {
		t.Errorf("no token: expected header X-Aws-Ec2-Metadata-Token 'required', got %q", got)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, unboundRequest(t, "/latest/meta-data/instance-id"))
	if rec.Code != http.StatusNotFound {
		t.Errorf("valid token, unbound: expected status 404, got %d", rec.Code)
	}
}

// TestRefusalIsIndistinguishableFromUnknownRoute verifies the refusal is byte
// for byte what the catch-all returns for a route that does not exist
// (task 3.3).
func TestRefusalIsIndistinguishableFromUnknownRoute(t *testing.T) {
	setupTestStore()
	mux := newTestMux(true)

	refused := httptest.NewRecorder()
	mux.ServeHTTP(refused, unboundRequest(t, "/latest/meta-data/instance-id"))

	unknown := httptest.NewRecorder()
	mux.ServeHTTP(unknown, unboundRequest(t, "/latest/no-such-thing"))

	if refused.Code != unknown.Code {
		t.Errorf("status differs: refused %d, unknown route %d", refused.Code, unknown.Code)
	}
	if refused.Body.String() != unknown.Body.String() {
		t.Errorf("body differs: refused %q, unknown route %q", refused.Body.String(), unknown.Body.String())
	}
	if refused.Body.String() != "404 page not found\n" {
		t.Errorf("expected the body \"404 page not found\\n\", got %q", refused.Body.String())
	}
	if refused.Header().Get("Content-Type") != unknown.Header().Get("Content-Type") {
		t.Errorf("Content-Type differs: refused %q, unknown route %q",
			refused.Header().Get("Content-Type"), unknown.Header().Get("Content-Type"))
	}
}

// TestTokenAndHealthAreNeverRefused verifies the two deliberately
// unauthenticated endpoints are served to an unbound caller (task 3.6).
func TestTokenAndHealthAreNeverRefused(t *testing.T) {
	setupTestStore()
	mux := newTestMux(true)

	req := httptest.NewRequest("PUT", "/latest/api/token", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("PUT /latest/api/token: expected status 200, got %d", rec.Code)
	}
	if len(rec.Body.String()) < 32 {
		t.Errorf("PUT /latest/api/token: expected a token, got %q", rec.Body.String())
	}

	req = httptest.NewRequest("GET", "/health", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /health: expected status 200, got %d", rec.Code)
	}
}

// TestFallbackInstanceIDWithoutRequireVMIdentity verifies the migration aid:
// with mds.require_vm_identity false the old synthesized id returns, derived
// from the peer address alone, so a caller sending X-Forwarded-For still
// cannot choose it (task 4.3).
func TestFallbackInstanceIDWithoutRequireVMIdentity(t *testing.T) {
	setupTestStore()
	mux := newTestMux(false)

	req := httptest.NewRequest("GET", "/latest/meta-data/instance-id", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "10.9.9.9")
	req.Header.Set("X-Aws-Ec2-Metadata-Token", newToken(t))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != "i-127-0-0-1" {
		t.Errorf("expected body 'i-127-0-0-1', got %q", body)
	}
}

// TestRefusalIsLogged verifies one log line per refused request, naming the
// caller's peer address, so an operator can tell a refused identity from a
// misspelled route even though the caller cannot (task 5.1).
func TestRefusalIsLogged(t *testing.T) {
	setupTestStore()
	mux := newTestMux(true)

	var logged bytes.Buffer
	log.SetOutput(&logged)
	defer log.SetOutput(os.Stderr)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, unboundRequest(t, "/latest/meta-data/instance-id"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}

	var refusals []string
	for _, line := range strings.Split(strings.TrimSpace(logged.String()), "\n") {
		if strings.Contains(line, "Refusing") {
			refusals = append(refusals, line)
		}
	}
	if len(refusals) != 1 {
		t.Fatalf("expected exactly one refusal log line, got %d: %v", len(refusals), refusals)
	}
	if !strings.Contains(refusals[0], "192.168.1.100") {
		t.Errorf("expected the log line to name the caller's peer address, got %q", refusals[0])
	}
	if !strings.Contains(refusals[0], "/latest/meta-data/instance-id") {
		t.Errorf("expected the log line to name the refused path, got %q", refusals[0])
	}
}

// newTestMux registers the metadata routes exactly as internal/server.New
// does: every route goes through the same RequiresVMIdentity predicate, so a
// path is wrapped in the refusal if and only if it is one of
// IdentityBearingPaths. The token, health and catch-all routes are registered
// unwrapped, as the server registers them.
func newTestMux(requireVMIdentity bool) *http.ServeMux {
	mux := http.NewServeMux()

	handle := func(pattern string, h http.HandlerFunc) {
		path := pattern
		if i := strings.IndexByte(pattern, ' '); i >= 0 {
			path = pattern[i+1:]
		}
		if RequiresVMIdentity(path) {
			mux.Handle(pattern, RequireVMIdentity(requireVMIdentity, store, h))
			return
		}
		mux.HandleFunc(pattern, h)
	}

	mux.HandleFunc("PUT /latest/api/token", HandleCreateToken(store))

	handle("GET /latest/meta-data/instance-id", HandleInstanceID)
	handle("GET /latest/meta-data/local-hostname", HandleHostname)
	handle("GET /latest/meta-data/public-ipv4", HandlePublicIPv4)
	handle("GET /latest/meta-data/local-ipv4", HandleLocalIPv4)
	handle("GET /latest/meta-data/placement/availability-zone", HandleAvailabilityZone)
	handle("GET /latest/meta-data/services/domain", HandleDomain)
	handle("GET /latest/meta-data/", HandleMetaDataIndex)
	handle("GET /latest/dynamic/instance-identity/document", HandleInstanceIdentityDocument)
	handle("GET /latest/dynamic/instance-identity/signature", HandleInstanceIdentitySignature)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	return mux
}

// notFoundBody is what http.NotFound writes, and therefore what both the
// catch-all and a refusal must return.
func notFoundBody() string {
	rec := httptest.NewRecorder()
	http.NotFound(rec, httptest.NewRequest("GET", "/", nil))
	return rec.Body.String()
}

func newToken(t *testing.T) string {
	t.Helper()
	token, err := store.GenerateToken()
	if err != nil {
		t.Fatalf("failed to generate a session token: %v", err)
	}
	return token.Token
}

// unboundRequest is a tokened request from a caller with no VM record bound
// to its connection, which also sends an X-Forwarded-For the service must
// ignore.
func unboundRequest(t *testing.T, path string) *http.Request {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	req.RemoteAddr = "192.168.1.100:12345"
	req.Header.Set("X-Forwarded-For", "10.9.9.9")
	req.Header.Set("X-Aws-Ec2-Metadata-Token", newToken(t))
	return req
}

// boundRequest is a tokened request whose connection carries the VM record
// the inventory resolved, as connContext sets it on the real server.
func boundRequest(t *testing.T, path, vmid string) *http.Request {
	t.Helper()
	req := unboundRequest(t, path)
	return req.WithContext(context.WithValue(req.Context(),
		inventory.VMConfigContextKey, &inventory.VMConfig{VMID: vmid}))
}

func setupTestStore() {
	cfg := config.DefaultConfig()
	store = NewTokenStore(cfg)
}


