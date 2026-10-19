package server

import (
	"path/filepath"
	"testing"

	"github.com/dembaca/vtpm-mds/internal/config" // pragma: allowlist secret
)

func TestEkPinDirUsesStateDirectory(t *testing.T) {
	t.Setenv("STATE_DIRECTORY", "/run/state/pkg")
	if got := ekPinDir(); got != "/run/state/pkg/ek-pins" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("STATE_DIRECTORY", "/a/b:/c/d")
	if got := ekPinDir(); got != "/a/b/ek-pins" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("STATE_DIRECTORY", "")
	want := filepath.Join(filepath.Dir(config.DefaultConfig().MDS.JWKSPath), "ek-pins")
	if got := ekPinDir(); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
