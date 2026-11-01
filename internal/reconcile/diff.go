package reconcile

import (
	"sort"
	"strings"
)

// Entry is one registration entry as the SPIRE server lists it.
type Entry struct {
	ID        string
	ParentID  string // e.g. "spiffe://example.org/spire/server"
	SPIFFEID  string
	Selectors []string // "type:value", e.g. "tpm_devid:subject:cn:350"; any order
	Default   bool     // true when every other field is at its default
}

// selectorKey joins a sorted copy of selectors into one comparable string.
func selectorKey(sel []string) string {
	s := append([]string(nil), sel...)
	sort.Strings(s)
	return strings.Join(s, "\x00")
}

// Diff compares the entries of trust domain td with the aliases wanted for the
// cluster named cluster and returns the wanted aliases to create and the IDs of
// entries to delete. Only entries parented on the SPIRE server under the
// cluster's /pve/ prefix are ever deleted.
func Diff(td, cluster string, existing []Entry, wanted []Alias) (create []Alias, del []string) {
	parent := "spiffe://" + td + "/spire/server"
	prefix := "spiffe://" + td + "/pve/" + cluster + "/"

	type key struct{ id, sel string }
	want := map[key]bool{}
	for _, a := range wanted {
		want[key{a.SPIFFEID, selectorKey(a.Selectors)}] = true
	}
	matched := map[key]bool{}
	for _, e := range existing {
		if e.ParentID != parent || !strings.HasPrefix(e.SPIFFEID, prefix) {
			continue
		}
		k := key{e.SPIFFEID, selectorKey(e.Selectors)}
		if e.Default && want[k] && !matched[k] {
			matched[k] = true
			continue
		}
		del = append(del, e.ID)
	}
	done := map[key]bool{}
	for _, a := range wanted {
		k := key{a.SPIFFEID, selectorKey(a.Selectors)}
		if matched[k] || done[k] {
			continue
		}
		done[k] = true
		create = append(create, a)
	}
	return create, del
}

// Agent is one attested agent as the SPIRE server lists it.
type Agent struct {
	ID        string   // e.g. "spiffe://example.org/spire/agent/tpm_devid/ab12..."
	Selectors []string // "type:value"
	Banned    bool
}

// Ban is one agent to ban.
type Ban struct{ AgentID, VMID string }

// BanTracker counts, per agent, consecutive good reads of a cluster in which the
// agent's VM was not listed. Its state lives in memory only. The zero value is
// ready to use.
type BanTracker struct {
	misses map[string]*miss
}

type miss struct {
	count int
	fps   []string // the CA fingerprints the agent carried
}

const (
	fpPrefix = "tpm_devid:ca:fingerprint:"
	cnPrefix = "tpm_devid:subject:cn:"
)

// agentInfo returns the fingerprints and VM ID of an agent, and whether it
// carries exactly one cn selector and is not banned.
func agentInfo(a Agent) (fps []string, vmid string, ok bool) {
	if a.Banned {
		return nil, "", false
	}
	cns := 0
	for _, s := range a.Selectors {
		switch {
		case strings.HasPrefix(s, fpPrefix):
			fps = append(fps, s[len(fpPrefix):])
		case strings.HasPrefix(s, cnPrefix):
			cns++
			vmid = s[len(cnPrefix):]
		}
	}
	return fps, vmid, cns == 1
}

// Observe is called once per good read of one cluster, with that cluster's
// intermediate fingerprints, the VM IDs listed in that read (templates included)
// and all attested agents. It returns the agents to ban now, sorted by agent ID,
// each with the VM ID it names. An agent is returned after two consecutive reads
// that miss its VM; its count is then dropped, so if it is still not banned it
// is returned again only after two more misses.
func (b *BanTracker) Observe(fingerprints []string, listed map[string]bool, agents []Agent) []Ban {
	if b.misses == nil {
		b.misses = map[string]*miss{}
	}
	mine := map[string]bool{}
	for _, f := range fingerprints {
		mine[f] = true
	}
	belongs := func(fps []string) bool {
		for _, f := range fps {
			if mine[f] {
				return true
			}
		}
		return false
	}

	seen := map[string]bool{}
	var bans []Ban
	for _, a := range agents {
		fps, vmid, ok := agentInfo(a)
		if !ok || !belongs(fps) {
			continue
		}
		seen[a.ID] = true
		if listed[vmid] {
			delete(b.misses, a.ID)
			continue
		}
		m := b.misses[a.ID]
		if m == nil {
			m = &miss{}
			b.misses[a.ID] = m
		}
		m.fps = fps
		m.count++
		if m.count >= 2 {
			bans = append(bans, Ban{AgentID: a.ID, VMID: vmid})
			delete(b.misses, a.ID)
		}
	}
	for id, m := range b.misses {
		if !seen[id] && belongs(m.fps) {
			delete(b.misses, id)
		}
	}
	sort.Slice(bans, func(i, j int) bool { return bans[i].AgentID < bans[j].AgentID })
	return bans
}
