package imds

import (
	"testing"
	"time"

	"github.com/dembaca/prox-mds/internal/config"
)

func TestTokenStore_GenerateToken(t *testing.T) {
	cfg := config.DefaultConfig()
	store := NewTokenStore(cfg)
	
	token, err := store.GenerateToken()
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}
	
	if token.Token == "" {
		t.Error("Token should not be empty")
	}
	
	if time.Now().After(token.ExpiresAt) {
		t.Error("Token should not be expired immediately")
	}
}

func TestTokenStore_ValidateToken(t *testing.T) {
	cfg := config.DefaultConfig()
	store := NewTokenStore(cfg)
	
	token, err := store.GenerateToken()
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}
	
	if !store.ValidateToken(token.Token) {
		t.Error("Valid token should be validated")
	}
	
	if store.ValidateToken("invalid-token") {
		t.Error("Invalid token should not be validated")
	}
}

func TestTokenStore_TokenExpiry(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MDS.TokenTTL = "1ms"
	store := NewTokenStore(cfg)
	
	token, err := store.GenerateToken()
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}
	
	time.Sleep(10 * time.Millisecond)
	
	if store.ValidateToken(token.Token) {
		t.Error("Expired token should not be validated")
	}
}


