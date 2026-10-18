package imds

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dembaca/vtpm-mds/internal/inventory"
)

// tagged is a tokened request from a connection bound to a VM record carrying
// the given instance tags.
func tagged(t *testing.T, path string, tags map[string]string) *http.Request {
	t.Helper()
	req := unboundRequest(t, path)
	return req.WithContext(context.WithValue(req.Context(),
		inventory.VMConfigContextKey, &inventory.VMConfig{VMID: "100", Tags: tags}))
}

func get(h http.HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func wantEmpty404(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusNotFound || rec.Body.Len() != 0 {
		t.Fatalf("got %d %q, want 404 with an empty body", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain" {
		t.Fatalf("Content-Type %q, want text/plain", ct)
	}
}

var twoTags = map[string]string{"rack": "b.2", "owner": "team-a"}

// A guest lists its tags: keys, ascending, each terminated by a newline.
func TestAGuestListsItsTags(t *testing.T) {
	setupTestStore()
	rec := get(HandleInstanceTagKeys, tagged(t, "/latest/meta-data/tags/instance", twoTags))
	if rec.Code != http.StatusOK || rec.Body.String() != "owner\nrack\n" {
		t.Fatalf("got %d %q, want 200 \"owner\\nrack\\n\"", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain" {
		t.Fatalf("Content-Type %q, want text/plain", ct)
	}
}

// One tag is served with its dots and no trailing newline.
func TestAGuestReadsOneTag(t *testing.T) {
	setupTestStore()
	req := tagged(t, "/latest/meta-data/tags/instance/rack", twoTags)
	req.SetPathValue("key", "rack")
	rec := get(HandleInstanceTag, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "b.2" {
		t.Fatalf("got %d %q, want 200 \"b.2\"", rec.Code, rec.Body.String())
	}
}

// Keys are case-sensitive, as EC2's are.
func TestTagKeysAreCaseSensitive(t *testing.T) {
	setupTestStore()
	req := tagged(t, "/latest/meta-data/tags/instance/Owner", twoTags)
	req.SetPathValue("key", "Owner")
	wantEmpty404(t, get(HandleInstanceTag, req))
}

// tags/ names the one directory.
func TestTheTagsDirectoryNamesTheInstanceTags(t *testing.T) {
	setupTestStore()
	rec := get(HandleTags, tagged(t, "/latest/meta-data/tags/", twoTags))
	if rec.Code != http.StatusOK || rec.Body.String() != "instance\n" {
		t.Fatalf("got %d %q, want 200 \"instance\\n\"", rec.Code, rec.Body.String())
	}
}

// A VM without tags, an unknown key and an unbound caller get the empty 404 on
// all three paths.
func TestNothingToServeIsAnEmpty404(t *testing.T) {
	setupTestStore()
	for name, tags := range map[string]map[string]string{"no tags": nil, "empty": {}} {
		wantEmpty404(t, get(HandleTags, tagged(t, "/latest/meta-data/tags/", tags)))
		wantEmpty404(t, get(HandleInstanceTagKeys, tagged(t, "/latest/meta-data/tags/instance", tags)))
		req := tagged(t, "/latest/meta-data/tags/instance/owner", tags)
		req.SetPathValue("key", "owner")
		wantEmpty404(t, get(HandleInstanceTag, req))
		_ = name
	}
	req := tagged(t, "/latest/meta-data/tags/instance/missing", twoTags)
	req.SetPathValue("key", "missing")
	wantEmpty404(t, get(HandleInstanceTag, req))

	wantEmpty404(t, get(HandleInstanceTagKeys, unboundRequest(t, "/latest/meta-data/tags/instance")))
	wantEmpty404(t, get(HandleTags, unboundRequest(t, "/latest/meta-data/tags/")))
	ureq := unboundRequest(t, "/latest/meta-data/tags/instance/owner")
	ureq.SetPathValue("key", "owner")
	wantEmpty404(t, get(HandleInstanceTag, ureq))
}

// Every tags path needs a session token, like every metadata read.
func TestTagsNeedAToken(t *testing.T) {
	setupTestStore()
	for name, h := range map[string]http.HandlerFunc{"tags/": HandleTags, "tags/instance": HandleInstanceTagKeys, "tags/instance/{key}": HandleInstanceTag} {
		rec := get(h, httptest.NewRequest("GET", "/latest/meta-data/"+name, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: expected 401 without a token, got %d", name, rec.Code)
		}
	}
}

// The index lists tags/ for a VM that has some, and for no other caller.
func TestTheIndexListsTagsOnlyForAVMThatHasSome(t *testing.T) {
	setupTestStore()
	with := get(HandleMetaDataIndex, tagged(t, "/latest/meta-data/", twoTags)).Body.String()
	if !strings.Contains(with, "tags/\n") {
		t.Errorf("a VM with tags: index lacks tags/: %q", with)
	}
	for name, req := range map[string]*http.Request{
		"no tags":  tagged(t, "/latest/meta-data/", nil),
		"unbound":  unboundRequest(t, "/latest/meta-data/"),
		"any else": boundRequest(t, "/latest/meta-data/", "100"),
	} {
		if body := get(HandleMetaDataIndex, req).Body.String(); strings.Contains(body, "tags/") {
			t.Errorf("%s: index lists tags/: %q", name, body)
		}
	}
}

// The tags are not in the identity document.
func TestTheTagsAreNotInTheIdentityDocument(t *testing.T) {
	setupTestStore()
	rec := get(HandleInstanceIdentityDocument, tagged(t, "/latest/dynamic/instance-identity/document", twoTags))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	for _, leak := range []string{"team-a", "b.2", "owner", "rack", "tags"} {
		if strings.Contains(rec.Body.String(), leak) {
			t.Errorf("the document mentions %q: %s", leak, rec.Body.String())
		}
	}
}
