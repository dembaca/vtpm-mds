package reconcile

import (
	"context"
	"fmt"
	"log"
)

// spireAPI is what a pass needs of the SPIRE server; *SPIRE implements it.
type spireAPI interface {
	Entries(context.Context) ([]Entry, error)
	Create(context.Context, []Alias) error
	Delete(context.Context, []string) error
	Agents(context.Context) ([]Agent, error)
	Ban(context.Context, string) error
}

// Target is one cluster a Reconciler works on.
type Target struct {
	Cluster Cluster
	Reader  interface {
		Read(context.Context) ([]Guest, error)
	}
}

// Reconciler brings SPIRE node aliases in line with Proxmox clusters.
type Reconciler struct {
	TD      string
	Targets []Target
	SPIRE   spireAPI
	Bans    *BanTracker
	Log     *log.Logger
}

// Pass reads every cluster and brings its aliases in line; with allowBans it
// also bans agents of destroyed VMs. A doubtful read leaves that cluster
// unchanged and does not count toward a ban; the other clusters go on. skipped
// is the number of clusters left unchanged in this pass for that reason.
func (r *Reconciler) Pass(ctx context.Context, allowBans bool) (skipped int, err error) {
	logger := r.Log
	if logger == nil {
		logger = log.Default()
	}
	var entries []Entry
	var agents []Agent
	haveEntries, haveAgents := false, false
	for _, t := range r.Targets {
		name := t.Cluster.Name
		guests, err := t.Reader.Read(ctx)
		if err != nil {
			logger.Printf("cluster %s: %v; left unchanged", name, err)
			skipped++
			continue
		}
		if !haveEntries {
			if entries, err = r.SPIRE.Entries(ctx); err != nil {
				return skipped, fmt.Errorf("cluster %s: list entries: %w", name, err)
			}
			haveEntries = true
		}
		aliases, problems := Wanted(r.TD, t.Cluster, guests)
		for _, p := range problems {
			logger.Printf("cluster %s vm %q: %s %q", name, p.VMID, p.Reason, p.Detail)
		}
		create, del := Diff(r.TD, name, entries, aliases)

		// An alias that replaces an entry about to be deleted (same SPIFFE ID
		// and selectors, e.g. a hand-edited one) is created after the delete,
		// since SPIRE refuses a duplicate; every other one before it, so a
		// failed create leaves the old aliases in place.
		type key struct{ id, sel string }
		doomed := map[string]bool{}
		for _, id := range del {
			doomed[id] = true
		}
		replaced := map[key]bool{}
		for _, e := range entries {
			if doomed[e.ID] {
				replaced[key{e.SPIFFEID, selectorKey(e.Selectors)}] = true
			}
		}
		var safe, colliding []Alias
		for _, a := range create {
			if replaced[key{a.SPIFFEID, selectorKey(a.Selectors)}] {
				colliding = append(colliding, a)
			} else {
				safe = append(safe, a)
			}
		}
		if len(safe) > 0 {
			if err := r.SPIRE.Create(ctx, safe); err != nil {
				return skipped, fmt.Errorf("cluster %s: create entries: %w", name, err)
			}
		}
		if len(del) > 0 {
			if err := r.SPIRE.Delete(ctx, del); err != nil {
				return skipped, fmt.Errorf("cluster %s: delete entries: %w", name, err)
			}
		}
		if len(colliding) > 0 {
			if err := r.SPIRE.Create(ctx, colliding); err != nil {
				return skipped, fmt.Errorf("cluster %s: create replaced entries: %w", name, err)
			}
		}
		if n := len(safe) + len(colliding); n+len(del) > 0 {
			logger.Printf("cluster %s: created %d, deleted %d", name, n, len(del))
		}
		if !allowBans {
			continue
		}
		if !haveAgents {
			if agents, err = r.SPIRE.Agents(ctx); err != nil {
				return skipped, fmt.Errorf("cluster %s: list agents: %w", name, err)
			}
			haveAgents = true
		}
		listed := make(map[string]bool, len(guests))
		for _, g := range guests {
			listed[g.VMID] = true
		}
		for _, b := range r.Bans.Observe(t.Cluster.Fingerprints, listed, agents) {
			if err := r.SPIRE.Ban(ctx, b.AgentID); err != nil {
				return skipped, fmt.Errorf("cluster %s: ban %s: %w", name, b.AgentID, err)
			}
			logger.Printf("cluster %s: banned %s (vm %s not listed in two reads)", name, b.AgentID, b.VMID)
		}
	}
	return skipped, nil
}
