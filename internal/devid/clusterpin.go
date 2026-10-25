package devid

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"time"
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
// cannot be read is ErrPinStore (D-f); a malformed file, or a directory under
// the VM ID's name, is no pin (D-d) and is logged unless quiet, so a leg that
// reads the pin twice logs it once.
func (e *Enroller) readClusterPin(vmid string, quiet bool) (pin string, dirExists bool, err error) {
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
		if !errors.Is(err, syscall.EISDIR) {
			return "", true, fmt.Errorf("%w: cluster pin: %v", ErrPinStore, err)
		}
		b = nil
	}
	pin, ok := parseClusterPin(b)
	if !ok {
		if !quiet {
			log.Printf("devid enroll: invalid cluster pin vm=%s", vmid)
		}
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
			// Removed since the listing (root's remedy, the hook's rename).
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("%w: cluster pin: %v", ErrPinStore, err)
		}
		if pin, ok := parseClusterPin(b); ok && pin == fp {
			ids = append(ids, name)
		}
	}
	return ids, nil
}

// guestConfigRecheck is how long guestExists waits before it looks a second
// time, so a config caught mid-move (migration, HA recovery) is not taken
// for a destroyed VM (D-g). A variable for tests.
var guestConfigRecheck = 250 * time.Millisecond

// guestExists reports whether any node under GuestConfigRoot (/etc/pve/nodes)
// has a qemu-server or lxc config for vmid (D-g). It answers true when
// GuestConfigRoot is empty (YAML backend) or cannot be listed, so a pin is
// never ignored on doubt; the latter is logged. A config that is not found
// is looked for once more after guestConfigRecheck.
func (e *Enroller) guestExists(vmid string) bool {
	if e == nil || e.GuestConfigRoot == "" {
		return true
	}
	if e.guestConfigFound(vmid) {
		return true
	}
	time.Sleep(guestConfigRecheck)
	return e.guestConfigFound(vmid)
}

// logStalePin records that a pin of a destroyed VM did not refuse caller,
// so reviving a destroyed VM's vTPM (a backup) shows in the journal.
func logStalePin(caller, stale string) {
	log.Printf("devid enroll: stale pin ignored vm=%s ignored_stale=%s", caller, stale)
}

func (e *Enroller) guestConfigFound(vmid string) bool {
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
