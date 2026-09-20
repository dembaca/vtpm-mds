package identity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dembaca/vtpm-mds/imds"
	"github.com/dembaca/vtpm-mds/internal/config"
	"github.com/dembaca/vtpm-mds/internal/inventory"
	"github.com/golang-jwt/jwt/v5"
)

func TestHandleJWKS(t *testing.T) {
	req := httptest.NewRequest("GET", "/.well-known/jwks.json", nil)
	rec := httptest.NewRecorder()
	
	HandleJWKS(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	
	var jwks JWKS
	if err := json.Unmarshal(rec.Body.Bytes(), &jwks); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}
	
	if len(jwks.Keys) == 0 {
		t.Error("Expected at least one key in JWKS")
	}
	
	key := jwks.Keys[0]
	if key.Kty != "RSA" {
		t.Errorf("Expected key type 'RSA', got '%s'", key.Kty)
	}
	if key.Use != "sig" {
		t.Errorf("Expected key use 'sig', got '%s'", key.Use)
	}
	if key.Alg != "RS256" {
		t.Errorf("Expected algorithm 'RS256', got '%s'", key.Alg)
	}
}

func TestHandleIdentity(t *testing.T) {
	store := setupTestStore()
	
	req := httptest.NewRequest("GET", "/latest/identity", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	
	token, _ := store.GenerateToken()
	req.Header.Set("X-Aws-Ec2-Metadata-Token", token.Token)
	
	rec := httptest.NewRecorder()
	HandleIdentity(store)(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	
	var response map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}
	
	// Verify expected fields
	if response["token"] == nil {
		t.Error("Expected token field")
	}
	if response["claims"] == nil {
		t.Error("Expected claims field")
	}
	if response["jwks_uri"] == nil {
		t.Error("Expected jwks_uri field")
	}
	
	tokenStr, ok := response["token"].(string)
	if !ok || tokenStr == "" {
		t.Error("Expected non-empty token string")
	}
	
	// Verify JWT can be parsed
	parsed, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return []byte("placeholder-secret-key"), nil
	})
	if err != nil {
		t.Fatalf("Failed to parse JWT: %v", err)
	}
	
	if !parsed.Valid {
		t.Error("JWT should be valid")
	}
	
	// Verify claims
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("Failed to parse claims")
	}
	
	if claims["iss"] != "vtpm-mds" {
		t.Errorf("Expected issuer 'vtpm-mds', got '%v'", claims["iss"])
	}
	
	if claims["instance_id"] == nil {
		t.Error("Expected instance_id in claims")
	}
	
	if claims["hostname"] == nil {
		t.Error("Expected hostname in claims")
	}
	
	if claims["ip"] == nil {
		t.Error("Expected ip in claims")
	}
}

func TestHandleIdentity_MissingToken(t *testing.T) {
	store := setupTestStore()
	
	req := httptest.NewRequest("GET", "/latest/identity", nil)
	rec := httptest.NewRecorder()
	
	HandleIdentity(store)(rec, req)
	
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
	
	body := rec.Body.String()
	if !strings.Contains(body, "IMDSv2 token required") {
		t.Error("Expected error message about missing token")
	}
}

func TestIdentityClaims_Structure(t *testing.T) {
	claims := IdentityClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:  "vtpm-mds",
			Subject: "test-instance",
		},
		InstanceID: "test-instance",
		Hostname:   "test-host",
		IP:         "192.168.1.100",
		Level:      "attested",
		Attest:     true,
	}
	
	// Verify claims structure
	if claims.InstanceID != "test-instance" {
		t.Errorf("Expected InstanceID 'test-instance', got '%s'", claims.InstanceID)
	}
	if claims.Hostname != "test-host" {
		t.Errorf("Expected Hostname 'test-host', got '%s'", claims.Hostname)
	}
	if claims.IP != "192.168.1.100" {
		t.Errorf("Expected IP '192.168.1.100', got '%s'", claims.IP)
	}
	if claims.Level != "attested" {
		t.Errorf("Expected Level 'attested', got '%s'", claims.Level)
	}
	if !claims.Attest {
		t.Error("Expected Attest to be true")
	}
}

// TestHandleIdentity_NamesTheBoundVM is the inverted task 1.2 test (task
// 4.2). Baseline, recorded before the fix: a caller bound to VM 100 that sent
// "X-Forwarded-For: 10.9.9.9" was named "i-10-9-9-9", because this endpoint
// had its own getInstanceID that consulted no inventory at all. It now
// reports the bound record's id, and the "ip" claim is the peer address.
func TestHandleIdentity_NamesTheBoundVM(t *testing.T) {
	store := setupTestStore()

	rec := httptest.NewRecorder()
	HandleIdentity(store)(rec, boundRequest(t, store, "100"))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	claims := decodeClaims(t, rec)

	if claims.InstanceID != "i-100" {
		t.Errorf("expected instance_id 'i-100', got %q", claims.InstanceID)
	}
	if claims.Subject != "i-100" {
		t.Errorf("expected sub 'i-100', got %q", claims.Subject)
	}
	if claims.IP != "192.168.1.100" {
		t.Errorf("expected the ip claim to be the peer address '192.168.1.100', got %q", claims.IP)
	}
	if strings.Contains(rec.Body.String(), "10.9.9.9") {
		t.Error("the forwarded-for address must not appear anywhere in the document")
	}
}

// TestHandleIdentity_UnboundCallerRefused covers the fourth refused path
// (tasks 3.3, 3.4): /latest/identity refuses a caller with no bound VM record
// exactly as the metadata handlers do, and issues no token.
func TestHandleIdentity_UnboundCallerRefused(t *testing.T) {
	store := setupTestStore()

	if !imds.RequiresVMIdentity("/latest/identity") {
		t.Fatal("/latest/identity must be on the refusing side of the line")
	}

	// The route as internal/server.New registers it.
	handler := imds.RequireVMIdentity(true, store, HandleIdentity(store))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, unboundRequest(t, store))

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != "404 page not found\n" {
		t.Errorf("expected the body \"404 page not found\\n\", got %q", body)
	}
	if strings.Contains(rec.Body.String(), "token") {
		t.Error("no token may be issued to a refused caller")
	}

	// The same route serves a bound caller.
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, boundRequest(t, store, "100"))
	if rec.Code != http.StatusOK {
		t.Errorf("bound caller: expected status 200, got %d", rec.Code)
	}
}

// TestHandleIdentity_FallbackWithoutRequireVMIdentity covers the migration
// aid at this endpoint: with mds.require_vm_identity false an unbound caller
// is named from its peer address, never from its header (task 4.3).
func TestHandleIdentity_FallbackWithoutRequireVMIdentity(t *testing.T) {
	store := setupTestStore()

	handler := imds.RequireVMIdentity(false, store, HandleIdentity(store))

	req := httptest.NewRequest("GET", "/latest/identity", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "10.9.9.9")
	req.Header.Set("X-Aws-Ec2-Metadata-Token", newToken(t, store))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	claims := decodeClaims(t, rec)
	if claims.InstanceID != "i-127-0-0-1" {
		t.Errorf("expected instance_id 'i-127-0-0-1', got %q", claims.InstanceID)
	}
	if claims.Subject != "i-127-0-0-1" {
		t.Errorf("expected sub 'i-127-0-0-1', got %q", claims.Subject)
	}
}

// TestHandleIdentity_IPClaimForPeerWithoutIPv4 verifies the ip claim is the
// empty string when the peer has no IPv4 address, rather than a fragment of
// the address text (task 4.4).
func TestHandleIdentity_IPClaimForPeerWithoutIPv4(t *testing.T) {
	store := setupTestStore()

	req := httptest.NewRequest("GET", "/latest/identity", nil)
	req.RemoteAddr = "[fe80::1]:5000"
	req.Header.Set("X-Aws-Ec2-Metadata-Token", newToken(t, store))
	req = req.WithContext(context.WithValue(req.Context(),
		inventory.VMConfigContextKey, &inventory.VMConfig{VMID: "100"}))

	rec := httptest.NewRecorder()
	HandleIdentity(store)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	claims := decodeClaims(t, rec)
	if claims.IP != "" {
		t.Errorf("expected an empty ip claim for a peer with no IPv4 address, got %q", claims.IP)
	}
	if claims.InstanceID != "i-100" {
		t.Errorf("expected instance_id 'i-100', got %q", claims.InstanceID)
	}
}

func decodeClaims(t *testing.T, rec *httptest.ResponseRecorder) IdentityClaims {
	t.Helper()
	var response struct {
		Claims IdentityClaims `json:"claims"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	return response.Claims
}

func newToken(t *testing.T, store *imds.TokenStore) string {
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
func unboundRequest(t *testing.T, store *imds.TokenStore) *http.Request {
	t.Helper()
	req := httptest.NewRequest("GET", "/latest/identity", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	req.Header.Set("X-Forwarded-For", "10.9.9.9")
	req.Header.Set("X-Aws-Ec2-Metadata-Token", newToken(t, store))
	return req
}

// boundRequest is a tokened request whose connection carries the VM record
// the inventory resolved, as connContext sets it on the real server.
func boundRequest(t *testing.T, store *imds.TokenStore, vmid string) *http.Request {
	t.Helper()
	req := unboundRequest(t, store)
	return req.WithContext(context.WithValue(req.Context(),
		inventory.VMConfigContextKey, &inventory.VMConfig{VMID: vmid}))
}

func setupTestStore() *imds.TokenStore {
	cfg := config.DefaultConfig()
	store := imds.NewTokenStore(cfg)
	SetStore(store)
	return store
}


