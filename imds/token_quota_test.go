package imds

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/dembaca/vtpm-mds/internal/config" // pragma: allowlist secret
)

func TestPeerTokenQuotaEvictsOldestOfThatPeer(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MDS.TokenTTL = "1h"
	store := NewTokenStore(cfg)

	var first string
	for i := 0; i < MaxLiveTokensPerPeer; i++ {
		tok, err := store.GenerateToken("10.0.0.1")
		if err != nil {
			t.Fatalf("mint %d: %v", i, err)
		}
		if i == 0 {
			first = tok.Token
		}
		time.Sleep(time.Millisecond)
	}
	if store.LiveTokenCountForPeer("10.0.0.1") != MaxLiveTokensPerPeer {
		t.Fatalf("want %d live for peer, got %d", MaxLiveTokensPerPeer, store.LiveTokenCountForPeer("10.0.0.1"))
	}

	other, err := store.GenerateToken("10.0.0.2")
	if err != nil {
		t.Fatal(err)
	}

	newer, err := store.GenerateToken("10.0.0.1")
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

	for i := 0; i < GlobalTokenEmergencyCap; i++ {
		if _, err := store.GenerateToken("peer-" + strconv.Itoa(i)); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
	if store.LiveTokenCount() != GlobalTokenEmergencyCap {
		t.Fatalf("want %d, got %d", GlobalTokenEmergencyCap, store.LiveTokenCount())
	}
	_, err := store.GenerateToken("overflow")
	if !errors.Is(err, ErrTokenStoreEmergency) {
		t.Fatalf("want ErrTokenStoreEmergency, got %v", err)
	}
}
