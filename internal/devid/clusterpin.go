package devid

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// Cluster pins: one file per VMID under ClusterPinDir (/etc/pve/vtpm-ek),
// written only by root outside this service (read-cluster-ek-pins-from-pmxcfs
// D-a). This file only reads them.

const (
	errClusterPinMismatch = "EK certificate does not match cluster pin"
	errNoClusterPin       = "no cluster pin for this VM"
)

// parseClusterPin accepts exactly 64 characters [0-9a-f], optionally followed
// by one "\n" (D-d). Unlike readStoredPin it neither trims nor lowercases.
func parseClusterPin(b []byte) (string, bool) {
	if n := len(b); n == 65 && b[64] == '\n' {
		b = b[:64]
	}
	if len(b) != 64 {
		return "", false
	}
	for _, c := range b {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return "", false
		}
	}
	return string(b), true
}

// readClusterPin returns the caller's valid cluster pin, or "" when there is
// none. dirExists reports whether ClusterPinDir exists, which decides the
// "no cluster pin" warning (D-e). A directory or a pin file that exists but
// cannot be read is ErrPinStore (D-f); a malformed file is logged and is no
// pin (D-d).
func (e *Enroller) readClusterPin(vmid string) (pin string, dirExists bool, err error) {
	if e == nil || e.ClusterPinDir == "" {
		return "", false, nil
	}
	if err := validateVMID(vmid); err != nil {
		return "", false, err
	}
	if _, err := os.Stat(e.ClusterPinDir); err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("%w: cluster pin dir: %v", ErrPinStore, err)
	}
	b, err := os.ReadFile(filepath.Join(e.ClusterPinDir, vmid))
	if err != nil {
		if os.IsNotExist(err) {
			return "", true, nil
		}
		return "", true, fmt.Errorf("%w: cluster pin: %v", ErrPinStore, err)
	}
	pin, ok := parseClusterPin(b)
	if !ok {
		log.Printf("devid enroll: invalid cluster pin vm=%s", vmid)
		return "", true, nil
	}
	return pin, true, nil
}

// clusterPinsMatching lists the VM IDs other than vmid whose valid cluster
// pin equals fp, sorted by name. Malformed files are skipped silently here;
// the owner's own leg logs them. A missing directory is empty; one that
// cannot be listed, or a pin file that cannot be read, is ErrPinStore.
func (e *Enroller) clusterPinsMatching(vmid, fp string) ([]string, error) {
	if e == nil || e.ClusterPinDir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(e.ClusterPinDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: list cluster pins: %v", ErrPinStore, err)
	}
	var ids []string
	for _, ent := range entries {
		name := ent.Name()
		if name == vmid || ent.IsDir() || !isVMIDName(name) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(e.ClusterPinDir, name))
		if err != nil {
			return nil, fmt.Errorf("%w: cluster pin: %v", ErrPinStore, err)
		}
		if pin, ok := parseClusterPin(b); ok && pin == fp {
			ids = append(ids, name)
		}
	}
	return ids, nil
}

// guestExists reports whether any node under GuestConfigRoot (/etc/pve/nodes)
// has a qemu-server or lxc config for vmid (D-g). It answers true when
// GuestConfigRoot is empty (YAML backend) or cannot be listed, so a pin is
// never ignored on doubt; the latter is logged.
func (e *Enroller) guestExists(vmid string) bool {
	if e == nil || e.GuestConfigRoot == "" {
		return true
	}
	nodes, err := os.ReadDir(e.GuestConfigRoot)
	if err != nil {
		log.Printf("devid enroll: cannot list %s, no pin is treated as stale: %v", e.GuestConfigRoot, err)
		return true
	}
	for _, n := range nodes {
		if !n.IsDir() {
			continue
		}
		for _, kind := range []string{"qemu-server", "lxc"} {
			_, err := os.Stat(filepath.Join(e.GuestConfigRoot, n.Name(), kind, vmid+".conf"))
			if err == nil || !os.IsNotExist(err) {
				return true
			}
		}
	}
	return false
}
