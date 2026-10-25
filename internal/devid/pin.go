package devid

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
)

// Pin store: one file per VMID under PinDir (see design D-a).
// Operator reset: delete the file. No auto-clear on fresh-tpm.

const (
	errStoredPinMismatch    = "EK certificate does not match stored pin"
	errInventoryPinMismatch = "EK certificate does not match inventory pin"
	errPinStoreUnavailable  = "pin store unavailable"
	errPinnedElsewhere      = "EK certificate is pinned to another VM"
)

// pinCreateMu serialises the cross-VM scan with the exclusive create in
// ensurePinBeforeIssue, so two VMs presenting one EK never both get a pin
// (design D-a2). One process serves every leg on this node.
var pinCreateMu sync.Mutex

// beforePinCreate runs between the cross-VM scan and the create; tests use it
// to hold the window open. Nil in production.
var beforePinCreate func()

// ErrPinStore indicates the pin directory or file could not be used; HTTP maps it to 500.
var ErrPinStore = errors.New("ek pin store")

var errAlreadyPinned = errors.New("ek pin already exists")

func validateVMID(vmid string) error {
	if vmid == "" {
		return fmt.Errorf("%w: empty vmid", ErrPinStore)
	}
	if strings.ContainsAny(vmid, `/\`) || strings.Contains(vmid, "..") || strings.HasPrefix(vmid, ".") {
		return fmt.Errorf("%w: invalid vmid", ErrPinStore)
	}
	if filepath.Base(vmid) != vmid {
		return fmt.Errorf("%w: invalid vmid", ErrPinStore)
	}
	return nil
}

func pinPath(dir, vmid string) string {
	return filepath.Join(dir, vmid)
}

// readStoredPin returns the fingerprint from dir/vmid, or "" if missing/empty.
func readStoredPin(dir, vmid string) (string, error) {
	if dir == "" {
		return "", nil
	}
	if err := validateVMID(vmid); err != nil {
		return "", err
	}
	b, err := os.ReadFile(pinPath(dir, vmid))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("%w: read: %v", ErrPinStore, err)
	}
	return strings.ToLower(strings.TrimSpace(string(b))), nil
}

// createStoredPinExclusive writes fp via a temp file then os.Link (atomic + exclusive).
// Returns errAlreadyPinned if the pin path already exists.
func createStoredPinExclusive(dir, vmid, fp string) error {
	if dir == "" || fp == "" {
		return fmt.Errorf("%w: missing dir or fingerprint", ErrPinStore)
	}
	if err := validateVMID(vmid); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("%w: mkdir: %v", ErrPinStore, err)
	}
	path := pinPath(dir, vmid)
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return fmt.Errorf("%w: rand: %v", ErrPinStore, err)
	}
	tmp := filepath.Join(dir, fmt.Sprintf(".%s.tmp-%s", vmid, hex.EncodeToString(nonce[:])))
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return fmt.Errorf("%w: create tmp: %v", ErrPinStore, err)
	}
	_, werr := fmt.Fprintf(f, "%s\n", strings.ToLower(fp))
	if serr := f.Sync(); serr != nil && werr == nil {
		werr = serr
	}
	cerr := f.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(tmp)
		if werr != nil {
			return fmt.Errorf("%w: write: %v", ErrPinStore, werr)
		}
		return fmt.Errorf("%w: close: %v", ErrPinStore, cerr)
	}
	if err := os.Link(tmp, path); err != nil {
		_ = os.Remove(tmp)
		if errors.Is(err, os.ErrExist) || errors.Is(err, syscall.EEXIST) {
			return errAlreadyPinned
		}
		return fmt.Errorf("%w: link: %v", ErrPinStore, err)
	}
	_ = os.Remove(tmp)
	syncDir(dir)
	return nil
}

func syncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}

// pinnedToAnotherVM returns the VM ID other than vmid that fp is pinned to:
// first a pin file in PinDir (sorted by name), then another VM's inventory
// ek_sha256 (sorted by VM ID), or "" when none (refuse-an-ek-pinned-to-another-vm
// D-b). Files whose name is not a valid VM ID (including the .<vmid>.tmp-*
// of an in-flight create) and empty files are ignored. A missing PinDir is an
// empty store; one that cannot be listed is ErrPinStore (D-d).
func (e *Enroller) pinnedToAnotherVM(vmid, fp string) (string, error) {
	if e == nil || fp == "" {
		return "", nil
	}
	if e.PinDir != "" {
		entries, err := os.ReadDir(e.PinDir)
		if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("%w: list: %v", ErrPinStore, err)
		}
		for _, ent := range entries {
			name := ent.Name()
			if name == vmid || ent.IsDir() || validateVMID(name) != nil {
				continue
			}
			stored, err := readStoredPin(e.PinDir, name)
			if err != nil {
				return "", err
			}
			if stored != "" && strings.EqualFold(stored, fp) {
				return name, nil
			}
		}
	}
	if e.InventoryPins == nil {
		return "", nil
	}
	pins := e.InventoryPins()
	ids := make([]string, 0, len(pins))
	for id := range pins {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if id == vmid {
			continue
		}
		if pin := strings.TrimSpace(pins[id]); pin != "" && strings.EqualFold(pin, fp) {
			return id, nil
		}
	}
	return "", nil
}

// ensurePinBeforeIssue creates the pin with an exclusive link before IssueDevID
// when none exists yet, or checks an existing pin matches fp (D-b). Under
// pinCreateMu it first repeats the cross-VM scan, so the refusal and the
// create are one step (refuse-an-ek-pinned-to-another-vm D-a2).
func (e *Enroller) ensurePinBeforeIssue(vmid, fp string) error {
	if e == nil || e.PinDir == "" {
		return nil
	}
	pinCreateMu.Lock()
	defer pinCreateMu.Unlock()
	existing, err := readStoredPin(e.PinDir, vmid)
	if err != nil {
		return err
	}
	if existing != "" && !strings.EqualFold(existing, fp) {
		return errUnauthorized(errStoredPinMismatch)
	}
	other, err := e.pinnedToAnotherVM(vmid, fp)
	if err != nil {
		return err
	}
	if other != "" {
		return errPinnedToAnotherVM(vmid, other)
	}
	if existing != "" {
		return nil
	}
	if beforePinCreate != nil {
		beforePinCreate()
	}
	err = createStoredPinExclusive(e.PinDir, vmid, fp)
	if err == nil {
		return nil
	}
	if errors.Is(err, errAlreadyPinned) {
		existing, rerr := readStoredPin(e.PinDir, vmid)
		if rerr != nil {
			return rerr
		}
		// A whitespace-only or empty file is "no pin" (D-c step 2): replace it.
		if existing == "" {
			_ = os.Remove(pinPath(e.PinDir, vmid))
			return createStoredPinExclusive(e.PinDir, vmid, fp)
		}
		if !strings.EqualFold(existing, fp) {
			return errUnauthorized(errStoredPinMismatch)
		}
		return nil
	}
	return err
}
