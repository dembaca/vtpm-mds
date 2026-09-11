package identity

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dembaca/vtpm-mds/imds"
	"github.com/dembaca/vtpm-mds/internal/config"
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

func setupTestStore() *imds.TokenStore {
	cfg := config.DefaultConfig()
	store := imds.NewTokenStore(cfg)
	SetStore(store)
	return store
}


