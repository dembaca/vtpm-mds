package proxmox

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dembaca/vtpm-mds/internal/inventory"
)

// ParseVMConfigs parses Proxmox qemu-server config files into an inventory map.
// If nodeName is empty, it scans /etc/pve/nodes/ for a node with qemu-server.
func ParseVMConfigs(nodeName string) (inventory.VMConfigMap, error) {
	vmConfigMap := make(inventory.VMConfigMap)

	if nodeName == "" {
		nodesDir := "/etc/pve/nodes"
		entries, err := os.ReadDir(nodesDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read nodes directory: %w", err)
		}

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

	netPattern := regexp.MustCompile(`^net(\d+)[:=]\s*virtio=([0-9A-Fa-f:]{17})`)

	for _, configFile := range configFiles {
		vmid := strings.TrimSuffix(filepath.Base(configFile), ".conf")

		data, err := os.ReadFile(configFile)
		if err != nil {
			continue
		}

		vmConfig := &inventory.VMConfig{
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

			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				vmConfig.RawConfig[key] = value
			} else {
				parts = strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					vmConfig.RawConfig[key] = value
				}
			}

			matches := netPattern.FindStringSubmatch(line)
			if len(matches) == 3 {
				mac := strings.ToLower(matches[2])
				vmConfig.MACs = append(vmConfig.MACs, mac)
			}
		}

		if name, ok := vmConfig.RawConfig["name"]; ok {
			vmConfig.Name = name
		}

		vmConfigMap[vmid] = vmConfig
	}

	return vmConfigMap, nil
}
