// Package bridgebind keeps the nftables table that admits, on each guest port
// of the IMDS bridge, only the MAC in that guest's configuration
// (bind-callers-to-their-bridge-port).
package bridgebind

import (
	"fmt"
	"strings"

	"github.com/dembaca/vtpm-mds/internal/proxmox"
)

// Table is the nftables table this package owns, in the bridge family.
const Table = "vtpm_mds_bind"

// Render returns the whole table for the NICs on bridge, in one nft script
// that declares, deletes and redefines the table, so loading it replaces the
// table in a single transaction however often it is loaded (D-c). nics must be
// sorted (GuestNICs sorts them); the output is then byte-identical for
// identical input. A NIC on the bridge without a valid MAC gets a rule that
// drops everything from its port: an unbound port must not be an open one.
func Render(bridge string, nics []proxmox.NIC) string {
	var b strings.Builder
	fmt.Fprintf(&b, "table bridge %s\n", Table)
	fmt.Fprintf(&b, "delete table bridge %s\n", Table)
	fmt.Fprintf(&b, "table bridge %s {\n", Table)
	b.WriteString("\tchain from_guests {\n")
	b.WriteString("\t\ttype filter hook prerouting priority -200; policy accept;\n")
	for _, n := range nics {
		if n.Bridge != bridge {
			continue
		}
		tap := n.Tap()
		if n.MAC == "" {
			fmt.Fprintf(&b, "\t\tiifname %q counter drop\n", tap)
			continue
		}
		fmt.Fprintf(&b, "\t\tiifname %q ether saddr != %s counter drop\n", tap, n.MAC)
		fmt.Fprintf(&b, "\t\tiifname %q ether type arp arp saddr ether != %s counter drop\n", tap, n.MAC)
	}
	b.WriteString("\t}\n}\n")
	return b.String()
}

// Bound counts the NICs Render writes rules for.
func Bound(bridge string, nics []proxmox.NIC) int {
	n := 0
	for _, nic := range nics {
		if nic.Bridge == bridge {
			n++
		}
	}
	return n
}
