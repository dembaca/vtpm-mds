package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/dembaca/vtpm-mds/internal/config"    // pragma: allowlist secret
	"github.com/dembaca/vtpm-mds/internal/inventory" // pragma: allowlist secret
)

func TestHealthBoundGuestSeesOnlyStatus(t *testing.T) {
	srv := newHealthTestServer(t, false)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req = req.WithContext(context.WithValue(req.Context(), inventory.VMConfigContextKey, &inventory.VMConfig{VMID: "100"}))
	rec := httptest.NewRecorder()
	srv.handleHealth(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 || body["status"] != "ok" {
		t.Fatalf("bound guest body=%v", body)
	}
}

func TestHealthUnboundDetailAndAttestationOffOmitsCA(t *testing.T) {
	srv := newHealthTestServer(t, false)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.handleHealth(rec, req)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status=%v body=%v", body["status"], body)
	}
	if _, ok := body["devid_ca_readable"]; ok {
		t.Fatalf("devid_ca_readable must be omitted when attestation off: %v", body)
	}
	if body["inventory_loaded"] != true {
		t.Fatalf("inventory_loaded=%v", body["inventory_loaded"])
	}
	if int(body["vm_count"].(float64)) < 1 {
		t.Fatalf("vm_count=%v", body["vm_count"])
	}
}

func TestHealthFailedReloadKeepsCount(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.yaml")
	bad := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(good, []byte("vms:\n  - id: \"100\"\n    name: g\n    macs: [\"52:54:00:00:00:01\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("vms:\n  - id: \"100\"\n    name: g\n    macs: [\"52:54:00:00:00:01\"]\n    nope: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	cfg.MDS.EnableTPMAttestation = false
	cfg.MDS.InventoryPath = good
	cfg.MDS.ListenAddr = "127.0.0.1:0"
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	before := srv.vmCount
	if before < 1 {
		t.Fatalf("vmCount=%d", before)
	}
	srv.cfg.MDS.InventoryPath = bad
	if err := srv.RefreshVMConfigCache(); err == nil {
		t.Fatal("expected reload failure")
	}
	if srv.vmCount != before {
		t.Fatalf("vmCount changed %d -> %d", before, srv.vmCount)
	}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.handleHealth(rec, req)
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["status"] != "degraded" {
		t.Fatalf("want degraded, got %v", body)
	}
	if body["last_error"] == "" {
		t.Fatalf("last_error empty: %v", body)
	}
	if int(body["vm_count"].(float64)) != before {
		t.Fatalf("vm_count=%v want %d", body["vm_count"], before)
	}
}

func newHealthTestServer(t *testing.T, attestation bool) *Server {
	t.Helper()
	dir := t.TempDir()
	inv := filepath.Join(dir, "inv.yaml")
	if err := os.WriteFile(inv, []byte("vms:\n  - id: \"100\"\n    name: g\n    macs: [\"52:54:00:00:00:01\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	cfg.MDS.EnableTPMAttestation = attestation
	cfg.MDS.InventoryPath = inv
	cfg.MDS.ListenAddr = "127.0.0.1:0"
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return srv
}
