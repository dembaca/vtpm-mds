package imds

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/dembaca/vtpm-mds/internal/config" // pragma: allowlist secret
)

// Per-peer and global token-store caps (throttle-guest-requests D-c / D-h).
// Vars so tests can lower the emergency brake without minting 65536 tokens.
var (
	MaxLiveTokensPerPeer    = 32
	GlobalTokenEmergencyCap = 65536
)

// ErrTokenStoreEmergency is returned by GenerateToken when the global
// emergency capacity would be exceeded after per-peer eviction.
var ErrTokenStoreEmergency = errors.New("token store emergency capacity reached")

// Token represents an IMDSv2 token
type Token struct {
	Token     string
	ExpiresAt time.Time
	CreatedAt time.Time
	Peer      string // minting peer, for quota/eviction only
}

// TokenStore manages IMDSv2 tokens
type TokenStore struct {
	mu       sync.RWMutex
	tokens   map[string]*Token
	tokenTTL time.Duration
	cleanup  *time.Ticker
}

// NewTokenStore creates a new token store. cfg.MDS.TokenTTL is assumed to
// already be a valid Go duration: config.Load validates it at load time and
// config.DefaultConfig() ships a constant that parses cleanly, so the error
// here is intentionally discarded rather than re-validated.
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

// GenerateToken mints a new IMDSv2 token for peer (D-h). peer is the minting
// caller's IP for quota only; validation stays peer-agnostic. An empty peer is
// stored as "unknown".
func (ts *TokenStore) GenerateToken(peer string) (*Token, error) {
	if peer == "" {
		peer = "unknown"
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("failed to generate random token: %w", err)
	}

	tokenValue := base64.StdEncoding.EncodeToString(b)
	now := time.Now()
	token := &Token{
		Token:     tokenValue,
		ExpiresAt: now.Add(ts.tokenTTL),
		CreatedAt: now,
		Peer:      peer,
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.discardExpiredLocked(now)
	ts.evictOldestOfPeerLocked(peer, MaxLiveTokensPerPeer-1)

	if len(ts.tokens) >= GlobalTokenEmergencyCap {
		log.Printf("token store emergency brake: peer=%s size=%d cap=%d", peer, len(ts.tokens), GlobalTokenEmergencyCap)
		return nil, ErrTokenStoreEmergency
	}

	ts.tokens[token.Token] = token
	return token, nil
}

// discardExpiredLocked removes expired tokens. Caller holds ts.mu.
func (ts *TokenStore) discardExpiredLocked(now time.Time) {
	for k, v := range ts.tokens {
		if now.After(v.ExpiresAt) {
			delete(ts.tokens, k)
		}
	}
}

// evictOldestOfPeerLocked deletes the oldest live tokens of peer until that
// peer holds at most keep live tokens. Other peers are untouched.
func (ts *TokenStore) evictOldestOfPeerLocked(peer string, keep int) {
	if keep < 0 {
		keep = 0
	}
	var mine []*Token
	for _, v := range ts.tokens {
		if v.Peer == peer {
			mine = append(mine, v)
		}
	}
	if len(mine) <= keep {
		return
	}
	// Sort by CreatedAt ascending (oldest first) without importing sort for
	// a tiny n: insertion of oldest picks.
	for len(mine) > keep {
		oldest := 0
		for i := 1; i < len(mine); i++ {
			if mine[i].CreatedAt.Before(mine[oldest].CreatedAt) {
				oldest = i
			}
		}
		delete(ts.tokens, mine[oldest].Token)
		mine[oldest] = mine[len(mine)-1]
		mine = mine[:len(mine)-1]
	}
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

// LiveTokenCount returns how many unexpired tokens the store holds (tests).
func (ts *TokenStore) LiveTokenCount() int {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.discardExpiredLocked(time.Now())
	return len(ts.tokens)
}

// LiveTokenCountForPeer returns unexpired tokens minted by peer (tests).
func (ts *TokenStore) LiveTokenCountForPeer(peer string) int {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	now := time.Now()
	n := 0
	for _, v := range ts.tokens {
		if v.Peer == peer && !now.After(v.ExpiresAt) {
			n++
		}
	}
	return n
}

// cleanupExpired removes expired tokens
func (ts *TokenStore) cleanupExpired() {
	for range ts.cleanup.C {
		ts.mu.Lock()
		ts.discardExpiredLocked(time.Now())
		ts.mu.Unlock()
	}
}
