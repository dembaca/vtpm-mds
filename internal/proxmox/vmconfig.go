package proxmox

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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

// descriptionLine matches a line of a node description that declares a topology
// key: the prefix, a key up to the first colon, and a value. The prefix is matched
// as it is spelled, case included.
var descriptionLine = regexp.MustCompile(`^\s*topology\.vtpm_mds\.io/([^:]*):\s*(.*?)\s*$`)

// maxTopologyKeys bounds what a hand-edited description can make the service read
// and list.
const maxTopologyKeys = 64

// HostTopology returns the topology of the node this service runs on. Each key
// takes the first value that is valid from its sources: availability-zone from the
// description line topology.vtpm_mds.io/zone (or .../availability-zone), then the
// node name, the target of <pveRoot>/local, then "proxmox"; region from the
// description, then the cluster_name of <pveRoot>/corosync.conf, then "local";
// node from the description, then the node name; cluster from the description,
// then the cluster_name; every other key from the description only. Each key is
// resolved on its own. A value is kept as its source spells it, capitals
// included; one that is not valid is logged and the next source is used.
// pveRoot "" means DefaultRoot.
func HostTopology(pveRoot string) inventory.Topology {
	if pveRoot == "" {
		pveRoot = DefaultRoot
	}
	topology := inventory.DefaultTopology()

	if target, err := os.Readlink(filepath.Join(pveRoot, "local")); err != nil {
		log.Printf("host topology: cannot read the %s link: %v", filepath.Join(pveRoot, "local"), err)
	} else if node := filepath.Base(filepath.Clean(target)); inventory.ValidPlacementName(node) {
		topology["availability-zone"] = node
		topology["node"] = node
	} else {
		log.Printf("host topology: node name %q is not a valid value, availability-zone stays %q", node, topology["availability-zone"])
	}

	if data, err := os.ReadFile(filepath.Join(pveRoot, "corosync.conf")); err == nil {
		if m := clusterNamePattern.FindSubmatch(data); m != nil {
			if cluster := string(m[1]); inventory.ValidPlacementName(cluster) {
				topology["region"] = cluster
				topology["cluster"] = cluster
			} else {
				log.Printf("host topology: cluster name %q is not a valid value, region stays %q", cluster, topology["region"])
			}
		}
	}

	for key, value := range descriptionTopology(filepath.Join(pveRoot, "local", "config")) {
		topology[key] = value
	}
	return topology
}

// descriptionTopology reads the topology keys a node declares in its description.
// Proxmox keeps the description in <node>/config as '#' lines, each URL-encoded, so
// "topology.vtpm_mds.io/zone: AZ4" is stored as "#topology.vtpm_mds.io/zone%3A AZ4".
// A key is declared by exactly one line with a valid key and value; two or more
// lines for one key, valid or not, declare nothing, as two tags do, and zone and
// availability-zone are one key. Keys are case-sensitive and only exactly "zone"
// is the short form. A missing file declares nothing and is not an error: most
// nodes have no node config.
func descriptionTopology(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("host topology: cannot read %s: %v", path, err)
		}
		return nil
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
		m := descriptionLine.FindStringSubmatch(decoded)
		if m == nil {
			continue
		}
		key, value := m[1], m[2]
		if key == "zone" {
			key = "availability-zone"
		}
		if !inventory.ValidTopologyKey(key) {
			log.Printf("host topology: description key %q is not valid, line ignored", m[1])
			continue
		}
		found[key] = append(found[key], value)
	}

	keys := make([]string, 0, len(found))
	for key := range found {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := map[string]string{}
	for _, key := range keys {
		values := found[key]
		switch {
		case len(values) > 1:
			log.Printf("host topology: %d lines for key %q in the description of %s, none used", len(values), key, path)
		case !inventory.ValidPlacementName(values[0]):
			log.Printf("host topology: description value %q for key %q is not valid, ignored", values[0], key)
		case len(out) >= maxTopologyKeys:
			log.Printf("host topology: more than %d keys in the description of %s, %q and any later key ignored", maxTopologyKeys, path, key)
		default:
			out[key] = values[0]
		}
	}
	return out
}
