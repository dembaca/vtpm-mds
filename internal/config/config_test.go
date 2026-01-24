package config

import (
	"os"
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
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("Load should return default config for missing file, got error: %v", err)
	}
	
	if cfg == nil {
		t.Fatal("Load should return default config, got nil")
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
	
	if cfg.MDS.EnableEC2Compat {
		t.Error("EnableEC2Compat should be false from config")
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

