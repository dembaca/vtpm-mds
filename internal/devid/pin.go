package devid

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Pin store: one file per VMID under PinDir (see design D-a).
// Operator reset: delete the file. No auto-clear on fresh-tpm.

const (
	errStoredPinMismatch    = "EK certificate does not match stored pin"
	errInventoryPinMismatch = "EK certificate does not match inventory pin"
	errPinStoreUnavailable  = "pin store unavailable"
)

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

// ensurePinBeforeIssue creates the pin with an exclusive link before IssueDevID
// when none exists yet, or checks an existing pin matches fp (D-b).
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
