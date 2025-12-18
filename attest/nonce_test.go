package attest

import (
	"testing"
)

func TestNonceStore_GenerateNonce(t *testing.T) {
	store := NewNonceStore()
	
	nonce, err := store.GenerateNonce()
	if err != nil {
		t.Fatalf("Failed to generate nonce: %v", err)
	}
	
	if nonce == "" {
		t.Error("Nonce should not be empty")
	}
}

func TestNonceStore_ValidateNonce(t *testing.T) {
	store := NewNonceStore()
	
	nonce, err := store.GenerateNonce()
	if err != nil {
		t.Fatalf("Failed to generate nonce: %v", err)
	}
	
	if !store.ValidateNonce(nonce) {
		t.Error("Valid nonce should be validated")
	}
	
	if store.ValidateNonce(nonce) {
		t.Error("Used nonce should not be validated again")
	}
	
	if store.ValidateNonce("invalid-nonce") {
		t.Error("Invalid nonce should not be validated")
	}
}


