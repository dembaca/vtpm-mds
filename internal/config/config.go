package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the service configuration
type Config struct {
	MDS MDSConfig `yaml:"mds"`
}

// MDSConfig holds the metadata service specific configuration
type MDSConfig struct {
	ListenAddr      string `yaml:"listen_addr"`
	TokenTTL        string `yaml:"token_ttl"`
	EnableEC2Compat bool   `yaml:"enable_ec2_compat"`
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		MDS: MDSConfig{
			ListenAddr:      "169.254.169.1:80",
			TokenTTL:        "60s",
			EnableEC2Compat: true,
		},
	}
}

// Load reads configuration from file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		// If file doesn't exist, return defaults
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate required fields
	if cfg.MDS.ListenAddr == "" {
		return nil, fmt.Errorf("listen_addr is required")
	}

	return &cfg, nil
}

