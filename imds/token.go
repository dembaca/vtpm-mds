package imds

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/dembaca/prox-mds/internal/config"
)

// Token represents an IMDSv2 token
type Token struct {
	Token      string
	ExpiresAt  time.Time
	CreatedAt  time.Time
}

// TokenStore manages IMDSv2 tokens
type TokenStore struct {
	mu       sync.RWMutex
	tokens   map[string]*Token
	tokenTTL time.Duration
	cleanup  *time.Ticker
}

// NewTokenStore creates a new token store
func NewTokenStore(cfg *config.Config) *TokenStore {
	ttl, _ := time.ParseDuration(cfg.MDS.TokenTTL)
	
	store := &TokenStore{
		tokens:   make(map[string]*Token),
		tokenTTL: ttl,
		cleanup:  time.NewTicker(1 * time.Minute),
	}
	
	go store.cleanupExpired()
	
	return store
}

// GenerateToken generates a new IMDSv2 token
func (ts *TokenStore) GenerateToken() (*Token, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("failed to generate random token: %w", err)
	}
	
	tokenValue := base64.StdEncoding.EncodeToString(b)
	
	token := &Token{
		Token:      tokenValue,
		ExpiresAt:  time.Now().Add(ts.tokenTTL),
		CreatedAt:  time.Now(),
	}
	
	ts.mu.Lock()
	ts.tokens[token.Token] = token
	ts.mu.Unlock()
	
	return token, nil
}

// ValidateToken checks if a token is valid
func (ts *TokenStore) ValidateToken(tokenStr string) bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	
	token, exists := ts.tokens[tokenStr]
	if !exists {
		return false
	}
	
	if time.Now().After(token.ExpiresAt) {
		return false
	}
	
	return true
}

// cleanupExpired removes expired tokens
func (ts *TokenStore) cleanupExpired() {
	for range ts.cleanup.C {
		ts.mu.Lock()
		now := time.Now()
		for k, v := range ts.tokens {
			if now.After(v.ExpiresAt) {
				delete(ts.tokens, k)
			}
		}
		ts.mu.Unlock()
	}
}


