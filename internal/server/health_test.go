package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dembaca/vtpm-mds/internal/config"    // pragma: allowlist secret
	"github.com/dembaca/vtpm-mds/internal/devid"     // pragma: allowlist secret
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

	// Bound guest still resolves from the kept cache (instance-id path).
	vmConfigCacheMu.RLock()
	vmid := inventory.GetVMIDByMAC(vmConfigCache, "52:54:00:00:00:01")
	kept, ok := vmConfigCache[vmid]
	vmConfigCacheMu.RUnlock()
	if !ok || vmid != "100" || kept.VMID != "100" {
		t.Fatalf("kept cache miss after failed reload: vmid=%q ok=%v", vmid, ok)
	}
	bound := httptest.NewRequest(http.MethodGet, "/health", nil)
	bound = bound.WithContext(context.WithValue(bound.Context(), inventory.VMConfigContextKey, kept))
	rec = httptest.NewRecorder()
	srv.handleHealth(rec, bound)
	var boundBody map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &boundBody); err != nil {
		t.Fatal(err)
	}
	if len(boundBody) != 1 || boundBody["status"] != "degraded" {
		t.Fatalf("bound guest while degraded: %v", boundBody)
	}
	if id := fmt.Sprintf("i-%s", kept.VMID); id != "i-100" {
		t.Fatalf("instance-id from kept VM: %s", id)
	}
}

func TestHealthAttestationOnDevIDNilDespiteReadableCA(t *testing.T) {
	dir := t.TempDir()
	inv := filepath.Join(dir, "inv.yaml")
	cert := filepath.Join(dir, "ca.pem")
	key := filepath.Join(dir, "ca.key")
	if err := os.WriteFile(inv, []byte("vms:\n  - id: \"100\"\n    name: g\n    macs: [\"52:54:00:00:00:01\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cert, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(key, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	cfg.MDS.EnableTPMAttestation = true
	cfg.MDS.InventoryPath = inv
	cfg.MDS.DevIDCACert = cert
	cfg.MDS.DevIDCAKey = key
	cfg.MDS.ListenAddr = "127.0.0.1:0"
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Readable files but enrollment not loaded: must not report CA ok.
	srv.devid = nil

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.handleHealth(rec, req)
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["status"] != "degraded" {
		t.Fatalf("devid nil want degraded, got %v", body)
	}
	if body["devid_ca_readable"] != false {
		t.Fatalf("devid_ca_readable=%v want false", body["devid_ca_readable"])
	}
}

func TestHealthAttestationOnUnreadableCAMode(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("chmod 0o000 is still readable as uid 0")
	}
	dir := t.TempDir()
	inv := filepath.Join(dir, "inv.yaml")
	cert := filepath.Join(dir, "ca.pem")
	key := filepath.Join(dir, "ca.key")
	if err := os.WriteFile(inv, []byte("vms:\n  - id: \"100\"\n    name: g\n    macs: [\"52:54:00:00:00:01\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cert, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(key, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	cfg.MDS.EnableTPMAttestation = true
	cfg.MDS.InventoryPath = inv
	cfg.MDS.DevIDCACert = cert
	cfg.MDS.DevIDCAKey = key
	cfg.MDS.ListenAddr = "127.0.0.1:0"
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	srv.devid = &devid.Enroller{}
	if err := os.Chmod(cert, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cert, 0o644) })

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.handleHealth(rec, req)
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["devid_ca_readable"] != false {
		t.Fatalf("chmod 0o000 cert want devid_ca_readable=false, got %v", body)
	}
	if body["status"] != "degraded" {
		t.Fatalf("want degraded, got %v", body)
	}
}

func TestHealthAttestationOnMissingCAAndCache(t *testing.T) {
	dir := t.TempDir()
	inv := filepath.Join(dir, "inv.yaml")
	cert := filepath.Join(dir, "ca.pem")
	key := filepath.Join(dir, "ca.key")
	if err := os.WriteFile(inv, []byte("vms:\n  - id: \"100\"\n    name: g\n    macs: [\"52:54:00:00:00:01\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	cfg.MDS.EnableTPMAttestation = true
	cfg.MDS.InventoryPath = inv
	cfg.MDS.DevIDCACert = cert
	cfg.MDS.DevIDCAKey = key
	cfg.MDS.ListenAddr = "127.0.0.1:0"
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// New leaves devid nil when CA files are missing; force a non-nil enroller
	// so the check exercises open/close rather than the nil short-circuit alone.
	srv.devid = &devid.Enroller{}

	var now atomic.Int64
	base := time.Now()
	srv.now = func() time.Time { return base.Add(time.Duration(now.Load())) }

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.handleHealth(rec, req)
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["status"] != "degraded" {
		t.Fatalf("missing CA want degraded, got %v", body)
	}
	if body["devid_ca_readable"] != false {
		t.Fatalf("devid_ca_readable=%v", body["devid_ca_readable"])
	}
	if body["last_error"] != errClassDevIDCAUnreadable {
		t.Fatalf("last_error=%v", body["last_error"])
	}

	if err := os.WriteFile(cert, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(key, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Still within 5s cache of false
	now.Store(int64(2 * time.Second))
	rec = httptest.NewRecorder()
	srv.handleHealth(rec, req)
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["devid_ca_readable"] != false {
		t.Fatalf("cached false should hold at +2s: %v", body)
	}

	now.Store(int64(6 * time.Second))
	rec = httptest.NewRecorder()
	srv.handleHealth(rec, req)
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["devid_ca_readable"] != true {
		t.Fatalf("after cache expiry want true: %v", body)
	}
	if body["status"] != "ok" {
		t.Fatalf("after cache expiry want ok: %v", body)
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
