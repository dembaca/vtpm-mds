package reconcile

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"testing"
)

// fakeSPIRE keeps entries in memory and records what was asked of it.
type fakeSPIRE struct {
	entries   []Entry
	agents    []Agent
	banned    []string
	createErr error
	next      int
}

func (f *fakeSPIRE) Entries(context.Context) ([]Entry, error) {
	return append([]Entry(nil), f.entries...), nil
}

func (f *fakeSPIRE) Create(_ context.Context, as []Alias) error {
	if f.createErr != nil {
		return f.createErr
	}
	for _, a := range as {
		f.next++
		f.entries = append(f.entries, Entry{ID: "e" + strconv.Itoa(f.next), ParentID: "spiffe://example.org/spire/server",
			SPIFFEID: a.SPIFFEID, Selectors: a.Selectors, Default: true})
	}
	return nil
}

func (f *fakeSPIRE) Delete(_ context.Context, ids []string) error {
	drop := map[string]bool{}
	for _, id := range ids {
		drop[id] = true
	}
	var keep []Entry
	for _, e := range f.entries {
		if !drop[e.ID] {
			keep = append(keep, e)
		}
	}
	f.entries = keep
	return nil
}

func (f *fakeSPIRE) Agents(context.Context) ([]Agent, error) { return f.agents, nil }

func (f *fakeSPIRE) Ban(_ context.Context, id string) error {
	f.banned = append(f.banned, id)
	for i := range f.agents {
		if f.agents[i].ID == id {
			f.agents[i].Banned = true
		}
	}
	return nil
}

// fakeReader returns its guests and error, both changeable between passes.
type fakeReader struct {
	guests []Guest
	err    error
}

func (r *fakeReader) Read(context.Context) ([]Guest, error) { return r.guests, r.err }

func newRec(s *fakeSPIRE, readers ...*fakeReader) *Reconciler {
	r := &Reconciler{TD: "example.org", SPIRE: s, Bans: &BanTracker{}, Log: log.New(io.Discard, "", 0)}
	for i, rd := range readers {
		name := fmt.Sprintf("cluster-%d", i+1)
		r.Targets = append(r.Targets, Target{Cluster: Cluster{Name: name, Fingerprints: []string{"f" + strconv.Itoa(i+1)}}, Reader: rd})
	}
	return r
}

func TestPass(t *testing.T) {
	ctx := context.Background()
	t.Run("a good pass creates aliases and a second changes nothing", func(t *testing.T) {
		s := &fakeSPIRE{}
		r := newRec(s, &fakeReader{guests: []Guest{{VMID: "350", Pool: "web"}}})
		if err := r.Pass(ctx, false); err != nil {
			t.Fatal(err)
		}
		if len(s.entries) != 2 {
			t.Fatalf("entries = %d, want 2", len(s.entries))
		}
		before := s.next
		if err := r.Pass(ctx, false); err != nil {
			t.Fatal(err)
		}
		if s.next != before || len(s.entries) != 2 {
			t.Errorf("second pass changed entries: next %d -> %d, entries %d", before, s.next, len(s.entries))
		}
	})
	t.Run("a doubtful read leaves that cluster alone", func(t *testing.T) {
		s := &fakeSPIRE{}
		a := &fakeReader{guests: []Guest{{VMID: "350"}}}
		b := &fakeReader{guests: []Guest{{VMID: "450"}}}
		r := newRec(s, a, b)
		if err := r.Pass(ctx, false); err != nil {
			t.Fatal(err)
		}
		a.err = fmt.Errorf("%w: down", ErrDoubtful)
		a.guests = nil
		b.guests = []Guest{{VMID: "451"}}
		if err := r.Pass(ctx, false); err != nil {
			t.Fatal(err)
		}
		have := map[string]bool{}
		for _, e := range s.entries {
			have[e.SPIFFEID] = true
		}
		for _, id := range []string{"cluster-1/vm/350", "cluster-2/vm/451"} {
			if !have["spiffe://example.org/pve/"+id] {
				t.Errorf("missing %s", id)
			}
		}
		if have["spiffe://example.org/pve/cluster-2/vm/450"] {
			t.Error("cluster-2 vm 450 was not removed")
		}
	})
	t.Run("a Create error is returned", func(t *testing.T) {
		s := &fakeSPIRE{createErr: errors.New("boom")}
		r := newRec(s, &fakeReader{guests: []Guest{{VMID: "350"}}})
		if err := r.Pass(ctx, false); err == nil {
			t.Error("want an error")
		}
	})
}

func TestPassBans(t *testing.T) {
	ctx := context.Background()
	agent := Agent{ID: "spiffe://example.org/spire/agent/tpm_devid/x", Selectors: []string{"tpm_devid:ca:fingerprint:f1", "tpm_devid:subject:cn:353"}}
	setup := func() (*fakeSPIRE, *fakeReader, *Reconciler) {
		s := &fakeSPIRE{agents: []Agent{agent}}
		rd := &fakeReader{guests: []Guest{{VMID: "350"}}}
		return s, rd, newRec(s, rd)
	}
	t.Run("a destroyed VM is banned on the second pass", func(t *testing.T) {
		s, _, r := setup()
		r.Pass(ctx, true)
		if len(s.banned) != 0 {
			t.Fatalf("banned after first pass: %v", s.banned)
		}
		r.Pass(ctx, true)
		if len(s.banned) != 1 || s.banned[0] != agent.ID {
			t.Errorf("banned = %v", s.banned)
		}
	})
	t.Run("a doubtful read does not advance the count", func(t *testing.T) {
		s, rd, r := setup()
		r.Pass(ctx, true)
		rd.err = fmt.Errorf("%w: down", ErrDoubtful)
		r.Pass(ctx, true)
		rd.err = nil
		if r.Pass(ctx, true); len(s.banned) != 1 {
			t.Errorf("banned = %v, want one on the third pass", s.banned)
		}
	})
	t.Run("no bans without allowBans", func(t *testing.T) {
		s, _, r := setup()
		for i := 0; i < 5; i++ {
			r.Pass(ctx, false)
		}
		if len(s.banned) != 0 {
			t.Errorf("banned = %v", s.banned)
		}
	})
}
