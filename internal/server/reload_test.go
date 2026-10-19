package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dembaca/vtpm-mds/internal/config"
	"github.com/dembaca/vtpm-mds/internal/inventory"
)

const (
	oneVM  = "vms:\n  - id: \"100\"\n    name: a\n    macs: [\"52:54:00:00:00:01\"]\n"
	twoVMs = oneVM + "  - id: \"101\"\n    name: b\n    macs: [\"52:54:00:00:00:02\", \"52:54:00:00:00:03\"]\n"
	broken = oneVM + "    nope: 1\n"
)

func yamlServer(t *testing.T, content string) (*Server, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "inventory.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	cfg.MDS.EnableTPMAttestation = false
	cfg.MDS.InventoryPath = path
	cfg.MDS.ListenAddr = "127.0.0.1:0"
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return srv, path
}

func cachedVMFor(mac string) string {
	vmConfigCacheMu.RLock()
	defer vmConfigCacheMu.RUnlock()
	return inventory.GetVMIDByMAC(vmConfigCache, mac)
}

func rewrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAnAddedVMIsLoadedWithoutSIGHUP(t *testing.T) {
	srv, path := yamlServer(t, oneVM)
	if got := cachedVMFor("52:54:00:00:00:02"); got != "" {
		t.Fatalf("VM 101 bound before it exists: %q", got)
	}
	rewrite(t, path, twoVMs)
	if !srv.reloadIfChanged() {
		t.Fatal("a changed inventory was not reloaded")
	}
	if got := cachedVMFor("52:54:00:00:00:03"); got != "101" {
		t.Fatalf("new MAC bound to %q, want 101", got)
	}
}

func TestNothingChangesNothingIsLogged(t *testing.T) {
	srv, _ := yamlServer(t, oneVM)
	logs := captureLog(t)
	for i := 0; i < 3; i++ {
		if srv.reloadIfChanged() {
			t.Fatalf("check %d reloaded an unchanged inventory", i)
		}
	}
	if logs.Len() != 0 {
		t.Fatalf("unchanged checks logged: %q", logs.String())
	}
}

func TestABrokenSourceKeepsTheInventoryAndIsLoggedOnce(t *testing.T) {
	srv, path := yamlServer(t, oneVM)
	logs := captureLog(t)

	rewrite(t, path, broken)
	if !srv.reloadIfChanged() {
		t.Fatal("a changed (broken) inventory was not attempted")
	}
	for i := 0; i < 3; i++ {
		if srv.reloadIfChanged() {
			t.Fatalf("check %d retried an unchanged broken inventory", i)
		}
	}
	if n := strings.Count(logs.String(), "Failed to load VM configs"); n != 1 {
		t.Fatalf("failure logged %d times, want 1: %q", n, logs.String())
	}
	if got := cachedVMFor("52:54:00:00:00:01"); got != "100" {
		t.Fatalf("previous inventory lost: MAC bound to %q", got)
	}

	rewrite(t, path, twoVMs)
	if !srv.reloadIfChanged() {
		t.Fatal("the fixed inventory was not reloaded")
	}
	if got := cachedVMFor("52:54:00:00:00:02"); got != "101" {
		t.Fatalf("fixed inventory not loaded: MAC bound to %q", got)
	}
}

// SIGHUP records the fingerprint, so the next check does not reload again.
func TestSIGHUPRecordsTheFingerprint(t *testing.T) {
	srv, path := yamlServer(t, oneVM)
	rewrite(t, path, twoVMs)
	if err := RefreshActiveVMConfigCache(); err != nil {
		t.Fatal(err)
	}
	if srv.reloadIfChanged() {
		t.Fatal("the check reloaded what SIGHUP had just loaded")
	}
}

func TestThePollerReloadsAndStops(t *testing.T) {
	srv, path := yamlServer(t, oneVM)
	srv.startInventoryPoller(10 * time.Millisecond)
	rewrite(t, path, twoVMs)

	deadline := time.Now().Add(2 * time.Second)
	for cachedVMFor("52:54:00:00:00:02") != "101" {
		if time.Now().After(deadline) {
			srv.stopInventoryPoller()
			t.Fatal("the poller did not load the added VM")
		}
		time.Sleep(5 * time.Millisecond)
	}

	stopped := make(chan struct{})
	go func() { srv.stopInventoryPoller(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("stopInventoryPoller did not return")
	}
	// After the stop nothing reloads: a further change stays unloaded.
	rewrite(t, path, oneVM)
	time.Sleep(50 * time.Millisecond)
	if got := cachedVMFor("52:54:00:00:00:02"); got != "101" {
		t.Fatalf("a stopped poller still reloaded: MAC bound to %q", got)
	}
	srv.stopInventoryPoller() // a second stop is a no-op
}

// Start runs the check and Shutdown stops it and waits for it.
func TestStartRunsThePollerAndShutdownStopsIt(t *testing.T) {
	srv, _ := yamlServer(t, oneVM)
	go func() { _ = srv.Start() }()

	var done chan struct{}
	deadline := time.Now().Add(2 * time.Second)
	for done == nil {
		srv.poll.mu.Lock()
		done = srv.poll.done
		srv.poll.mu.Unlock()
		if done == nil {
			if time.Now().After(deadline) {
				t.Fatal("Start did not start the inventory check")
			}
			time.Sleep(5 * time.Millisecond)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	select {
	case <-done:
	default:
		t.Fatal("Shutdown returned with the inventory check still running")
	}
}
