package config

import (
	"bytes"
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
	ListenAddr    string `yaml:"listen_addr"`
	JWKSPath      string `yaml:"jwks_path"`      // pragma: allowlist secret
	AttestationCA string `yaml:"attestation_ca"` // pragma: allowlist secret
	EKCAChain     string `yaml:"ek_ca_chain"`    // pragma: allowlist secret
	DevIDCACert   string `yaml:"devid_ca_cert"`  // pragma: allowlist secret
	DevIDCAKey    string `yaml:"devid_ca_key"`   // pragma: allowlist secret
	// DevIDLifetime is the LDevID certificate lifetime (Go duration string).
	// Default 2160h (90 days); values greater than 8760h (one year) are refused.
	DevIDLifetime        string `yaml:"devid_lifetime"`
	TokenTTL             string `yaml:"token_ttl"`
	JWTTTL               string `yaml:"jwt_ttl"`
	EnableEC2Compat      bool   `yaml:"enable_ec2_compat"`
	EnableTPMAttestation bool   `yaml:"enable_tpm_attestation"`
	TPMDevice            string `yaml:"tpm_device"`       // pragma: allowlist secret
	HostTPMDevice        string `yaml:"host_tpm_device"`  // pragma: allowlist secret
	SigningKeyPath       string `yaml:"signing_key_path"` // pragma: allowlist secret
	// InventoryPath is a YAML VM inventory (MAC → VM ID). When set, it is
	// preferred over Proxmox /etc/pve parsing so the service can run on plain QEMU.
	InventoryPath string `yaml:"inventory_path"`
	// RequireVMIdentity makes a caller the VM inventory could not bind to a
	// VM record be refused at the endpoints that report its instance
	// identity. Its built-in default is true; it is only effective because
	// Load starts from DefaultConfig(), so a file that omits the key keeps
	// the secure value. Setting it false is a migration aid for a deployment
	// whose inventory is incomplete: unbound callers are then served an id
	// synthesized from their peer address, which names the connection and is
	// not an identity.
	RequireVMIdentity bool `yaml:"require_vm_identity"`
	// ClusterPinDir holds cluster-wide EK pins, one file per VM ID, written
	// by root outside this service; read only with the Proxmox inventory.
	// Empty turns the cluster source off.
	ClusterPinDir string `yaml:"cluster_pin_dir"`
	// RequireClusterPin refuses enrollment of a VM without a cluster pin
	// instead of falling back to the local first-use pin.
	RequireClusterPin bool `yaml:"require_cluster_pin"`
}

// MaxDevIDLifetime is the longest accepted mds.devid_lifetime (one year of hours).
const MaxDevIDLifetime = 8760 * time.Hour

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		MDS: MDSConfig{
			ListenAddr:           "169.254.169.1:80",
			JWKSPath:             "/var/lib/vtpm-mds/jwks.json",      // pragma: allowlist secret
			AttestationCA:        "/etc/vtpm-mds/attestation-ca.pem", // pragma: allowlist secret
			EKCAChain:            "/etc/vtpm-mds/ek-chain.pem",       // pragma: allowlist secret
			DevIDCACert:          "/etc/vtpm-mds/devid-ca.pem",       // pragma: allowlist secret
			DevIDCAKey:           "/etc/vtpm-mds/devid-ca-key.pem",   // pragma: allowlist secret
			DevIDLifetime:        "2160h",
			TokenTTL:             "60s",
			JWTTTL:               "5m",
			EnableEC2Compat:      true,
			EnableTPMAttestation: true,
			TPMDevice:            "/dev/tpm0",                         // pragma: allowlist secret
			HostTPMDevice:        "/dev/tpmrm0",                       // pragma: allowlist secret
			SigningKeyPath:       "/var/lib/vtpm-mds/signing-key.pem", // pragma: allowlist secret
			InventoryPath:        "",
			RequireVMIdentity:    true,
			ClusterPinDir:        "/etc/pve/vtpm-ek",
			RequireClusterPin:    false,
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
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate required fields
	if cfg.MDS.ListenAddr == "" {
		return nil, fmt.Errorf("listen_addr is required")
	}

	if _, err := time.ParseDuration(cfg.MDS.TokenTTL); err != nil {
		return nil, fmt.Errorf("invalid token_ttl %q: %w", cfg.MDS.TokenTTL, err)
	}

	lifetime, err := time.ParseDuration(cfg.MDS.DevIDLifetime)
	if err != nil {
		return nil, fmt.Errorf("invalid devid_lifetime %q: %w", cfg.MDS.DevIDLifetime, err)
	}
	if lifetime <= 0 {
		return nil, fmt.Errorf("invalid devid_lifetime %q: must be strictly positive", cfg.MDS.DevIDLifetime)
	}
	if lifetime > MaxDevIDLifetime {
		return nil, fmt.Errorf("invalid devid_lifetime %q: must not exceed %s", cfg.MDS.DevIDLifetime, MaxDevIDLifetime)
	}

	if cfg.MDS.RequireClusterPin {
		if cfg.MDS.ClusterPinDir == "" {
			return nil, fmt.Errorf("require_cluster_pin needs cluster_pin_dir")
		}
		if cfg.MDS.InventoryPath != "" {
			return nil, fmt.Errorf("require_cluster_pin cannot be used with inventory_path: cluster pins are read only with the Proxmox inventory")
		}
	}

	return cfg, nil
}
