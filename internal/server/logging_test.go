package server

import (
	"context"
	"net"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"log"

	"github.com/dembaca/vtpm-mds/internal/inventory"
)

// logged runs f and returns what it wrote to the standard logger.
func logged(t *testing.T, f func()) string {
	t.Helper()
	var buf strings.Builder
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	f()
	return buf.String()
}

func withDebug(t *testing.T, on bool) {
	t.Helper()
	SetDebug(on)
	t.Cleanup(func() { SetDebug(false) })
}

// request sends a tokened GET and returns the log it wrote.
func request(t *testing.T, srv *Server, token, path, remote string, bind *inventory.VMConfig, headers map[string]string) string {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	req.RemoteAddr = remote
	req.Header.Set("X-Aws-Ec2-Metadata-Token", token)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if bind != nil {
		req = req.WithContext(context.WithValue(req.Context(), inventory.VMConfigContextKey, bind))
	}
	return logged(t, func() { serve(srv, req) })
}

// A normal run writes no [DEBUG line, for a request or for a reload; the access
// line and the reload summary are written.
func TestANormalRunWritesNoDebugLine(t *testing.T) {
	withDebug(t, false)
	srv, token := signatureTestServer(t)
	out := request(t, srv, token, "/latest/meta-data/instance-id", "10.0.0.5:5000", &inventory.VMConfig{VMID: "100"}, nil)
	out += logged(t, func() {
		if err := srv.RefreshVMConfigCache(); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Contains(out, "[DEBUG") {
		t.Fatalf("a [DEBUG line was written without -debug: %q", out)
	}
	if !strings.Contains(out, "Cache refresh complete") {
		t.Errorf("the reload summary must be written without the flag: %q", out)
	}
	if !strings.Contains(out, "/latest/meta-data/instance-id") {
		t.Errorf("the access line must be written without the flag: %q", out)
	}
}

// A debug run writes the debug lines of a reload as before.
func TestADebugRunWritesTheDebugLines(t *testing.T) {
	withDebug(t, true)
	srv, _ := signatureTestServer(t)
	out := logged(t, func() {
		if err := srv.RefreshVMConfigCache(); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "[DEBUG RefreshVMConfigCache] Loading YAML inventory") {
		t.Fatalf("the debug lines are missing with -debug: %q", out)
	}
}

// A failed load and a connection of an unexpected type are errors, and are
// written without the flag and without the debug marker.
func TestErrorsStayVisibleWithoutTheFlag(t *testing.T) {
	withDebug(t, false)
	srv, _ := signatureTestServer(t)
	srv.cfg.MDS.InventoryPath = "../../testdata/vm-inventory/does-not-exist.yaml"
	out := logged(t, func() {
		if err := srv.RefreshVMConfigCache(); err == nil {
			t.Fatal("expected the load to fail")
		}
	})
	if !strings.Contains(out, "Failed to load VM configs from") || strings.Contains(out, "[DEBUG") {
		t.Errorf("a failed load must be written, unmarked, without the flag: %q", out)
	}

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	out = logged(t, func() { connContext(context.Background(), c1) })
	if !strings.Contains(out, "not a vmConnection") || strings.Contains(out, "[DEBUG") {
		t.Errorf("a connection of an unexpected type must be written, unmarked: %q", out)
	}
}

// The access line holds method, quoted path, status, duration, peer and VM, in
// that order, for a bound caller.
func TestTheAccessLineNamesThePeerAndTheVM(t *testing.T) {
	withDebug(t, false)
	srv, token := signatureTestServer(t)
	out := strings.TrimSpace(request(t, srv, token, "/latest/meta-data/instance-id", "10.0.0.5:5000", &inventory.VMConfig{VMID: "350"}, nil))
	f := strings.Fields(out)
	// date, time, then the fields of the line
	if len(f) < 8 {
		t.Fatalf("unexpected line: %q", out)
	}
	got := f[len(f)-6:]
	if got[0] != "GET" || got[1] != `"/latest/meta-data/instance-id"` || got[2] != "200" || got[4] != "10.0.0.5" || got[5] != "350" {
		t.Fatalf("got %q", got)
	}
}

// An unbound caller is named too, with "-" for the VM, and an unparseable peer is
// "-".
func TestAnUnboundCallerAndAnUnparseablePeer(t *testing.T) {
	withDebug(t, false)
	srv, token := signatureTestServer(t)
	f := strings.Fields(strings.TrimSpace(request(t, srv, token, "/health", "10.0.0.9:1234", nil, nil)))
	if got := f[len(f)-2:]; got[0] != "10.0.0.9" || got[1] != "-" {
		t.Errorf("unbound: got %q", got)
	}
	f = strings.Fields(strings.TrimSpace(request(t, srv, token, "/health", "not-an-address", nil, nil)))
	if got := f[len(f)-2:]; got[0] != "-" || got[1] != "-" {
		t.Errorf("unparseable peer: got %q", got)
	}
}

// The peer is the connection's address, never a header.
func TestThePeerIsNotTakenFromAHeader(t *testing.T) {
	withDebug(t, false)
	srv, token := signatureTestServer(t)
	out := request(t, srv, token, "/health", "10.0.0.5:5000", nil, map[string]string{"X-Forwarded-For": "10.9.9.9"})
	if strings.Contains(out, "10.9.9.9") || !strings.Contains(out, "10.0.0.5") {
		t.Fatalf("the peer must be 10.0.0.5: %q", out)
	}
}

// A path that holds a line break writes one line, with the break escaped.
func TestAPathCannotWriteASecondLine(t *testing.T) {
	withDebug(t, false)
	srv, token := signatureTestServer(t)
	req := httptest.NewRequest("GET", "/latest/meta-data/x", nil)
	req.URL.Path = "/latest/x\n2026/10/18 00:00:00 forged line"
	req.RemoteAddr = "10.0.0.5:5000"
	req.Header.Set("X-Aws-Ec2-Metadata-Token", token)
	out := logged(t, func() { serve(srv, req) })
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// the unmatched-route line and the access line: one line each, no more
	if len(lines) != 2 {
		t.Fatalf("expected two lines (unmatched route, access), got %d: %q", len(lines), out)
	}
	for _, l := range lines {
		if strings.Contains(l, "\\n") == false {
			t.Errorf("the break is not escaped in %q", l)
		}
	}
}
