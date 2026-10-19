package bridgebind

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/dembaca/vtpm-mds/internal/inventory"
	"github.com/dembaca/vtpm-mds/internal/proxmox"
)

// Interval is how often the binder checks the guest configurations and the
// table, the same as the inventory check (D-g).
const Interval = 5 * time.Second

// NFT applies and lists the table; a seam for tests.
type NFT interface {
	// Apply loads an nft script in one transaction.
	Apply(script string) error
	// List returns the table without counters, or an error when it does not
	// exist.
	List() (string, error)
}

// Exec runs the nft binary at Path.
type Exec struct{ Path string }

func (e Exec) run(stdin string, args ...string) (string, error) {
	cmd := exec.Command(e.Path, args...)
	cmd.Stdin = bytes.NewBufferString(stdin)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %v: %w: %s", e.Path, args, err, bytes.TrimSpace(errOut.Bytes()))
	}
	return out.String(), nil
}

func (e Exec) Apply(script string) error {
	_, err := e.run(script, "-f", "-")
	return err
}

func (e Exec) List() (string, error) {
	// -s (stateless) leaves out the counters, so the listing changes only when
	// the rules do.
	return e.run("", "-s", "list", "table", "bridge", Table)
}

// Binder keeps the table in step with this node's guest configurations.
type Binder struct {
	PVERoot string // "" means proxmox.DefaultRoot
	Bridge  string
	NFT     NFT
	// RunDir, when set, receives the last rendered script as vtpm_mds_bind.nft.
	RunDir string
	Logf   func(format string, args ...any)

	fp     inventory.Fingerprint
	haveFP bool
	want   string // the script for the current configurations; "" until read
	ports  int
	seen   tableState // the table as the last apply, or failed apply, left it
}

// tableState is what a listing found: the rules, or that the table is missing.
type tableState struct {
	missing bool
	listing string
}

func stateOf(listing string, err error) tableState {
	if err != nil {
		return tableState{missing: true}
	}
	return tableState{listing: listing}
}

func (b *Binder) logf(format string, args ...any) {
	if b.Logf != nil {
		b.Logf(format, args...)
		return
	}
	log.Printf(format, args...)
}

// Tick runs one check and reports whether it applied the table. The
// configurations are read again only when their fingerprint changed; the table
// is listed every time and restored when it is missing or differs from what
// the last apply left. Nothing is logged when nothing changes; a failure is
// logged once and retried on the next change of the configurations or of the
// table.
func (b *Binder) Tick() bool {
	changed := false
	if fp := proxmox.SourceFingerprint(b.PVERoot); !b.haveFP || fp != b.fp {
		b.fp, b.haveFP = fp, true
		nics, err := proxmox.GuestNICs(b.PVERoot)
		if err != nil {
			b.logf("Failed to read guest NICs, keeping the %s table as it is: %v", Table, err)
		} else if script := Render(b.Bridge, nics); script != b.want {
			b.want, b.ports, changed = script, Bound(b.Bridge, nics), true
		}
	}
	if b.want == "" {
		return false
	}

	now := stateOf(b.NFT.List())
	if !changed {
		if now == b.seen {
			return false
		}
		if now.missing {
			b.logf("Table bridge %s is missing, restoring it", Table)
		} else {
			b.logf("Table bridge %s was changed outside the binder, restoring it", Table)
		}
	}

	if err := b.NFT.Apply(b.want); err != nil {
		b.logf("Failed to apply table bridge %s: %v", Table, err)
		// Remember the table as it is now, so it is not retried, and the
		// failure not logged again, until it or the configurations change.
		b.seen = now
		return false
	}
	listing, err := b.NFT.List()
	if err != nil {
		b.logf("Applied table bridge %s but cannot list it: %v", Table, err)
	}
	b.seen = stateOf(listing, err)
	b.writeScript()
	b.logf("Applied table bridge %s: %d guest ports on %s", Table, b.ports, b.Bridge)
	return true
}

func (b *Binder) writeScript() {
	if b.RunDir == "" {
		return
	}
	path := filepath.Join(b.RunDir, Table+".nft")
	if err := os.WriteFile(path, []byte(b.want), 0o644); err != nil {
		b.logf("Failed to write %s: %v", path, err)
	}
}

// Run ticks at once and then every interval until ctx is done.
func (b *Binder) Run(ctx context.Context, interval time.Duration) {
	b.Tick()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			b.Tick()
		}
	}
}
