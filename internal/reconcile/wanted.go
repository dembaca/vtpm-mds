// Package reconcile computes the SPIRE node aliases wanted for a Proxmox cluster.
package reconcile

import (
	"sort"
	"strings"
	"unicode"
)

// Guest is one qemu guest of a cluster as the Proxmox API lists it.
type Guest struct {
	VMID string // e.g. "350"
	Pool string // Proxmox pool ID, "" when in no pool; may contain "/" (nested pool)
	Tags string // Proxmox tags string as the API returns it, e.g. "vtpm_mds.io.role.web;other"
}

// Cluster is one configured cluster.
type Cluster struct {
	Name         string   // the configured name used in SPIFFE IDs, e.g. "cluster-1"
	Fingerprints []string // lowercase hex SHA-1 of each configured intermediate's DER
}

// Alias is one wanted node alias.
type Alias struct {
	SPIFFEID  string   // e.g. "spiffe://example.org/pve/cluster-1/vm/350"
	Selectors []string // exactly two, sorted: "tpm_devid:ca:fingerprint:<fp>", "tpm_devid:subject:cn:<vmid>"
}

// Problem is one thing that yielded no alias, to be logged once per pass by the caller.
type Problem struct {
	VMID   string // the guest concerned
	Reason string // one of the Reason* constants
	Detail string // the offending tag, pool or pool pair
}

// The reasons a Problem can carry.
const (
	ReasonBadSegment    = "bad-segment"    // pool/key/value not a valid path segment
	ReasonNestedPool    = "nested-pool"    // pool contains "/"
	ReasonPoolCollision = "pool-collision" // two distinct pools lowercase to the same segment
)

const tagPrefix = "vtpm_mds.io."

// validSegment reports whether s can be one path segment of a SPIFFE ID.
func validSegment(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

// splitTags splits a Proxmox tags string at ';', ',' and white space.
func splitTags(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == ';' || r == ',' || unicode.IsSpace(r)
	})
}

// Wanted returns the aliases wanted for cluster c with trust domain td, sorted by
// SPIFFEID then by Selectors, without duplicates, and the problems found, sorted
// by VMID then Reason then Detail.
func Wanted(td string, c Cluster, guests []Guest) ([]Alias, []Problem) {
	// Pools that collide once lowercased.
	byLower := map[string]map[string]bool{}
	for _, g := range guests {
		if g.Pool == "" || strings.Contains(g.Pool, "/") {
			continue
		}
		l := strings.ToLower(g.Pool)
		if !validSegment(l) {
			continue
		}
		if byLower[l] == nil {
			byLower[l] = map[string]bool{}
		}
		byLower[l][g.Pool] = true
	}
	collide := map[string]string{} // pool as written -> detail
	for _, set := range byLower {
		if len(set) < 2 {
			continue
		}
		ids := make([]string, 0, len(set))
		for id := range set {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		detail := strings.Join(ids, ",")
		for _, id := range ids {
			collide[id] = detail
		}
	}

	type want struct{ id, vmid string }
	seen := map[want]bool{}
	var wants []want
	add := func(id, vmid string) {
		w := want{id, vmid}
		if !seen[w] {
			seen[w] = true
			wants = append(wants, w)
		}
	}
	var problems []Problem
	base := "spiffe://" + td + "/pve/" + c.Name

	for _, g := range guests {
		add(base+"/vm/"+g.VMID, g.VMID)
		if g.Pool == "" {
			continue
		}
		if strings.Contains(g.Pool, "/") {
			problems = append(problems, Problem{g.VMID, ReasonNestedPool, g.Pool})
			continue
		}
		pool := strings.ToLower(g.Pool)
		if !validSegment(pool) {
			problems = append(problems, Problem{g.VMID, ReasonBadSegment, g.Pool})
			continue
		}
		if d, ok := collide[g.Pool]; ok {
			problems = append(problems, Problem{g.VMID, ReasonPoolCollision, d})
			continue
		}
		add(base+"/pool/"+pool, g.VMID)
		for _, piece := range splitTags(g.Tags) {
			if len(piece) < len(tagPrefix) || !strings.EqualFold(piece[:len(tagPrefix)], tagPrefix) {
				continue
			}
			rest := piece[len(tagPrefix):]
			key, value, ok := strings.Cut(rest, ".")
			key, value = strings.ToLower(key), strings.ToLower(value)
			if !ok || !validSegment(key) || !validSegment(value) {
				problems = append(problems, Problem{g.VMID, ReasonBadSegment, piece})
				continue
			}
			add(base+"/pool/"+pool+"/tag/"+key+"/"+value, g.VMID)
		}
	}

	var aliases []Alias
	for _, w := range wants {
		for _, fp := range c.Fingerprints {
			sel := []string{"tpm_devid:ca:fingerprint:" + fp, "tpm_devid:subject:cn:" + w.vmid}
			sort.Strings(sel)
			aliases = append(aliases, Alias{SPIFFEID: w.id, Selectors: sel})
		}
	}
	sort.Slice(aliases, func(i, j int) bool {
		if aliases[i].SPIFFEID != aliases[j].SPIFFEID {
			return aliases[i].SPIFFEID < aliases[j].SPIFFEID
		}
		return strings.Join(aliases[i].Selectors, "\x00") < strings.Join(aliases[j].Selectors, "\x00")
	})
	sort.Slice(problems, func(i, j int) bool {
		a, b := problems[i], problems[j]
		if a.VMID != b.VMID {
			return a.VMID < b.VMID
		}
		if a.Reason != b.Reason {
			return a.Reason < b.Reason
		}
		return a.Detail < b.Detail
	})
	// A tag written twice is reported once.
	uniq := problems[:0]
	for i, pr := range problems {
		if i == 0 || pr != problems[i-1] {
			uniq = append(uniq, pr)
		}
	}
	return aliases, uniq
}
