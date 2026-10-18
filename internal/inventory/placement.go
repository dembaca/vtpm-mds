package inventory

import (
	"regexp"
	"sync"
)

// Topology is where the host sits, as keys and values: the metadata tree serves
// each as placement/<key>. availability-zone and region are always present, node
// and cluster when the host knows them, and any other key only when the node's
// description declares it.
type Topology map[string]string

// DefaultTopology is the host topology of the YAML backend, and what the Proxmox
// backend starts from: the constants the service served before topology existed.
func DefaultTopology() Topology {
	return Topology{"availability-zone": "proxmox", "region": "local"}
}

// placementName is a valid topology value, and so a valid zone or region name: 1
// to 63 characters from the characters of a Proxmox tag, starting with a letter or
// digit. Capitals are allowed and kept: a value is served as its source spells it,
// so that what a node label controller copies from the same source reads alike.
var placementName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.+-]{0,62}$`)

// ValidPlacementName reports whether s may be served as the value of a topology key.
func ValidPlacementName(s string) bool {
	return placementName.MatchString(s)
}

// ValidTopologyKey reports whether s may be served as a topology key, that is
// whether it is a valid instance tag key: a safe path segment of placement/<key>.
func ValidTopologyKey(s string) bool {
	return ValidTagKey(s)
}

var (
	hostTopologyMu sync.RWMutex
	hostTopology   = DefaultTopology()
)

// SetHostTopology records the host's topology. It is called after every
// successful inventory load, so a failed reload keeps the previous value.
func SetHostTopology(t Topology) {
	hostTopologyMu.Lock()
	defer hostTopologyMu.Unlock()
	hostTopology = copyTopology(t)
}

// HostTopology returns a copy of the host's topology, DefaultTopology until an
// inventory has been loaded.
func HostTopology() Topology {
	hostTopologyMu.RLock()
	defer hostTopologyMu.RUnlock()
	return copyTopology(hostTopology)
}

func copyTopology(t Topology) Topology {
	out := make(Topology, len(t))
	for k, v := range t {
		out[k] = v
	}
	return out
}
