package attest

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

// NonceStore manages attestation nonces
type NonceStore struct {
	mu     sync.RWMutex
	nonces map[string]time.Time
}

// NewNonceStore creates a new nonce store
func NewNonceStore() *NonceStore {
	return &NonceStore{
		nonces: make(map[string]time.Time),
	}
}

// GenerateNonce generates a new nonce
func (ns *NonceStore) GenerateNonce() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	
	hash := sha256.Sum256(b)
	nonce := base64.URLEncoding.EncodeToString(hash[:])
	
	ns.mu.Lock()
	ns.nonces[nonce] = time.Now().Add(5 * time.Minute)
	ns.mu.Unlock()
	
	return nonce, nil
}

// ValidateNonce validates and consumes a nonce
func (ns *NonceStore) ValidateNonce(nonce string) bool {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	
	expiry, exists := ns.nonces[nonce]
	if !exists {
		return false
	}
	
	if time.Now().After(expiry) {
		delete(ns.nonces, nonce)
		return false
	}
	
	// Consume the nonce
	delete(ns.nonces, nonce)
	return true
}


