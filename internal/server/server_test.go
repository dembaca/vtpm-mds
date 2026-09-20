package server

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/dembaca/vtpm-mds/internal/config"
)

// TestStartReturnsErrServerClosedOnShutdown reproduces the condition that
// exit-cleanly-on-server-shutdown depends on: once Shutdown is called,
// Start's blocking Serve call must return an error satisfying
// errors.Is(err, http.ErrServerClosed), so that the caller (main.go) has a
// sentinel it can distinguish from a genuine listener failure.
func TestStartReturnsErrServerClosedOnShutdown(t *testing.T) {
	cfg := &config.Config{
		MDS: config.MDSConfig{
			ListenAddr:           "127.0.0.1:0",
			EnableEC2Compat:      false,
			EnableTPMAttestation: false,
		},
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New returned an error: %v", err)
	}

	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- srv.Start()
	}()

	// Give Start a moment to begin serving before shutting it down.
	time.Sleep(100 * time.Millisecond)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown returned an error: %v", err)
	}

	select {
	case startErr := <-startErrCh:
		if !errors.Is(startErr, http.ErrServerClosed) {
			t.Fatalf("Start returned %v, want an error satisfying errors.Is(err, http.ErrServerClosed)", startErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return after Shutdown was called")
	}
}
