package proxmox

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dembaca/vtpm-mds/internal/inventory"
)

// DefaultRoot is the Proxmox cluster filesystem mount.
const DefaultRoot = "/etc/pve"

// ParseVMConfigs parses this node's Proxmox qemu-server config files into an
// inventory map. pveRoot is the directory that plays /etc/pve; "" means
// DefaultRoot. The node is the one <pveRoot>/local links to, which Proxmox
// maintains; no other node is ever read, because the IMDS bridge does not
// leave this node.
func ParseVMConfigs(pveRoot string) (inventory.VMConfigMap, error) {
	if pveRoot == "" {
		pveRoot = DefaultRoot
	}
	configDir := filepath.Join(pveRoot, "local", "qemu-server")

	// Glob reports no error for a missing directory, so check it first:
	// otherwise a missing local link would load zero VMs without a word.
	if _, err := os.ReadDir(configDir); err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", configDir, err)
	}

	configFiles, err := filepath.Glob(filepath.Join(configDir, "*.conf"))
	if err != nil {
		return nil, fmt.Errorf("failed to list VM config files: %w", err)
	}

	vmConfigMap := make(inventory.VMConfigMap)
	for _, configFile := range configFiles {
		data, err := os.ReadFile(configFile)
		if err != nil {
			continue
		}
		vmid := strings.TrimSuffix(filepath.Base(configFile), ".conf")
		vmConfigMap[vmid] = parseVMConfig(vmid, string(data))
	}

	return vmConfigMap, nil
}

var netPattern = regexp.MustCompile(`^net(\d+)[:=]\s*virtio=([0-9A-Fa-f:]{17})`)

// parseVMConfig reads the live section of one config file: the lines before
// the first section header. Proxmox writes the live configuration first and
// [PENDING], [special:...] and snapshot sections after it, so none of those
// describe the running guest.
func parseVMConfig(vmid, data string) *inventory.VMConfig {
	vmConfig := &inventory.VMConfig{
		VMID:      vmid,
		RawConfig: make(map[string]string),
		MACs:      []string{},
	}

	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			break
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			parts = strings.SplitN(line, "=", 2)
		}
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Proxmox cannot write this key, so a line carrying it was added
			// by hand and is gone after the next edit of the VM.
			if key != "public-ipv4" {
				vmConfig.RawConfig[key] = value
			}
		}

		matches := netPattern.FindStringSubmatch(line)
		if len(matches) == 3 {
			vmConfig.MACs = append(vmConfig.MACs, strings.ToLower(matches[2]))
		}
	}

	if name, ok := vmConfig.RawConfig["name"]; ok {
		vmConfig.Name = name
	}

	return vmConfig
}
