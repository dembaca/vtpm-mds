package proxmox

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

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
			// Proxmox cannot write this key, so a line carrying one was
			// added by hand and is gone after the next edit of the VM.
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

	vmConfig.Tags = instanceTags(vmid, vmConfig.RawConfig["tags"])

	return vmConfig
}

// tagPrefix starts a Proxmox tag that declares an instance tag:
// vtpm_mds.io.<key>.<value>. Proxmox ignores case in tags, so the prefix is
// matched the same way; the key and the value keep the case they were written in.
const tagPrefix = "vtpm_mds.io."

// legacyPlacementPrefixes began the tags with which a VM once declared its own
// zone and region. They place nothing now, and a VM still carrying one is logged.
var legacyPlacementPrefixes = []string{"mds.zone.", "mds.region."}

type tagEntry struct {
	key, value string
	ok         bool
}

// instanceTags reads the instance tags a VM declares. A key is declared by
// exactly one tag with a valid key and value. Two or more tags for one key, valid
// or not, declare nothing, so a typo next to a good tag is never resolved
// silently in the good tag's favour; a single invalid tag is ignored. Each fault
// is logged and touches only this VM, and a tag without the prefix is none of this
// code's business.
func instanceTags(vmid, tags string) map[string]string {
	fields := strings.FieldsFunc(tags, func(r rune) bool {
		return r == ';' || r == ',' || unicode.IsSpace(r)
	})

	entries := map[string][]tagEntry{}
	var order []string
	for _, tag := range fields {
		lower := strings.ToLower(tag)
		for _, legacy := range legacyPlacementPrefixes {
			if strings.HasPrefix(lower, legacy) {
				log.Printf("VM %s: ignored legacy placement tag %q; topology now comes from the node", vmid, tag)
			}
		}
		if !strings.HasPrefix(lower, tagPrefix) {
			continue
		}
		key, value, hasValue := strings.Cut(tag[len(tagPrefix):], ".")
		e := tagEntry{key: key, value: value, ok: hasValue && inventory.ValidTagKey(key) && inventory.ValidTagValue(value)}
		if _, seen := entries[key]; !seen {
			order = append(order, key)
		}
		entries[key] = append(entries[key], e)
	}

	var out map[string]string
	for _, key := range order {
		list := entries[key]
		switch {
		case len(list) > 1:
			log.Printf("VM %s: %d tags for key %q, none used", vmid, len(list), key)
		case !list[0].ok:
			log.Printf("VM %s: tag %q is not a valid instance tag, ignored", vmid, tagPrefix+key+"."+list[0].value)
		default:
			if out == nil {
				out = make(map[string]string)
			}
			out[key] = list[0].value
		}
	}
	return out
}

var clusterNamePattern = regexp.MustCompile(`(?m)^\s*cluster_name:\s*(\S+)`)

// descriptionLine matches a line of a node description that declares a placement.
var descriptionLine = regexp.MustCompile(`^\s*topology\.vtpm_mds\.io/(zone|region):\s*(\S+)\s*$`)

// HostPlacement returns the placement of the node this service runs on, from
// three sources in this order: the zone and region lines of the node's
// description (topology.vtpm_mds.io/zone and .../region), then the node name,
// the target of <pveRoot>/local, as the zone and the cluster_name of
// <pveRoot>/corosync.conf as the region, then the defaults, "proxmox" and
// "local". Each of the two is resolved on its own, so a description that names a
// zone only still takes the region from the cluster. A name is kept as its source
// spells it, capitals included. One that is not a valid placement name is logged
// and the next source is used. pveRoot "" means DefaultRoot.
func HostPlacement(pveRoot string) inventory.Placement {
	if pveRoot == "" {
		pveRoot = DefaultRoot
	}
	placement := inventory.DefaultPlacement

	if target, err := os.Readlink(filepath.Join(pveRoot, "local")); err != nil {
		log.Printf("host placement: cannot read the %s link: %v", filepath.Join(pveRoot, "local"), err)
	} else if node := filepath.Base(filepath.Clean(target)); inventory.ValidPlacementName(node) {
		placement.Zone = node
	} else {
		log.Printf("host placement: node name %q is not a valid placement name, zone stays %q", node, placement.Zone)
	}

	if data, err := os.ReadFile(filepath.Join(pveRoot, "corosync.conf")); err == nil {
		if m := clusterNamePattern.FindSubmatch(data); m != nil {
			if cluster := string(m[1]); inventory.ValidPlacementName(cluster) {
				placement.Region = cluster
			} else {
				log.Printf("host placement: cluster name %q is not a valid placement name, region stays %q", cluster, placement.Region)
			}
		}
	}

	zone, region := descriptionPlacement(filepath.Join(pveRoot, "local", "config"))
	if zone != "" {
		placement.Zone = zone
	}
	if region != "" {
		placement.Region = region
	}
	return placement
}

// descriptionPlacement reads the zone and region a node declares in its
// description. Proxmox keeps the description in <node>/config as '#' lines, each
// URL-encoded, so "topology.vtpm_mds.io/zone: DUS4" is stored as
// "#topology.vtpm_mds.io/zone%3A DUS4". A kind is declared by exactly one line
// with a valid name; two or more lines of one kind declare nothing, as two tags
// do. A missing file declares nothing and is not an error: most nodes have no
// node config.
func descriptionPlacement(path string) (zone, region string) {
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("host placement: cannot read %s: %v", path, err)
		}
		return "", ""
	}
	found := map[string][]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") {
			continue
		}
		decoded, err := url.PathUnescape(strings.TrimPrefix(line, "#"))
		if err != nil {
			continue
		}
		if m := descriptionLine.FindStringSubmatch(decoded); m != nil {
			found[m[1]] = append(found[m[1]], m[2])
		}
	}
	pick := func(kind string) string {
		values := found[kind]
		switch {
		case len(values) == 0:
			return ""
		case len(values) > 1:
			log.Printf("host placement: %d %s lines in the description of %s, none used", len(values), kind, path)
			return ""
		case !inventory.ValidPlacementName(values[0]):
			log.Printf("host placement: description %s %q is not a valid placement name, ignored", kind, values[0])
			return ""
		}
		return values[0]
	}
	return pick("zone"), pick("region")
}
