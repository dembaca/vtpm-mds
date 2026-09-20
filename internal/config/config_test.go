package config

import (
	"os"
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

