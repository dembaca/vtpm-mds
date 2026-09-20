package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the service configuration
type Config struct {
	MDS MDSConfig `yaml:"mds"`
}

// MDSConfig holds the metadata service specific configuration
type MDSConfig struct {
	ListenAddr           string `yaml:"listen_addr"`
	JWKSPath             string `yaml:"jwks_path"`
	AttestationCA        string `yaml:"attestation_ca"`
	EKCAChain            string `yaml:"ek_ca_chain"`
	DevIDCACert          string `yaml:"devid_ca_cert"`
	DevIDCAKey           string `yaml:"devid_ca_key"`
	TokenTTL             string `yaml:"token_ttl"`
	JWTTTL               string `yaml:"jwt_ttl"`
	EnableEC2Compat      bool   `yaml:"enable_ec2_compat"`
	EnableTPMAttestation bool   `yaml:"enable_tpm_attestation"`
	TPMDevice            string `yaml:"tpm_device"`
	HostTPMDevice        string `yaml:"host_tpm_device"`
	SigningKeyPath       string `yaml:"signing_key_path"`
	// InventoryPath is a YAML VM inventory (MAC → VM ID). When set, it is
	// preferred over Proxmox /etc/pve parsing so the service can run on plain QEMU.
	InventoryPath string `yaml:"inventory_path"`
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		MDS: MDSConfig{
			ListenAddr:         "169.254.169.1:80",
			JWKSPath:          "/var/lib/vtpm-mds/jwks.json",
			AttestationCA:     "/etc/vtpm-mds/attestation-ca.pem",
			EKCAChain:         "/etc/vtpm-mds/ek-chain.pem",
			DevIDCACert:       "/etc/vtpm-mds/devid-ca.pem",
			DevIDCAKey:        "/etc/vtpm-mds/devid-ca-key.pem",
			TokenTTL:          "60s",
			JWTTTL:            "5m",
			EnableEC2Compat:   true,
			EnableTPMAttestation: true,
			TPMDevice:         "/dev/tpm0",
			HostTPMDevice:     "/dev/tpmrm0",
			SigningKeyPath:    "/var/lib/vtpm-mds/signing-key.pem",
			InventoryPath:     "",
		},
	}
}

// Load reads configuration from file. It starts from DefaultConfig() and
// overlays the file's values on top, so a setting the file omits keeps its
// built-in default and a setting the file states explicitly — including a
// zero value — wins.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate required fields
	if cfg.MDS.ListenAddr == "" {
		return nil, fmt.Errorf("listen_addr is required")
	}

	if _, err := time.ParseDuration(cfg.MDS.TokenTTL); err != nil {
		return nil, fmt.Errorf("invalid token_ttl %q: %w", cfg.MDS.TokenTTL, err)
	}

	return cfg, nil
}

