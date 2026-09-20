package imds

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dembaca/vtpm-mds/internal/config"
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

func setupTestStore() {
	cfg := config.DefaultConfig()
	store = NewTokenStore(cfg)
}


