package attest

import (
	"encoding/json"
	"net/http"
	"time"
)

// Store reference (initialized by server)
var store *NonceStore

// SetStore sets the global nonce store
func SetStore(s *NonceStore) {
	store = s
}

// AttestationRequest represents a TPM attestation request
type AttestationRequest struct {
	Quote     string `json:"quote"`
	EKCert    string `json:"ek_cert"`
	Nonce     string `json:"nonce"`
	PCRValues []struct {
		Bank string `json:"bank"`
		PCR  int    `json:"pcr"`
		Hash string `json:"hash"`
	} `json:"pcr_values"`
}

// AttestationResponse represents the attestation response
type AttestationResponse struct {
	Valid    bool   `json:"valid"`
	Message  string `json:"message,omitempty"`
	Identity string `json:"identity,omitempty"`
}

// HandleNonce generates a new nonce for TPM quote requests
func HandleNonce(w http.ResponseWriter, r *http.Request) {
	if store == nil {
		http.Error(w, "Nonce store not initialized", http.StatusInternalServerError)
		return
	}

	nonce, err := store.GenerateNonce()
	if err != nil {
		http.Error(w, "Failed to generate nonce", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nonce":  nonce,
		"expiry": time.Now().Add(5 * time.Minute).Format(time.RFC3339),
	})
}

// HandleAttest handles TPM attestation verification
func HandleAttest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AttestationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate nonce
	if store != nil && !store.ValidateNonce(req.Nonce) {
		resp := AttestationResponse{
			Valid:   false,
			Message: "Invalid or expired nonce",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// TODO: Implement TPM quote verification
	// 1. Verify quote signature using EK certificate
	// 2. Verify nonce in quote
	// 3. Check PCR values against policy
	// 4. Extract attestation claims

	// Placeholder response
	resp := AttestationResponse{
		Valid:    true,
		Message:  "Attestation verified (placeholder)",
		Identity: "attested-identity",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}


