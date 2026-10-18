package inventory

import (
	"fmt"
	"log"
	"net"
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
	// Tags are the instance tags served under /latest/meta-data/tags/instance/.
	// They are kept apart from RawConfig so that a tag can never collide with a
	// Proxmox key such as name, and nil when the VM has none.
	Tags map[string]string
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
		ID         string   `yaml:"id"`
		Name       string   `yaml:"name"`
		MACs       []string `yaml:"macs"`
		EKSHA256   string   `yaml:"ek_sha256"`
		PublicIPv4 string   `yaml:"public_ipv4"`
		// Zone and Region are read only to say they no longer place anything.
		Zone   string            `yaml:"zone"`
		Region string            `yaml:"region"`
		Tags   map[string]string `yaml:"tags"`
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
		raw := map[string]string{"name": vm.Name}
		// A declared public address is validated here rather than where it is
		// served. This inventory is the project's own format and is written by
		// an operator, so a typo is a configuration error: it fails the load
		// loudly instead of degrading to "no public address", which would look
		// identical to not having declared one at all. The precedent is
		// fail-fast-on-unusable-configuration.
		if declared := strings.TrimSpace(vm.PublicIPv4); declared != "" {
			ip := net.ParseIP(declared)
			if ip == nil || ip.To4() == nil {
				return nil, fmt.Errorf("inventory entry %q: public_ipv4 %q is not an IPv4 address", vm.ID, declared)
			}
			raw["public-ipv4"] = ip.To4().String()
		}

		// Topology belongs to the node, not to a VM: a zone or region declared
		// here has no effect, and the log says so once per load.
		for field, declared := range map[string]string{"zone": vm.Zone, "region": vm.Region} {
			if strings.TrimSpace(declared) != "" {
				log.Printf("inventory entry %q: ignored legacy %s %q; topology now comes from the node", vm.ID, field, declared)
			}
		}

		// An invalid tag degrades to one tag fewer, so unlike public_ipv4 it is
		// logged and ignored rather than failing the load.
		var tags map[string]string
		for key, value := range vm.Tags {
			key, value = strings.TrimSpace(key), strings.TrimSpace(value)
			if !ValidTagKey(key) || !ValidTagValue(value) {
				log.Printf("inventory entry %q: tag %q=%q is not a valid instance tag, ignored", vm.ID, key, value)
				continue
			}
			if tags == nil {
				tags = make(map[string]string)
			}
			tags[key] = value
		}

		out[vm.ID] = &VMConfig{
			VMID:      vm.ID,
			Name:      vm.Name,
			RawConfig: raw,
			Tags:      tags,
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
