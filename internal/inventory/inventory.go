package inventory

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// VMConfig holds identity metadata for a guest VM.
type VMConfig struct {
	VMID      string            // Logical VM ID
	Name      string            // Optional human-readable name
	RawConfig map[string]string // Optional key/value metadata
	MACs      []string          // NIC MAC addresses used for caller identification
	// EKSHA256 is an optional pin: lowercase hex SHA-256 of the TPM EK
	// certificate DER. When set, DevID enroll requires a matching EK cert.
	EKSHA256 string
}

// VMConfigMap maps VMID to VMConfig.
type VMConfigMap map[string]*VMConfig

// File format for YAML inventory used by the QEMU lab (and non-Proxmox hosts).
type fileFormat struct {
	VMs []struct {
		ID       string   `yaml:"id"`
		Name     string   `yaml:"name"`
		MACs     []string `yaml:"macs"`
		EKSHA256 string   `yaml:"ek_sha256"`
	} `yaml:"vms"`
}

// LoadYAML reads a YAML inventory file and returns a VMConfigMap.
func LoadYAML(path string) (VMConfigMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read inventory file: %w", err)
	}

	var parsed fileFormat
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse inventory YAML: %w", err)
	}

	out := make(VMConfigMap)
	for _, vm := range parsed.VMs {
		if vm.ID == "" {
			continue
		}
		macs := make([]string, 0, len(vm.MACs))
		for _, mac := range vm.MACs {
			mac = strings.ToLower(strings.TrimSpace(mac))
			if mac != "" {
				macs = append(macs, mac)
			}
		}
		out[vm.ID] = &VMConfig{
			VMID:      vm.ID,
			Name:      vm.Name,
			RawConfig: map[string]string{"name": vm.Name},
			MACs:      macs,
			EKSHA256:  strings.ToLower(strings.TrimSpace(vm.EKSHA256)),
		}
	}

	return out, nil
}

// GetVMIDByMAC looks up a VM ID by MAC address.
func GetVMIDByMAC(vmConfigMap VMConfigMap, mac string) string {
	mac = strings.ToLower(strings.TrimSpace(mac))
	for vmid, vmConfig := range vmConfigMap {
		for _, vmMac := range vmConfig.MACs {
			if strings.ToLower(vmMac) == mac {
				return vmid
			}
		}
	}
	return ""
}

// Context key type for type-safe context values.
type contextKey string

const (
	VMConfigContextKey contextKey = "vm_config"
)

// GetVMConfigFromRequest extracts the VM config from the HTTP request context.
func GetVMConfigFromRequest(r *http.Request) *VMConfig {
	vmConfig, ok := r.Context().Value(VMConfigContextKey).(*VMConfig)
	if ok && vmConfig != nil {
		return vmConfig
	}
	return nil
}
