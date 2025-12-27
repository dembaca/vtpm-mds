package proxmox

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// VMConfig holds the parsed configuration for a VM
type VMConfig struct {
	VMID      string            // VM ID (from filename)
	RawConfig map[string]string // Raw key-value pairs from config file
	MACs      []string          // List of MAC addresses for this VM
}

// VMConfigMap maps VMID to VMConfig
type VMConfigMap map[string]*VMConfig

// ParseVMConfigs parses all VM config files and returns a map of VMID to VMConfig
// If nodeName is empty, it will scan /etc/pve/nodes/ for any node directories
func ParseVMConfigs(nodeName string) (VMConfigMap, error) {
	vmConfigMap := make(VMConfigMap)

	// Auto-detect node name by scanning /etc/pve/nodes/ if not provided
	if nodeName == "" {
		nodesDir := "/etc/pve/nodes"
		entries, err := os.ReadDir(nodesDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read nodes directory: %w", err)
		}

		// Find first directory that contains qemu-server
		for _, entry := range entries {
			if entry.IsDir() {
				qemuServerPath := filepath.Join(nodesDir, entry.Name(), "qemu-server")
				if _, err := os.Stat(qemuServerPath); err == nil {
					nodeName = entry.Name()
					break
				}
			}
		}

		if nodeName == "" {
			return nil, fmt.Errorf("no node directory with qemu-server found in /etc/pve/nodes/")
		}
	}

	configDir := fmt.Sprintf("/etc/pve/nodes/%s/qemu-server", nodeName)
	configFiles, err := filepath.Glob(filepath.Join(configDir, "*.conf"))
	if err != nil {
		return nil, fmt.Errorf("failed to list VM config files: %w", err)
	}

	// Regex to match net entries: net0=virtio=MAC,... or net1: virtio=MAC,...
	netPattern := regexp.MustCompile(`^net(\d+)[:=]\s*virtio=([0-9A-Fa-f:]{17})`)

	for _, configFile := range configFiles {
		// Extract VMID from filename: /path/to/200.conf -> 200
		vmid := strings.TrimSuffix(filepath.Base(configFile), ".conf")

		data, err := os.ReadFile(configFile)
		if err != nil {
			// Skip files we can't read
			continue
		}

		vmConfig := &VMConfig{
			VMID:      vmid,
			RawConfig: make(map[string]string),
			MACs:      []string{},
		}

		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			// Parse key=value pairs
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				// Handle lines like "net0: virtio=..."
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				vmConfig.RawConfig[key] = value
			} else {
				// Handle lines like "net0=virtio=..."
				parts = strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					vmConfig.RawConfig[key] = value
				}
			}

			// Extract MAC addresses from net entries
			matches := netPattern.FindStringSubmatch(line)
			if len(matches) == 3 {
				mac := strings.ToLower(matches[2])
				vmConfig.MACs = append(vmConfig.MACs, mac)
			}
		}

		vmConfigMap[vmid] = vmConfig
	}

	return vmConfigMap, nil
}

// GetVMIDByMAC looks up a VM ID by MAC address in the VM config map
// Returns empty string if not found
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

// Context key type for type-safe context values
type contextKey string

const (
	VMConfigContextKey contextKey = "vm_config"
)

// GetVMConfigFromRequest extracts the VM config from the HTTP request context
func GetVMConfigFromRequest(r *http.Request) *VMConfig {
	vmConfig, ok := r.Context().Value(VMConfigContextKey).(*VMConfig)
	if ok && vmConfig != nil {
		return vmConfig
	}
	return nil
}
