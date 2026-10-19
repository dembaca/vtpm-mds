package proxmox

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// NIC is one net<n> line of a guest's live configuration.
type NIC struct {
	VMID   string
	Index  int
	Model  string
	MAC    string // lowercase; "" when the line carries no valid MAC
	Bridge string
}

// Tap is the name Proxmox gives the host side of the NIC: tap<vmid>i<n>.
func (n NIC) Tap() string {
	return fmt.Sprintf("tap%si%d", n.VMID, n.Index)
}

var (
	nicKeyPattern = regexp.MustCompile(`^net(\d+)$`)
	macPattern    = regexp.MustCompile(`^[0-9A-Fa-f]{2}(:[0-9A-Fa-f]{2}){5}$`)
	vmidPattern   = regexp.MustCompile(`^[1-9][0-9]*$`)
)

// GuestNICs lists every NIC of every guest under this node's qemu-server
// directory (pveRoot "" means DefaultRoot), any model, from the live section of
// each config only, sorted by VM id and NIC index. Unlike ParseVMConfigs it
// keeps the index and the bridge, and it does not skip models other than
// virtio: a port has to be bound whatever the guest emulates.
func GuestNICs(pveRoot string) ([]NIC, error) {
	if pveRoot == "" {
		pveRoot = DefaultRoot
	}
	configDir := filepath.Join(pveRoot, "local", "qemu-server")
	if _, err := os.ReadDir(configDir); err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", configDir, err)
	}
	files, err := filepath.Glob(filepath.Join(configDir, "*.conf"))
	if err != nil {
		return nil, fmt.Errorf("failed to list VM config files: %w", err)
	}

	var nics []NIC
	for _, f := range files {
		vmid := strings.TrimSuffix(filepath.Base(f), ".conf")
		if !vmidPattern.MatchString(vmid) {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", f, err)
		}
		nics = append(nics, parseNICs(vmid, string(data))...)
	}
	sort.Slice(nics, func(i, j int) bool {
		a, _ := strconv.Atoi(nics[i].VMID)
		b, _ := strconv.Atoi(nics[j].VMID)
		if a != b {
			return a < b
		}
		return nics[i].Index < nics[j].Index
	})
	return nics, nil
}

// parseNICs reads the net<n> lines before the first section header, as
// parseVMConfig does.
func parseNICs(vmid, data string) []NIC {
	var nics []NIC
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		m := nicKeyPattern.FindStringSubmatch(strings.TrimSpace(key))
		if m == nil {
			continue
		}
		index, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		nics = append(nics, parseNIC(vmid, index, strings.TrimSpace(value)))
	}
	return nics
}

// parseNIC reads "<model>=<MAC>,bridge=<name>,…", and the older
// "model=<model>,macaddr=<MAC>,…" form.
func parseNIC(vmid string, index int, value string) NIC {
	nic := NIC{VMID: vmid, Index: index}
	mac := ""
	for _, field := range strings.Split(value, ",") {
		k, v, _ := strings.Cut(strings.TrimSpace(field), "=")
		switch {
		case k == "bridge":
			nic.Bridge = v
		case k == "model":
			nic.Model = v
		case k == "macaddr":
			mac = v
		case mac == "" && macPattern.MatchString(v):
			// <model>=<MAC>: whatever the model is called, its value is
			// the MAC.
			nic.Model, mac = k, v
		}
	}
	if macPattern.MatchString(mac) {
		nic.MAC = strings.ToLower(mac)
	}
	return nic
}
