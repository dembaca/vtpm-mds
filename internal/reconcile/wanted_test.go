package reconcile

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

const td = "example.org"

var one = Cluster{Name: "cluster-1", Fingerprints: []string{"aa"}}

func ids(as []Alias) []string {
	var out []string
	for _, a := range as {
		out = append(out, strings.TrimPrefix(a.SPIFFEID, "spiffe://example.org/pve/cluster-1/"))
	}
	return out
}

func sels(vmid, fp string) []string {
	return []string{"tpm_devid:ca:fingerprint:" + fp, "tpm_devid:subject:cn:" + vmid}
}

func TestWanted(t *testing.T) {
	tests := []struct {
		name     string
		cluster  Cluster
		guests   []Guest
		want     []string // aliases without the common prefix, sorted
		problems []Problem
	}{
		{"a VM with a pool and tags", one,
			[]Guest{{"350", "web", "vtpm_mds.io.role.Frontend;vtpm_mds.io.role.cache;other"}},
			[]string{"pool/web", "pool/web/tag/role/cache", "pool/web/tag/role/frontend", "vm/350"}, nil},
		{"a VM without a pool", one,
			[]Guest{{"351", "", "vtpm_mds.io.role.admin"}},
			[]string{"vm/351"}, nil},
		{"a tag that cannot be a path segment", one,
			[]Guest{{"352", "web", "vtpm_mds.io.role.a+b"}},
			[]string{"pool/web", "vm/352"},
			[]Problem{{"352", ReasonBadSegment, "vtpm_mds.io.role.a+b"}}},
		{"two pools that differ only in case", one,
			[]Guest{{"360", "Web", "vtpm_mds.io.role.x"}, {"361", "web", "vtpm_mds.io.role.x"}},
			[]string{"vm/360", "vm/361"},
			[]Problem{{"360", ReasonPoolCollision, "Web,web"}, {"361", ReasonPoolCollision, "Web,web"}}},
		{"a nested pool", one,
			[]Guest{{"354", "web/front", "vtpm_mds.io.role.db"}},
			[]string{"vm/354"},
			[]Problem{{"354", ReasonNestedPool, "web/front"}}},
		{"dot segments pool", one,
			[]Guest{{"355", "..", ""}},
			[]string{"vm/355"},
			[]Problem{{"355", ReasonBadSegment, ".."}}},
		{"dot segments tag", one,
			[]Guest{{"356", "web", "vtpm_mds.io.role.."}},
			[]string{"pool/web", "vm/356"},
			[]Problem{{"356", ReasonBadSegment, "vtpm_mds.io.role.."}}},
		{"prefix case-insensitive and value with dots", one,
			[]Guest{{"357", "web", "VTPM_MDS.IO.Env.Prod.EU"}},
			[]string{"pool/web", "pool/web/tag/env/prod.eu", "vm/357"}, nil},
		{"separators", one,
			[]Guest{{"358", "web", "vtpm_mds.io.a.1, vtpm_mds.io.b.2 vtpm_mds.io.c.3"}},
			[]string{"pool/web", "pool/web/tag/a/1", "pool/web/tag/b/2", "pool/web/tag/c/3", "vm/358"}, nil},
		{"two intermediates", Cluster{Name: "cluster-1", Fingerprints: []string{"aa", "bb"}},
			[]Guest{{"359", "", ""}},
			[]string{"vm/359", "vm/359"}, nil},
		{"no fingerprints", Cluster{Name: "cluster-1"},
			[]Guest{{"359", "web", ""}},
			nil, nil},
		{"duplicates removed", one,
			[]Guest{{"362", "web", "vtpm_mds.io.role.web;vtpm_mds.io.role.web"}},
			[]string{"pool/web", "pool/web/tag/role/web", "vm/362"}, nil},
		{"a tag with no value", one,
			[]Guest{{"363", "web", "vtpm_mds.io.role"}},
			[]string{"pool/web", "vm/363"},
			[]Problem{{"363", ReasonBadSegment, "vtpm_mds.io.role"}}},
		{"a bad tag written twice is reported once", one,
			[]Guest{{"364", "web", "vtpm_mds.io.role.a+b;vtpm_mds.io.role.a+b"}},
			[]string{"pool/web", "vm/364"},
			[]Problem{{"364", ReasonBadSegment, "vtpm_mds.io.role.a+b"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, problems := Wanted(td, tt.cluster, tt.guests)
			if !reflect.DeepEqual(ids(got), tt.want) {
				t.Errorf("aliases = %v, want %v", ids(got), tt.want)
			}
			if !reflect.DeepEqual(problems, tt.problems) {
				t.Errorf("problems = %v, want %v", problems, tt.problems)
			}
		})
	}
}

func TestWantedSelectors(t *testing.T) {
	t.Run("two selectors per fingerprint", func(t *testing.T) {
		c := Cluster{Name: "cluster-1", Fingerprints: []string{"aa", "bb"}}
		got, _ := Wanted(td, c, []Guest{{"350", "web", ""}, {"351", "web", ""}})
		var pool [][]string
		for _, a := range got {
			if a.SPIFFEID == "spiffe://example.org/pve/cluster-1/pool/web" {
				pool = append(pool, a.Selectors)
			}
		}
		want := [][]string{sels("350", "aa"), sels("351", "aa"), sels("350", "bb"), sels("351", "bb")}
		if !reflect.DeepEqual(pool, want) {
			t.Errorf("pool selectors = %v, want %v", pool, want)
		}
	})
}

func TestWantedSorted(t *testing.T) {
	t.Run("output sorted", func(t *testing.T) {
		c := Cluster{Name: "cluster-1", Fingerprints: []string{"bb", "aa"}}
		guests := []Guest{
			{"9", "web", "vtpm_mds.io.z.1;vtpm_mds.io.a.1;vtpm_mds.io.a.."},
			{"10", "db", "vtpm_mds.io.k"},
			{"2", "web/x", ""},
		}
		got, problems := Wanted(td, c, guests)
		if !sort.SliceIsSorted(got, func(i, j int) bool {
			if got[i].SPIFFEID != got[j].SPIFFEID {
				return got[i].SPIFFEID < got[j].SPIFFEID
			}
			return strings.Join(got[i].Selectors, "\x00") < strings.Join(got[j].Selectors, "\x00")
		}) {
			t.Errorf("aliases not sorted: %v", got)
		}
		if !sort.SliceIsSorted(problems, func(i, j int) bool {
			a, b := problems[i], problems[j]
			if a.VMID != b.VMID {
				return a.VMID < b.VMID
			}
			if a.Reason != b.Reason {
				return a.Reason < b.Reason
			}
			return a.Detail < b.Detail
		}) {
			t.Errorf("problems not sorted: %v", problems)
		}
		again, _ := Wanted(td, c, guests)
		if !reflect.DeepEqual(got, again) {
			t.Error("output differs between calls")
		}
	})
}
