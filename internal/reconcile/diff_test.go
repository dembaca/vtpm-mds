package reconcile

import (
	"reflect"
	"testing"
)

func sel(fp, vmid string) []string {
	return []string{"tpm_devid:ca:fingerprint:" + fp, "tpm_devid:subject:cn:" + vmid}
}

func TestDiff(t *testing.T) {
	const srv = "spiffe://example.org/spire/server"
	const p = "spiffe://example.org/pve/cluster-1/"
	vm350 := Alias{p + "vm/350", sel("aa", "350")}
	pool := Alias{p + "pool/web", sel("aa", "350")}
	tag := Alias{p + "pool/web/tag/role/db", sel("aa", "350")}
	tests := []struct {
		name     string
		existing []Entry
		wanted   []Alias
		create   []Alias
		del      []string
	}{
		{"a removed tag",
			[]Entry{{"1", srv, vm350.SPIFFEID, vm350.Selectors, true}, {"2", srv, tag.SPIFFEID, tag.Selectors, true}, {"3", srv, pool.SPIFFEID, pool.Selectors, true}},
			[]Alias{vm350, pool}, nil, []string{"2"}},
		{"an entry outside the prefix",
			[]Entry{{"1", srv, "spiffe://example.org/nodes/build", sel("aa", "350"), true}},
			[]Alias{vm350}, []Alias{vm350}, nil},
		{"a workload entry under the prefix",
			[]Entry{{"1", p + "pool/web", p + "pool/web/app", []string{"unix:uid:1"}, true}},
			nil, nil, nil},
		{"a hand-edited alias",
			[]Entry{{"1", srv, vm350.SPIFFEID, vm350.Selectors, false}},
			[]Alias{vm350}, []Alias{vm350}, []string{"1"}},
		{"a duplicate owned entry",
			[]Entry{{"1", srv, vm350.SPIFFEID, vm350.Selectors, true}, {"2", srv, vm350.SPIFFEID, vm350.Selectors, true}},
			[]Alias{vm350}, nil, []string{"2"}},
		{"selector order irrelevant",
			[]Entry{{"1", srv, vm350.SPIFFEID, []string{vm350.Selectors[1], vm350.Selectors[0]}, true}},
			[]Alias{vm350}, nil, nil},
		{"another cluster's prefix",
			[]Entry{{"1", srv, "spiffe://example.org/pve/cluster-10/vm/350", sel("aa", "350"), true}},
			nil, nil, nil},
		{"empty existing creates all",
			nil, []Alias{vm350, pool}, []Alias{vm350, pool}, nil},
		{"duplicate wanted created once",
			nil, []Alias{vm350, vm350}, []Alias{vm350}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			create, del := Diff("example.org", "cluster-1", tt.existing, tt.wanted)
			if !reflect.DeepEqual(create, tt.create) {
				t.Errorf("create = %v, want %v", create, tt.create)
			}
			if !reflect.DeepEqual(del, tt.del) {
				t.Errorf("del = %v, want %v", del, tt.del)
			}
		})
	}

	t.Run("input slices not mutated", func(t *testing.T) {
		e := []Entry{{"1", srv, vm350.SPIFFEID, []string{"tpm_devid:subject:cn:350", "tpm_devid:ca:fingerprint:aa"}, true}}
		w := []Alias{{vm350.SPIFFEID, []string{"tpm_devid:subject:cn:350", "tpm_devid:ca:fingerprint:aa"}}}
		wantE := []string{"tpm_devid:subject:cn:350", "tpm_devid:ca:fingerprint:aa"}
		Diff("example.org", "cluster-1", e, w)
		if !reflect.DeepEqual(e[0].Selectors, wantE) || !reflect.DeepEqual(w[0].Selectors, wantE) {
			t.Errorf("inputs mutated: %v %v", e[0].Selectors, w[0].Selectors)
		}
	})
}

func agent(id, fp, vmid string, banned bool) Agent {
	return Agent{ID: id, Selectors: sel(fp, vmid), Banned: banned}
}

func TestBanTracker(t *testing.T) {
	fps := []string{"aa"}
	none := map[string]bool{}
	a350 := agent("a1", "aa", "350", false)

	t.Run("a destroyed VM", func(t *testing.T) {
		var b BanTracker
		if got := b.Observe(fps, none, []Agent{a350}); len(got) != 0 {
			t.Fatalf("first miss banned: %v", got)
		}
		want := []Ban{{"a1", "350"}}
		if got := b.Observe(fps, none, []Agent{a350}); !reflect.DeepEqual(got, want) {
			t.Errorf("second miss = %v, want %v", got, want)
		}
	})
	t.Run("a VM missing from one read only", func(t *testing.T) {
		var b BanTracker
		b.Observe(fps, none, []Agent{a350})
		b.Observe(fps, map[string]bool{"350": true}, []Agent{a350})
		if got := b.Observe(fps, none, []Agent{a350}); len(got) != 0 {
			t.Errorf("banned: %v", got)
		}
	})
	t.Run("count restarts after a ban is returned", func(t *testing.T) {
		var b BanTracker
		b.Observe(fps, none, []Agent{a350})
		b.Observe(fps, none, []Agent{a350})
		if got := b.Observe(fps, none, []Agent{a350}); len(got) != 0 {
			t.Fatalf("third call returned %v", got)
		}
		if got := b.Observe(fps, none, []Agent{a350}); len(got) != 1 {
			t.Errorf("fourth call = %v, want one ban", got)
		}
	})
	t.Run("an already banned agent", func(t *testing.T) {
		var b BanTracker
		a := agent("a1", "aa", "350", true)
		b.Observe(fps, none, []Agent{a})
		if got := b.Observe(fps, none, []Agent{a}); len(got) != 0 {
			t.Errorf("banned agent returned: %v", got)
		}
	})
	t.Run("another cluster's agent", func(t *testing.T) {
		var b BanTracker
		other := agent("o1", "bb", "7", false)
		b.Observe([]string{"bb"}, none, []Agent{other, a350})
		b.Observe(fps, none, []Agent{other, a350}) // a350 second miss, other ignored
		got := b.Observe([]string{"bb"}, none, []Agent{other, a350})
		if want := []Ban{{"o1", "7"}}; !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("an agent without a cn selector", func(t *testing.T) {
		var b BanTracker
		a := Agent{ID: "x", Selectors: []string{"tpm_devid:ca:fingerprint:aa"}}
		b.Observe(fps, none, []Agent{a})
		if got := b.Observe(fps, none, []Agent{a}); len(got) != 0 {
			t.Errorf("got %v", got)
		}
	})
	t.Run("an agent with two cn selectors", func(t *testing.T) {
		var b BanTracker
		a := Agent{ID: "x", Selectors: append(sel("aa", "1"), "tpm_devid:subject:cn:2")}
		b.Observe(fps, none, []Agent{a})
		if got := b.Observe(fps, none, []Agent{a}); len(got) != 0 {
			t.Errorf("got %v", got)
		}
	})
	t.Run("zero value usable", func(t *testing.T) {
		var b BanTracker
		if got := b.Observe(nil, nil, nil); len(got) != 0 {
			t.Errorf("got %v", got)
		}
	})
	t.Run("an agent that disappears loses its count", func(t *testing.T) {
		var b BanTracker
		b.Observe(fps, none, []Agent{a350})
		b.Observe(fps, none, nil)
		if got := b.Observe(fps, none, []Agent{a350}); len(got) != 0 {
			t.Errorf("got %v", got)
		}
	})
	t.Run("result sorted", func(t *testing.T) {
		var b BanTracker
		as := []Agent{agent("b", "aa", "2", false), agent("a", "aa", "1", false)}
		b.Observe(fps, none, as)
		want := []Ban{{"a", "1"}, {"b", "2"}}
		if got := b.Observe(fps, none, as); !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}
