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
// unchanged and does not count toward a ban; the other clusters go on.
func (r *Reconciler) Pass(ctx context.Context, allowBans bool) error {
	var entries []Entry
	var agents []Agent
	haveEntries, haveAgents := false, false
	for _, t := range r.Targets {
		name := t.Cluster.Name
		guests, err := t.Reader.Read(ctx)
		if err != nil {
			r.Log.Printf("cluster %s: %v; left unchanged", name, err)
			continue
		}
		if !haveEntries {
			if entries, err = r.SPIRE.Entries(ctx); err != nil {
				return fmt.Errorf("cluster %s: list entries: %w", name, err)
			}
			haveEntries = true
		}
		aliases, problems := Wanted(r.TD, t.Cluster, guests)
		for _, p := range problems {
			r.Log.Printf("cluster %s vm %s: %s %s", name, p.VMID, p.Reason, p.Detail)
		}
		create, del := Diff(r.TD, name, entries, aliases)
		if len(del) > 0 {
			if err := r.SPIRE.Delete(ctx, del); err != nil {
				return fmt.Errorf("cluster %s: delete entries: %w", name, err)
			}
		}
		if len(create) > 0 {
			if err := r.SPIRE.Create(ctx, create); err != nil {
				return fmt.Errorf("cluster %s: create entries: %w", name, err)
			}
		}
		if len(create)+len(del) > 0 {
			r.Log.Printf("cluster %s: created %d, deleted %d", name, len(create), len(del))
		}
		if !allowBans {
			continue
		}
		if !haveAgents {
			if agents, err = r.SPIRE.Agents(ctx); err != nil {
				return fmt.Errorf("cluster %s: list agents: %w", name, err)
			}
			haveAgents = true
		}
		listed := make(map[string]bool, len(guests))
		for _, g := range guests {
			listed[g.VMID] = true
		}
		for _, b := range r.Bans.Observe(t.Cluster.Fingerprints, listed, agents) {
			if err := r.SPIRE.Ban(ctx, b.AgentID); err != nil {
				return fmt.Errorf("cluster %s: ban %s: %w", name, b.AgentID, err)
			}
			r.Log.Printf("cluster %s: banned %s (vm %s not listed in two reads)", name, b.AgentID, b.VMID)
		}
	}
	return nil
}
