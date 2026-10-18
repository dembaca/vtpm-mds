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

// TokenStore manages IMDSv2 tokens.
//
// Live tokens are indexed by value and also per minting peer (D-m): mint,
// per-peer expiry and eviction touch only that peer's list under the write
// lock. ValidateToken is a map lookup. The minute ticker still sweeps every
// peer for expiry; the global emergency brake uses liveCount.
type TokenStore struct {
	mu             sync.RWMutex
	tokens         map[string]*Token
	byPeer         map[string][]*Token // CreatedAt ascending
	liveCount      int
	emergencyBrake bool // true while at the global cap; one log per engagement
	tokenTTL       time.Duration
	cleanup        *time.Ticker
}

// NewTokenStore creates a new token store. cfg.MDS.TokenTTL is assumed to
// already be a valid Go duration: config.Load validates it at load time and
// config.DefaultConfig() ships a constant that parses cleanly, so the error
// here is intentionally discarded rather than re-validated.
func NewTokenStore(cfg *config.Config) *TokenStore {
	ttl, _ := time.ParseDuration(cfg.MDS.TokenTTL)

	store := &TokenStore{
		tokens:   make(map[string]*Token),
		byPeer:   make(map[string][]*Token),
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
	return ts.generateTokenAt(peer, time.Now())
}

// generateTokenAt is GenerateToken with an injectable clock (tests).
func (ts *TokenStore) generateTokenAt(peer string, now time.Time) (*Token, error) {
	if peer == "" {
		peer = "unknown"
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("failed to generate random token: %w", err)
	}

	tokenValue := base64.StdEncoding.EncodeToString(b)
	token := &Token{
		Token:     tokenValue,
		ExpiresAt: now.Add(ts.tokenTTL),
		CreatedAt: now,
		Peer:      peer,
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.prunePeerExpiredLocked(peer, now)
	ts.evictOldestOfPeerLocked(peer, MaxLiveTokensPerPeer-1)

	if ts.liveCount >= GlobalTokenEmergencyCap {
		if !ts.emergencyBrake {
			log.Printf("token store emergency brake: peer=%s size=%d cap=%d", peer, ts.liveCount, GlobalTokenEmergencyCap)
			ts.emergencyBrake = true
		}
		return nil, ErrTokenStoreEmergency
	}
	ts.emergencyBrake = false

	ts.tokens[token.Token] = token
	ts.byPeer[peer] = append(ts.byPeer[peer], token)
	ts.liveCount++
	return token, nil
}

// prunePeerExpiredLocked removes expired tokens of peer only. Caller holds ts.mu.
func (ts *TokenStore) prunePeerExpiredLocked(peer string, now time.Time) {
	list := ts.byPeer[peer]
	if len(list) == 0 {
		return
	}
	n := 0
	for _, tok := range list {
		if now.After(tok.ExpiresAt) {
			delete(ts.tokens, tok.Token)
			ts.liveCount--
			continue
		}
		list[n] = tok
		n++
	}
	if n == 0 {
		delete(ts.byPeer, peer)
		return
	}
	ts.byPeer[peer] = list[:n]
}

// evictOldestOfPeerLocked deletes the oldest live tokens of peer until that
// peer holds at most keep. Other peers are untouched. Caller holds ts.mu.
// byPeer[peer] is kept in CreatedAt ascending order.
func (ts *TokenStore) evictOldestOfPeerLocked(peer string, keep int) {
	if keep < 0 {
		keep = 0
	}
	list := ts.byPeer[peer]
	for len(list) > keep {
		oldest := list[0]
		delete(ts.tokens, oldest.Token)
		ts.liveCount--
		list = list[1:]
	}
	if len(list) == 0 {
		delete(ts.byPeer, peer)
		return
	}
	ts.byPeer[peer] = list
}

// discardExpiredLocked removes expired tokens across every peer (ticker).
// Caller holds ts.mu.
func (ts *TokenStore) discardExpiredLocked(now time.Time) {
	for peer := range ts.byPeer {
		ts.prunePeerExpiredLocked(peer, now)
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
	return ts.liveCount
}

// LiveTokenCountForPeer returns unexpired tokens minted by peer (tests).
func (ts *TokenStore) LiveTokenCountForPeer(peer string) int {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.prunePeerExpiredLocked(peer, time.Now())
	return len(ts.byPeer[peer])
}

// cleanupExpired removes expired tokens
func (ts *TokenStore) cleanupExpired() {
	for range ts.cleanup.C {
		ts.mu.Lock()
		ts.discardExpiredLocked(time.Now())
		if ts.liveCount < GlobalTokenEmergencyCap {
			ts.emergencyBrake = false
		}
		ts.mu.Unlock()
	}
}
