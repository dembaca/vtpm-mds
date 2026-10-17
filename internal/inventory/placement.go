package inventory

import (
	"regexp"
	"sync"
)

// Placement is where a guest, or the host, sits: the zone and region the
// metadata tree serves as placement/availability-zone and placement/region.
type Placement struct {
	Zone   string
	Region string
}

// DefaultPlacement is the host placement of the YAML backend, and what the
// Proxmox backend falls back to for a name it cannot use.
var DefaultPlacement = Placement{Zone: "proxmox", Region: "local"}

// placementName is a valid zone or region name: 1 to 63 characters from the
// characters of a Proxmox tag, starting with a letter or digit. Capitals are
// allowed and kept: a name is served as its source spells it, so that the zone
// and region a node label controller copies from the same source read alike.
var placementName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.+-]{0,62}$`)

// ValidPlacementName reports whether s may be served as a zone or region.
func ValidPlacementName(s string) bool {
	return placementName.MatchString(s)
}

var (
	hostPlacementMu sync.RWMutex
	hostPlacement   = DefaultPlacement
)

// SetHostPlacement records the host's placement. It is called after every
// successful inventory load, so a failed reload keeps the previous value.
func SetHostPlacement(p Placement) {
	hostPlacementMu.Lock()
	defer hostPlacementMu.Unlock()
	hostPlacement = p
}

// HostPlacement returns the host's placement, DefaultPlacement until an
// inventory has been loaded.
func HostPlacement() Placement {
	hostPlacementMu.RLock()
	defer hostPlacementMu.RUnlock()
	return hostPlacement
}
