package reconcile

import (
	"bytes"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Default and minimum time between passes.
const (
	DefaultInterval = 60 * time.Second
	MinInterval     = 10 * time.Second
)

// Config is the reconciler's configuration file.
type Config struct {
	TrustDomain string          `yaml:"trust_domain"`
	SPIRESocket string          `yaml:"spire_socket"`
	Interval    time.Duration   `yaml:"interval"`
	Clusters    []ClusterConfig `yaml:"clusters"`
}

// ClusterConfig is one Proxmox cluster of the configuration file.
type ClusterConfig struct {
	Name          string   `yaml:"name"`
	APIURL        string   `yaml:"api_url"`
	CAFile        string   `yaml:"ca_file"`
	TokenFile     string   `yaml:"token_file"`
	Intermediates []string `yaml:"intermediates"`

	// Fingerprints is computed by LoadConfig: the lowercase hex SHA-1 of each
	// intermediate's DER, in the order of Intermediates.
	Fingerprints []string `yaml:"-"`
}

// validSegmentChars reports whether s is non-empty, uses only a-z 0-9 . - _
// and is not "." or "..".
func validSegmentChars(s string) bool { return validSegment(s) }

// LoadConfig reads, strictly decodes and validates the configuration at path,
// and computes the fingerprints of every cluster's intermediates.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if cfg.Interval == 0 {
		cfg.Interval = DefaultInterval
	}
	if cfg.TrustDomain == "" || !validSegmentChars(cfg.TrustDomain) {
		return nil, fmt.Errorf("config %s: trust_domain %q must be non-empty and use only a-z 0-9 . - _", path, cfg.TrustDomain)
	}
	if cfg.SPIRESocket == "" {
		return nil, fmt.Errorf("config %s: spire_socket is empty", path)
	}
	if !filepath.IsAbs(cfg.SPIRESocket) {
		return nil, fmt.Errorf("config %s: spire_socket %q must be an absolute path", path, cfg.SPIRESocket)
	}
	if cfg.Interval < MinInterval {
		return nil, fmt.Errorf("config %s: interval %s is below the minimum %s", path, cfg.Interval, MinInterval)
	}
	if len(cfg.Clusters) == 0 {
		return nil, fmt.Errorf("config %s: no clusters", path)
	}
	names := map[string]bool{}
	fpOwner := map[string]string{}
	for i := range cfg.Clusters {
		c := &cfg.Clusters[i]
		if !validSegmentChars(c.Name) {
			return nil, fmt.Errorf("config %s: cluster name %q must use only a-z 0-9 . - _ and not be . or ..", path, c.Name)
		}
		if names[c.Name] {
			return nil, fmt.Errorf("config %s: duplicate cluster name %q", path, c.Name)
		}
		names[c.Name] = true
		if !strings.HasPrefix(c.APIURL, "https://") {
			return nil, fmt.Errorf("config %s: cluster %s: api_url must start with https://", path, c.Name)
		}
		if c.TokenFile == "" {
			return nil, fmt.Errorf("config %s: cluster %s: token_file is empty", path, c.Name)
		}
		if len(c.Intermediates) == 0 {
			return nil, fmt.Errorf("config %s: cluster %s: no intermediates", path, c.Name)
		}
		seenFile := map[string]bool{}
		seenFP := map[string]bool{}
		var files []string
		for _, f := range c.Intermediates {
			if seenFile[f] {
				continue
			}
			seenFile[f] = true
			fp, err := intermediateFingerprint(f)
			if err != nil {
				return nil, fmt.Errorf("cluster %s: %w", c.Name, err)
			}
			if seenFP[fp] {
				continue
			}
			seenFP[fp] = true
			files = append(files, f)
			if other, ok := fpOwner[fp]; ok && other != c.Name {
				return nil, fmt.Errorf("fingerprint %s (%s) is configured for clusters %s and %s", fp, f, other, c.Name)
			}
			fpOwner[fp] = c.Name
			c.Fingerprints = append(c.Fingerprints, fp)
		}
		c.Intermediates = files
	}
	return &cfg, nil
}

// intermediateFingerprint reads the first certificate of a PEM file, refuses a
// self-signed one and returns the lowercase hex SHA-1 of its DER.
func intermediateFingerprint(file string) (string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("read intermediate %s: %w", file, err)
	}
	for {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			return "", fmt.Errorf("intermediate %s: no certificate in PEM", file)
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return "", fmt.Errorf("intermediate %s: %w", file, err)
		}
		if bytes.Equal(cert.RawSubject, cert.RawIssuer) && cert.CheckSignatureFrom(cert) == nil {
			return "", fmt.Errorf("intermediate %s is self-signed (a root), not an intermediate", file)
		}
		sum := sha1.Sum(cert.Raw)
		return hex.EncodeToString(sum[:]), nil
	}
}

// Client builds the Proxmox client for c: it reads TokenFile (trimmed; an empty
// one is an error naming the file, never its content) and, when CAFile is set,
// trusts only the certificates in that file.
func (c ClusterConfig) Client() (*ProxmoxClient, error) {
	raw, err := os.ReadFile(c.TokenFile)
	if err != nil {
		return nil, fmt.Errorf("cluster %s: read token file: %w", c.Name, err)
	}
	token := strings.TrimSpace(string(raw))
	if token == "" {
		return nil, fmt.Errorf("cluster %s: token file %s is empty", c.Name, c.TokenFile)
	}
	tr := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
	if c.CAFile != "" {
		pemData, err := os.ReadFile(c.CAFile)
		if err != nil {
			return nil, fmt.Errorf("cluster %s: read ca_file: %w", c.Name, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pemData) {
			return nil, fmt.Errorf("cluster %s: ca_file %s holds no certificate", c.Name, c.CAFile)
		}
		tr.TLSClientConfig.RootCAs = pool
	}
	return &ProxmoxClient{BaseURL: c.APIURL, Token: token, HTTP: &http.Client{Transport: tr, Timeout: 30 * time.Second}}, nil
}
