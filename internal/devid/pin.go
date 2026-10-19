package devid

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Pin store: one file per VMID under PinDir (see design D-a).
// Operator reset: delete the file. No auto-clear on fresh-tpm.

const (
	errStoredPinMismatch    = "EK certificate does not match stored pin"
	errInventoryPinMismatch = "EK certificate does not match inventory pin"
)

// ErrPinStore indicates the pin directory or file could not be used; HTTP maps it to 500.
var ErrPinStore = errors.New("ek pin store")

func pinPath(dir, vmid string) string {
	return filepath.Join(dir, vmid)
}

// readStoredPin returns the fingerprint from dir/vmid, or "" if missing/empty.
func readStoredPin(dir, vmid string) (string, error) {
	if dir == "" || vmid == "" {
		return "", nil
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

// createStoredPinExclusive writes fp with O_EXCL. Returns errAlreadyPinned if the file exists.
func createStoredPinExclusive(dir, vmid, fp string) error {
	if dir == "" || vmid == "" || fp == "" {
		return fmt.Errorf("%w: missing dir, vmid or fingerprint", ErrPinStore)
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("%w: mkdir: %v", ErrPinStore, err)
	}
	path := pinPath(dir, vmid)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		if os.IsExist(err) {
			return errAlreadyPinned
		}
		return fmt.Errorf("%w: create: %v", ErrPinStore, err)
	}
	_, werr := fmt.Fprintf(f, "%s\n", strings.ToLower(fp))
	cerr := f.Close()
	if werr != nil {
		_ = os.Remove(path)
		return fmt.Errorf("%w: write: %v", ErrPinStore, werr)
	}
	if cerr != nil {
		return fmt.Errorf("%w: close: %v", ErrPinStore, cerr)
	}
	return nil
}

var errAlreadyPinned = errors.New("ek pin already exists")

// ensurePinBeforeIssue creates the pin with O_EXCL before IssueDevID when none
// exists yet, or checks an existing pin matches fp (D-b).
func (e *Enroller) ensurePinBeforeIssue(vmid, fp string) error {
	if e == nil || e.PinDir == "" {
		return nil
	}
	existing, err := readStoredPin(e.PinDir, vmid)
	if err != nil {
		return err
	}
	if existing != "" {
		if !strings.EqualFold(existing, fp) {
			return errUnauthorized(errStoredPinMismatch)
		}
		return nil
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
		if existing == "" || !strings.EqualFold(existing, fp) {
			return errUnauthorized(errStoredPinMismatch)
		}
		return nil
	}
	return err
}
