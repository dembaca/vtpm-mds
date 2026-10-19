package server

import (
	"context"
	"sync"
	"time"

	"github.com/dembaca/vtpm-mds/internal/inventory"
	"github.com/dembaca/vtpm-mds/internal/proxmox"
)

// inventoryPollInterval is how often the running service checks its inventory
// source for a change (reload-when-guest-configs-change D-c).
const inventoryPollInterval = 5 * time.Second

// inventoryPoller remembers the fingerprint of the last load attempt and owns
// the background check. A load is attempted once per distinct fingerprint, so
// an unchanged source causes no reload and no log line, and a broken source is
// reported once, by the load, until it changes.
type inventoryPoller struct {
	mu   sync.Mutex
	last inventory.Fingerprint
	have bool
	stop context.CancelFunc
	done chan struct{}
}

// inventoryFingerprint digests what the configured backend reads; the same
// selection as RefreshVMConfigCache.
func (s *Server) inventoryFingerprint() inventory.Fingerprint {
	if s.cfg.MDS.InventoryPath != "" {
		return inventory.YAMLFingerprint(s.cfg.MDS.InventoryPath)
	}
	return proxmox.SourceFingerprint("")
}

// reloadNow records the current fingerprint and reloads unconditionally, as
// startup and SIGHUP do. The fingerprint is taken before the load, so a change
// made during the load is seen again by the next check.
func (s *Server) reloadNow() error {
	fp := s.inventoryFingerprint()
	s.poll.mu.Lock()
	s.poll.last, s.poll.have = fp, true
	s.poll.mu.Unlock()
	return s.RefreshVMConfigCache()
}

// reloadIfChanged reloads through the SIGHUP path when the source differs from
// the last load attempt, and reports whether it did.
func (s *Server) reloadIfChanged() bool {
	fp := s.inventoryFingerprint()
	s.poll.mu.Lock()
	if s.poll.have && fp == s.poll.last {
		s.poll.mu.Unlock()
		return false
	}
	s.poll.last, s.poll.have = fp, true
	s.poll.mu.Unlock()
	// RefreshVMConfigCache logs the outcome and, on failure, keeps the
	// previous inventory.
	_ = s.RefreshVMConfigCache()
	return true
}

// startInventoryPoller checks the source every interval until
// stopInventoryPoller is called.
func (s *Server) startInventoryPoller(interval time.Duration) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.poll.mu.Lock()
	s.poll.stop, s.poll.done = cancel, done
	s.poll.mu.Unlock()

	go func() {
		defer close(done)
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.reloadIfChanged()
			}
		}
	}()
}

// stopInventoryPoller stops the check and waits for it to return; it does
// nothing when no check runs.
func (s *Server) stopInventoryPoller() {
	s.poll.mu.Lock()
	stop, done := s.poll.stop, s.poll.done
	s.poll.stop, s.poll.done = nil, nil
	s.poll.mu.Unlock()
	if stop == nil {
		return
	}
	stop()
	<-done
}
