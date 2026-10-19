package proxmox

import (
	"os"
	"path/filepath"

	"github.com/dembaca/vtpm-mds/internal/inventory"
)

// SourceFingerprint is the fingerprint of exactly what ParseVMConfigs and
// HostTopology read under pveRoot ("" means DefaultRoot): the target of the
// local link, each local/qemu-server/*.conf by name and content, the node
// description in local/config and corosync.conf. It never fails; a missing or
// unreadable input is part of the digest, and the reload it triggers reports
// the error.
func SourceFingerprint(pveRoot string) inventory.Fingerprint {
	if pveRoot == "" {
		pveRoot = DefaultRoot
	}
	b := inventory.NewFingerprint()

	local := filepath.Join(pveRoot, "local")
	if target, err := os.Readlink(local); err == nil {
		b.AddString("local->" + target)
	} else {
		b.AddString("local!" + err.Error())
	}

	configDir := filepath.Join(local, "qemu-server")
	if _, err := os.ReadDir(configDir); err != nil {
		b.AddString("qemu-server!" + err.Error())
	} else if files, err := filepath.Glob(filepath.Join(configDir, "*.conf")); err != nil {
		b.AddString("glob!" + err.Error())
	} else {
		// Glob returns the names sorted, so the order is stable.
		for _, f := range files {
			b.AddFile(f)
		}
	}

	b.AddFile(filepath.Join(local, "config"))
	b.AddFile(filepath.Join(pveRoot, "corosync.conf"))
	return b.Sum()
}
