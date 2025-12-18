package attest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleNonce(t *testing.T) {
	setupTestStore()
	
	req := httptest.NewRequest("GET", "/latest/attest/nonce", nil)
	rec := httptest.NewRecorder()
	
	HandleNonce(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	
	var response map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}
	
	if response["nonce"] == nil {
		t.Error("Expected nonce field")
	}
	
	nonce, ok := response["nonce"].(string)
	if !ok || nonce == "" {
		t.Error("Expected non-empty nonce string")
	}
	
	// Verify expiry
	if response["expiry"] == nil {
		t.Error("Expected expiry field")
	}
}

func TestHandleAttest_ValidNonce(t *testing.T) {
	setupTestStore()
	nonce, _ := store.GenerateNonce()
	
	reqBody := map[string]interface{}{
		"quote":     "test-quote",
		"ek_cert":   "test-cert",
		"nonce":     nonce,
		"pcr_values": []map[string]interface{}{},
	}
	
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/latest/attest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	
	HandleAttest(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	
	var response AttestationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}
	
	if !response.Valid {
		t.Error("Expected valid attestation response")
	}
}

func TestHandleAttest_InvalidNonce(t *testing.T) {
	setupTestStore()
	
	reqBody := map[string]interface{}{
		"quote":     "test-quote",
		"ek_cert":   "test-cert",
		"nonce":     "invalid-nonce",
		"pcr_values": []map[string]interface{}{},
	}
	
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/latest/attest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	
	HandleAttest(rec, req)
	
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
	
	var response AttestationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}
	
	if response.Valid {
		t.Error("Expected invalid attestation for invalid nonce")
	}
}

func TestHandleAttest_WrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/latest/attest", nil)
	rec := httptest.NewRecorder()
	
	HandleAttest(rec, req)
	
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", rec.Code)
	}
}

func TestHandleAttest_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/latest/attest", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	
	HandleAttest(rec, req)
	
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rec.Code)
	}
}

func TestAttestationResponse_MarshalJSON(t *testing.T) {
	resp := AttestationResponse{
		Valid:    true,
		Message:  "test",
		Identity: "test-identity",
	}
	
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}
	
	var unmarshalled AttestationResponse
	if err := json.Unmarshal(data, &unmarshalled); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	
	if unmarshalled.Valid != resp.Valid {
		t.Error("Valid field mismatch")
	}
	if unmarshalled.Message != resp.Message {
		t.Error("Message field mismatch")
	}
	if unmarshalled.Identity != resp.Identity {
		t.Error("Identity field mismatch")
	}
}

func setupTestStore() {
	store = NewNonceStore()
}


