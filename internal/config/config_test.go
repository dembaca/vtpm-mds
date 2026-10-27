package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MDS.ListenAddr != "169.254.169.1:80" {
		t.Errorf("Expected listen_addr '169.254.169.1:80', got '%s'", cfg.MDS.ListenAddr)
	}

	if !cfg.MDS.EnableEC2Compat {
		t.Error("EnableEC2Compat should be true by default")
	}

	if !cfg.MDS.EnableTPMAttestation {
		t.Error("EnableTPMAttestation should be true by default")
	}

	// The refusal of unbound callers is on unless an operator turns it off
	// (task 2.1).
	if !cfg.MDS.RequireVMIdentity {
		t.Error("RequireVMIdentity should be true by default")
	}
}

// TestLoadOmittedRequireVMIdentityDefaultsToTrue pins the secure default
// against the defaults merge being lost (task 2.2). If Load ever stops
// starting from DefaultConfig(), a configuration file that does not mention
// require_vm_identity would load it as the Go zero value — false — and
// silently serve unbound callers an instance identity again. That is the
// defect this repository already shipped once with enable_ec2_compat.
func TestLoadOmittedRequireVMIdentityDefaultsToTrue(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Deliberately says nothing about require_vm_identity. Everything Load
	// validates is stated, so that a lost defaults merge fails on the
	// setting under test rather than earlier.
	configData := `mds:
  listen_addr: "127.0.0.1:8080"
  token_ttl: "60s"
  enable_ec2_compat: true`

	if _, err := tmpFile.WriteString(configData); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !cfg.MDS.RequireVMIdentity {
		t.Error("RequireVMIdentity must keep its built-in default (true) when the file omits it")
	}
}

// TestLoadExplicitRequireVMIdentityFalse verifies an operator can state the
// zero value and have it win over the default, which is what makes the
// migration aid usable (task 2.3).
func TestLoadExplicitRequireVMIdentityFalse(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	configData := `mds:
  listen_addr: "127.0.0.1:8080"
  require_vm_identity: false`

	if _, err := tmpFile.WriteString(configData); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.MDS.RequireVMIdentity {
		t.Error("RequireVMIdentity should be false when the file states it explicitly")
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("Load should return an error for a missing file")
	}
}

func TestLoadValidFile(t *testing.T) {
	// Create temporary config file
	tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	configData := `mds:
  listen_addr: "127.0.0.1:8080"
  enable_ec2_compat: false`

	if _, err := tmpFile.WriteString(configData); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.MDS.ListenAddr != "127.0.0.1:8080" {
		t.Errorf("Expected listen_addr '127.0.0.1:8080', got '%s'", cfg.MDS.ListenAddr)
	}

	// An explicit `enable_ec2_compat: false` must still win over the built-in
	// default of true, so an operator can state a zero value.
	if cfg.MDS.EnableEC2Compat {
		t.Error("EnableEC2Compat should be false when the file states it explicitly")
	}
}

func TestLoadMergesDefaultsForOmittedFields(t *testing.T) {
	// Create temporary config file that sets only listen_addr.
	tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	configData := `mds:
  listen_addr: "127.0.0.1:8080"`

	if _, err := tmpFile.WriteString(configData); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.MDS.ListenAddr != "127.0.0.1:8080" {
		t.Errorf("Expected listen_addr '127.0.0.1:8080', got '%s'", cfg.MDS.ListenAddr)
	}

	// Omitting enable_ec2_compat must keep the built-in default (true),
	// not fall back to the Go zero value (false).
	if !cfg.MDS.EnableEC2Compat {
		t.Error("EnableEC2Compat should keep its built-in default (true) when omitted from the file")
	}

	// Omitting token_ttl must keep the built-in default ("60s"), not "".
	if cfg.MDS.TokenTTL != "60s" {
		t.Errorf("Expected token_ttl to keep its default '60s' when omitted, got '%s'", cfg.MDS.TokenTTL)
	}
}

func TestLoadEmptyListenAddrIsRejected(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// An explicitly empty listen_addr must still fail the load, even though
	// an omitted listen_addr now takes the built-in default.
	configData := `mds:
  listen_addr: ""`

	if _, err := tmpFile.WriteString(configData); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	_, err = Load(tmpFile.Name())
	if err == nil {
		t.Fatal("Load should return an error for an explicitly empty listen_addr")
	}
	if err.Error() != "listen_addr is required" {
		t.Errorf("Expected error 'listen_addr is required', got '%v'", err)
	}
}

func TestLoadOmittedListenAddrTakesDefault(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	configData := `mds:
  enable_ec2_compat: false`

	if _, err := tmpFile.WriteString(configData); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.MDS.ListenAddr != "169.254.169.1:80" {
		t.Errorf("Expected listen_addr to keep its built-in default when omitted, got '%s'", cfg.MDS.ListenAddr)
	}
}

func TestLoadRejectsUnparseableTokenTTL(t *testing.T) {
	for _, ttl := range []string{"60", "sixty"} {
		t.Run(ttl, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			configData := "mds:\n  listen_addr: \"127.0.0.1:8080\"\n  token_ttl: \"" + ttl + "\"\n"
			if _, err := tmpFile.WriteString(configData); err != nil {
				t.Fatalf("Failed to write config: %v", err)
			}
			tmpFile.Close()

			_, err = Load(tmpFile.Name())
			if err == nil {
				t.Fatalf("Load should fail for an unparseable token_ttl %q", ttl)
			}
			if !strings.Contains(err.Error(), "token_ttl") || !strings.Contains(err.Error(), ttl) {
				t.Errorf("Expected error naming token_ttl and the offending value %q, got '%v'", ttl, err)
			}
		})
	}
}

func TestLoadAcceptsValidTokenTTL(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	configData := `mds:
  listen_addr: "127.0.0.1:8080"
  token_ttl: "60s"`

	if _, err := tmpFile.WriteString(configData); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load should succeed for a valid token_ttl: %v", err)
	}
	if cfg.MDS.TokenTTL != "60s" {
		t.Errorf("Expected token_ttl '60s', got '%s'", cfg.MDS.TokenTTL)
	}
}

func TestLoadOmittedTokenTTLPassesValidationAndTakesDefault(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	configData := `mds:
  listen_addr: "127.0.0.1:8080"`

	if _, err := tmpFile.WriteString(configData); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load should succeed when token_ttl is omitted: %v", err)
	}
	if cfg.MDS.TokenTTL != "60s" {
		t.Errorf("Expected omitted token_ttl to take the built-in default '60s', got '%s'", cfg.MDS.TokenTTL)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-invalid-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	invalidYAML := "this is not valid yaml: ["
	if _, err := tmpFile.WriteString(invalidYAML); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	_, err = Load(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestLoadRejectsUnknownKey(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-config-unknown-*.yaml")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString(`mds:
  listen_addr: "127.0.0.1:8080"
  listen_addres: "typo"
`); err != nil {
		t.Fatalf("write: %v", err)
	}
	tmpFile.Close()
	if _, err := Load(tmpFile.Name()); err == nil {
		t.Fatal("expected error for unknown key listen_addres")
	}
}

func TestLoadShippedDebianConfig(t *testing.T) {
	cfg, err := Load("../../debian/config.yaml")
	if err != nil {
		t.Fatalf("debian/config.yaml must strict-decode: %v", err)
	}
	if cfg.MDS.ListenAddr == "" {
		t.Fatal("listen_addr empty after load")
	}
}

func TestLoadRootExampleConfig(t *testing.T) {
	cfg, err := Load("../../config.yaml")
	if err != nil {
		t.Fatalf("config.yaml must strict-decode: %v", err)
	}
	if cfg.MDS.ListenAddr == "" {
		t.Fatal("listen_addr empty after load")
	}
}


func TestLoadOmittedDevIDLifetimeTakesDefault(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	configData := `mds:
  listen_addr: "127.0.0.1:8080"`

	if _, err := tmpFile.WriteString(configData); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load should succeed when devid_lifetime is omitted: %v", err)
	}
	if cfg.MDS.DevIDLifetime != "2160h" {
		t.Errorf("Expected omitted devid_lifetime to take default '2160h', got %q", cfg.MDS.DevIDLifetime)
	}
}

func TestLoadRejectsBadDevIDLifetime(t *testing.T) {
	for _, lifetime := range []string{"90", "0s", "9000h", "8761h"} {
		t.Run(lifetime, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			configData := "mds:\n  listen_addr: \"127.0.0.1:8080\"\n  devid_lifetime: \"" + lifetime + "\"\n"
			if _, err := tmpFile.WriteString(configData); err != nil {
				t.Fatalf("Failed to write config: %v", err)
			}
			tmpFile.Close()

			_, err = Load(tmpFile.Name())
			if err == nil {
				t.Fatalf("Load should fail for devid_lifetime %q", lifetime)
			}
			if !strings.Contains(err.Error(), "devid_lifetime") || !strings.Contains(err.Error(), lifetime) {
				t.Errorf("Expected error naming devid_lifetime and %q, got %v", lifetime, err)
			}
		})
	}
}

func TestLoadAcceptsMaxDevIDLifetime(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	configData := `mds:
  listen_addr: "127.0.0.1:8080"
  devid_lifetime: "8760h"`

	if _, err := tmpFile.WriteString(configData); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load should accept devid_lifetime 8760h: %v", err)
	}
	if cfg.MDS.DevIDLifetime != "8760h" {
		t.Errorf("Expected devid_lifetime '8760h', got %q", cfg.MDS.DevIDLifetime)
	}
}

func loadFromString(t *testing.T, data string) (*Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return Load(path)
}

func TestLoadClusterPinDefaults(t *testing.T) {
	cfg, err := loadFromString(t, "mds:\n  listen_addr: \"127.0.0.1:8080\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MDS.ClusterPinDir != "/etc/pve/vtpm-ek" || cfg.MDS.RequireClusterPin {
		t.Fatalf("defaults: cluster_pin_dir=%q require_cluster_pin=%v", cfg.MDS.ClusterPinDir, cfg.MDS.RequireClusterPin)
	}
	cfg, err = loadFromString(t, "mds:\n  cluster_pin_dir: \"/srv/pins\"\n  require_cluster_pin: true\n")
	if err != nil || cfg.MDS.ClusterPinDir != "/srv/pins" || !cfg.MDS.RequireClusterPin {
		t.Fatalf("explicit: cfg=%+v err=%v", cfg.MDS, err)
	}
}

func TestLoadRejectsRequireClusterPinWithoutSource(t *testing.T) {
	for conflict, data := range map[string]string{
		"cluster_pin_dir": "mds:\n  cluster_pin_dir: \"\"\n  require_cluster_pin: true\n",
		"inventory_path":  "mds:\n  inventory_path: \"/etc/vtpm-mds/inventory.yaml\"\n  require_cluster_pin: true\n",
	} {
		_, err := loadFromString(t, data)
		if err == nil || !strings.Contains(err.Error(), "require_cluster_pin") || !strings.Contains(err.Error(), conflict) {
			t.Fatalf("%s: want an error naming require_cluster_pin and %s, got %v", conflict, conflict, err)
		}
	}
}
