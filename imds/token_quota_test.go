package imds

import (
	"bytes"
	"errors"
	"log"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dembaca/vtpm-mds/internal/config" // pragma: allowlist secret
)

func TestPeerTokenQuotaEvictsOldestOfThatPeer(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MDS.TokenTTL = "1h"
	store := NewTokenStore(cfg)

	base := time.Now()
	var first string
	for i := 0; i < MaxLiveTokensPerPeer; i++ {
		tok, err := store.generateTokenAt("10.0.0.1", base.Add(time.Duration(i)*time.Second))
		if err != nil {
			t.Fatalf("mint %d: %v", i, err)
		}
		if i == 0 {
			first = tok.Token
		}
	}
	if store.LiveTokenCountForPeer("10.0.0.1") != MaxLiveTokensPerPeer {
		t.Fatalf("want %d live for peer, got %d", MaxLiveTokensPerPeer, store.LiveTokenCountForPeer("10.0.0.1"))
	}

	other, err := store.generateTokenAt("10.0.0.2", base.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	newer, err := store.generateTokenAt("10.0.0.1", base.Add(time.Duration(MaxLiveTokensPerPeer)*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if store.ValidateToken(first) {
		t.Fatal("oldest token of peer 1 should have been evicted")
	}
	if !store.ValidateToken(newer.Token) {
		t.Fatal("new token should be valid")
	}
	if !store.ValidateToken(other.Token) {
		t.Fatal("peer 2 token must be untouched")
	}
	if store.LiveTokenCountForPeer("10.0.0.1") != MaxLiveTokensPerPeer {
		t.Fatalf("peer 1 should still hold %d, got %d", MaxLiveTokensPerPeer, store.LiveTokenCountForPeer("10.0.0.1"))
	}
}

func TestGlobalEmergencyBrake(t *testing.T) {
	orig := GlobalTokenEmergencyCap
	GlobalTokenEmergencyCap = 8
	t.Cleanup(func() { GlobalTokenEmergencyCap = orig })

	cfg := config.DefaultConfig()
	cfg.MDS.TokenTTL = "1h"
	store := NewTokenStore(cfg)
	base := time.Now()

	for i := 0; i < GlobalTokenEmergencyCap; i++ {
		if _, err := store.generateTokenAt("peer-"+strconv.Itoa(i), base.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
	if store.LiveTokenCount() != GlobalTokenEmergencyCap {
		t.Fatalf("want %d, got %d", GlobalTokenEmergencyCap, store.LiveTokenCount())
	}

	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })

	_, err := store.generateTokenAt("overflow", base.Add(time.Hour))
	if !errors.Is(err, ErrTokenStoreEmergency) {
		t.Fatalf("want ErrTokenStoreEmergency, got %v", err)
	}
	_, err = store.generateTokenAt("overflow-2", base.Add(time.Hour+time.Second))
	if !errors.Is(err, ErrTokenStoreEmergency) {
		t.Fatalf("second want ErrTokenStoreEmergency, got %v", err)
	}
	logs := buf.String()
	if c := strings.Count(logs, "token store emergency brake"); c != 1 {
		t.Fatalf("want one emergency log line, got %d in %q", c, logs)
	}
}

func TestPeerExpiryDoesNotScanOtherPeers(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MDS.TokenTTL = "1h"
	store := NewTokenStore(cfg)
	base := time.Now()

	if _, err := store.generateTokenAt("10.0.0.1", base); err != nil {
		t.Fatal(err)
	}
	tok2, err := store.generateTokenAt("10.0.0.2", base)
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	store.tokens[tok2.Token].ExpiresAt = base.Add(-time.Minute)
	store.mu.Unlock()

	if _, err := store.generateTokenAt("10.0.0.1", base.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	n := store.liveCount
	_, peer2Still := store.tokens[tok2.Token]
	store.mu.Unlock()
	if n != 3 || !peer2Still {
		t.Fatalf("peer-1 mint should not sweep peer-2; liveCount=%d peer2=%v", n, peer2Still)
	}
}
