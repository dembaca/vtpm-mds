package imds

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dembaca/prox-mds/internal/config"
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


