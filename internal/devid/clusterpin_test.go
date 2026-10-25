package devid

import (
	"context"
	"crypto/x509/pkix"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dembaca/vtpm-mds/internal/inventory" // pragma: allowlist secret
)

const testHexPin = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

func TestParseClusterPin(t *testing.T) {
	for _, tc := range []struct {
		in string
		ok bool
	}{
		{testHexPin, true},
		{testHexPin + "\n", true},
		{strings.ToUpper(testHexPin), false},
		{testHexPin + "\n\n", false},
		{testHexPin + "\nsecond", false},
		{" " + testHexPin, false},
		{testHexPin[:63], false},
		{"", false},
		{strings.Repeat("g", 64), false},
	} {
		got, ok := parseClusterPin([]byte(tc.in))
		if ok != tc.ok || (ok && got != testHexPin) {
			t.Fatalf("parseClusterPin(%q) = %q, %v; want ok=%v", tc.in, got, ok, tc.ok)
		}
	}
}

func authRequestAs(vmid string, cert string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(HeaderEKCert, cert)
	return req.WithContext(context.WithValue(req.Context(), inventory.VMConfigContextKey, &inventory.VMConfig{VMID: vmid}))
}

// Own pin: cluster pin before local pin, strict format, the three log lines.
func TestAuthenticateClusterPin(t *testing.T) {
	cert, key, roots := testEKMaterial(t)
	other, _, _ := testEKMaterial(t)
	fp, otherFP := EKFingerprint(cert), EKFingerprint(other)
	ekPub := rsaKeyToEKPub(key)
	newEnroller := func() (*Enroller, string, string) {
		e := NewEnroller(&CA{}, roots, nil)
		e.PinDir = t.TempDir()
		e.ClusterPinDir = filepath.Join(t.TempDir(), "vtpm-ek")
		return e, e.PinDir, e.ClusterPinDir
	}
	auth := func(e *Enroller) (error, string) {
		var err error
		logs := captureLog(t, func() {
			_, _, err = e.AuthenticateEnrollCaller(authRequestAs("100", EncodeEKCertHeader(cert)), cert, ekPub, "")
		})
		return err, logs
	}

	t.Run("missing dir is quiet", func(t *testing.T) {
		e, _, _ := newEnroller()
		if err, logs := auth(e); err != nil || logs != "" {
			t.Fatalf("err=%v logs=%q", err, logs)
		}
	})
	t.Run("dir without pin warns", func(t *testing.T) {
		e, _, cdir := newEnroller()
		if err := os.Mkdir(cdir, 0o755); err != nil {
			t.Fatal(err)
		}
		err, logs := auth(e)
		if err != nil || !strings.Contains(logs, "devid enroll: no cluster pin vm=100, using local pin") {
			t.Fatalf("err=%v logs=%q", err, logs)
		}
	})
	t.Run("valid pin enforced", func(t *testing.T) {
		e, _, cdir := newEnroller()
		_ = os.Mkdir(cdir, 0o755)
		writePinFile(t, cdir, "100", otherFP+"\n")
		if err, _ := auth(e); !isUnauthorized(err) || err.Error() != errClusterPinMismatch {
			t.Fatalf("want cluster pin mismatch, got %v", err)
		}
		writePinFile(t, cdir, "100", fp+"\n")
		if err, logs := auth(e); err != nil || logs != "" {
			t.Fatalf("matching cluster pin: err=%v logs=%q", err, logs)
		}
	})
	t.Run("cluster pin wins over differing local pin", func(t *testing.T) {
		e, dir, cdir := newEnroller()
		_ = os.Mkdir(cdir, 0o755)
		writePinFile(t, cdir, "100", fp)
		writePinFile(t, dir, "100", otherFP+"\n")
		err, logs := auth(e)
		if err != nil || !strings.Contains(logs, "devid enroll: local pin differs from cluster pin vm=100") {
			t.Fatalf("err=%v logs=%q", err, logs)
		}
		if strings.Contains(logs, fp) || strings.Contains(logs, otherFP) {
			t.Fatalf("log carries a fingerprint: %q", logs)
		}
		if got, _ := readStoredPin(dir, "100"); got != otherFP {
			t.Fatalf("local pin changed: %q", got)
		}
	})
	for name, bad := range map[string]string{"upper case": strings.ToUpper(fp), "second line": fp + "\nx", "empty": ""} {
		t.Run("malformed is no pin: "+name, func(t *testing.T) {
			e, dir, cdir := newEnroller()
			_ = os.Mkdir(cdir, 0o755)
			writePinFile(t, cdir, "100", bad)
			writePinFile(t, dir, "100", otherFP)
			err, logs := auth(e)
			if !strings.Contains(logs, "devid enroll: invalid cluster pin vm=100") ||
				!strings.Contains(logs, "no cluster pin vm=100, using local pin") {
				t.Fatalf("logs=%q", logs)
			}
			if !isUnauthorized(err) || err.Error() != errStoredPinMismatch {
				t.Fatalf("local pin must decide, got %v", err)
			}
		})
	}
	t.Run("required pin refuses", func(t *testing.T) {
		e, _, _ := newEnroller()
		e.RequireClusterPin = true
		if err, _ := auth(e); !isUnauthorized(err) || err.Error() != errNoClusterPin {
			t.Fatalf("want %q, got %v", errNoClusterPin, err)
		}
	})
	t.Run("inventory pin wins", func(t *testing.T) {
		e, _, cdir := newEnroller()
		e.RequireClusterPin = true
		_ = os.Mkdir(cdir, 0o755)
		writePinFile(t, cdir, "100", otherFP)
		req := authRequestAs("100", EncodeEKCertHeader(cert))
		inventory.GetVMConfigFromRequest(req).EKSHA256 = fp
		if _, _, err := e.AuthenticateEnrollCaller(req, cert, ekPub, ""); err != nil {
			t.Fatalf("inventory pin must win: %v", err)
		}
	})
}

// Finish creates no local pin beside a cluster pin, and none when one is
// required and missing.
func TestFinishWithClusterPin(t *testing.T) {
	ca := mustParseTestCA(t)
	sr := mustMinimalSigningRequest(t, pkix.Name{})
	e := NewEnroller(ca, nil, NewSessionStore(time.Minute))
	e.PinDir = t.TempDir()
	e.ClusterPinDir = t.TempDir()
	writePinFile(t, e.ClusterPinDir, "100", testHexPin+"\n")

	sid, err := e.Sessions.Put([]byte("n"), *sr, "100", testHexPin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Finish(sid, []byte("n"), "100", testHexPin, ""); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if _, err := os.Stat(pinPath(e.PinDir, "100")); !os.IsNotExist(err) {
		t.Fatalf("local pin created beside cluster pin: %v", err)
	}

	e.RequireClusterPin = true
	sid, _ = e.Sessions.Put([]byte("m"), *sr, "101", testHexPin)
	if _, err := e.Finish(sid, []byte("m"), "101", testHexPin, ""); !isUnauthorized(err) || err.Error() != errNoClusterPin {
		t.Fatalf("want %q, got %v", errNoClusterPin, err)
	}
	if _, err := os.Stat(pinPath(e.PinDir, "101")); !os.IsNotExist(err) {
		t.Fatalf("local pin created although a cluster pin is required: %v", err)
	}
}

// writeGuestConfig creates <root>/<node>/<kind>/<vmid>.conf.
func writeGuestConfig(t *testing.T, root, node, kind, vmid string) {
	t.Helper()
	d := filepath.Join(root, node, kind)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, vmid+".conf"), []byte("name: x\n"), 0o640); err != nil {
		t.Fatal(err)
	}
}

func TestPinnedToAnotherVMClusterAndStale(t *testing.T) {
	cdir, dir, nodes := t.TempDir(), t.TempDir(), t.TempDir()
	writeGuestConfig(t, nodes, "holger", "qemu-server", "399")
	e := &Enroller{PinDir: dir, ClusterPinDir: cdir, GuestConfigRoot: nodes}

	// A clone is refused by a cluster pin of a VM on another node.
	writePinFile(t, cdir, "399", testHexPin+"\n")
	writePinFile(t, cdir, "398", strings.ToUpper(testHexPin)) // malformed: ignored
	if got, err := e.pinnedToAnotherVM("390", testHexPin); err != nil || got != "399" {
		t.Fatalf("cluster pin: got %q err=%v", got, err)
	}
	if got, _ := e.pinnedToAnotherVM("399", testHexPin); got != "" {
		t.Fatalf("own cluster pin: got %q", got)
	}

	// A destroyed VM's cluster and local pins refuse nobody.
	writePinFile(t, dir, "500", testHexPin+"\n")
	writePinFile(t, cdir, "500", testHexPin+"\n")
	if err := os.Remove(filepath.Join(cdir, "399")); err != nil {
		t.Fatal(err)
	}
	if got, err := e.pinnedToAnotherVM("450", testHexPin); err != nil || got != "" {
		t.Fatalf("stale pins: got %q err=%v", got, err)
	}

	// A container config counts as existing.
	writeGuestConfig(t, nodes, "horst", "lxc", "500")
	if got, _ := e.pinnedToAnotherVM("450", testHexPin); got != "500" {
		t.Fatalf("lxc counts: got %q", got)
	}

	// No guest-config tree (YAML backend): nothing is ignored.
	e = &Enroller{PinDir: dir}
	writePinFile(t, dir, "600", "abc\n")
	if got, _ := e.pinnedToAnotherVM("601", "abc"); got != "600" {
		t.Fatalf("YAML backend: got %q", got)
	}
}

func TestUnlistableGuestConfigRootIgnoresNothing(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can list a mode 0300 directory")
	}
	dir := t.TempDir()
	nodes := filepath.Join(t.TempDir(), "nodes")
	if err := os.Mkdir(nodes, 0o300); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(nodes, 0o755) })
	writePinFile(t, dir, "399", "abc\n")
	e := &Enroller{PinDir: dir, GuestConfigRoot: nodes}
	var got string
	logs := captureLog(t, func() { got, _ = e.pinnedToAnotherVM("390", "abc") })
	if got != "399" || !strings.Contains(logs, "no pin is treated as stale") {
		t.Fatalf("got %q logs=%q", got, logs)
	}
}

func TestUnusableClusterPinDirFailsClosed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read mode 0300 and 0200 entries")
	}
	ekCert, ekKey, roots := testEKMaterial(t)
	requestData, sig, _ := fakeSignedEnrollRequest(t, ekCert, rsaKeyToEKPub(ekKey))

	t.Run("unlistable directory", func(t *testing.T) {
		e := NewEnroller(mustTestDevIDCA(t), roots, NewSessionStore(time.Minute))
		e.PinDir = t.TempDir()
		e.ClusterPinDir = filepath.Join(t.TempDir(), "vtpm-ek")
		if err := os.Mkdir(e.ClusterPinDir, 0o300); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(e.ClusterPinDir, 0o755) })
		var start, finish *httptest.ResponseRecorder
		_ = captureLog(t, func() {
			start = postEnrollStart(t, e, ekCert, requestData, sig)
			finish = postEnrollFinishAs(t, e, "100", EncodeEKCertHeader(ekCert))
		})
		for leg, rr := range map[string]*httptest.ResponseRecorder{"start": start, "finish": finish} {
			if rr.Code != http.StatusInternalServerError || strings.TrimSpace(rr.Body.String()) != errPinStoreUnavailable {
				t.Fatalf("%s: got %d %q", leg, rr.Code, rr.Body.String())
			}
		}
		if _, err := e.clusterPinsMatching("390", testHexPin); !errors.Is(err, ErrPinStore) {
			t.Fatalf("scan: want ErrPinStore, got %v", err)
		}
	})
	t.Run("unreadable own pin", func(t *testing.T) {
		e := &Enroller{ClusterPinDir: t.TempDir()}
		writePinFile(t, e.ClusterPinDir, "100", testHexPin)
		if err := os.Chmod(filepath.Join(e.ClusterPinDir, "100"), 0o200); err != nil {
			t.Fatal(err)
		}
		if _, _, err := e.readClusterPin("100", false); !errors.Is(err, ErrPinStore) {
			t.Fatalf("want ErrPinStore, got %v", err)
		}
	})
}

// Both legs, through the HTTP handlers: a required pin refuses with 401, and
// a malformed pin logs one "invalid cluster pin" line per leg.
func TestClusterPinHandlers(t *testing.T) {
	ekCert, ekKey, roots := testEKMaterial(t)
	requestData, sig, _ := fakeSignedEnrollRequest(t, ekCert, rsaKeyToEKPub(ekKey))
	legs := func(e *Enroller) (map[string]*httptest.ResponseRecorder, string) {
		var start, finish *httptest.ResponseRecorder
		logs := captureLog(t, func() {
			start = postEnrollStart(t, e, ekCert, requestData, sig) // as VM 100
			finish = postEnrollFinishAs(t, e, "100", EncodeEKCertHeader(ekCert))
		})
		return map[string]*httptest.ResponseRecorder{"start": start, "finish": finish}, logs
	}

	e := NewEnroller(mustTestDevIDCA(t), roots, NewSessionStore(time.Minute))
	e.PinDir = t.TempDir()
	e.ClusterPinDir = t.TempDir()
	e.RequireClusterPin = true
	rrs, _ := legs(e)
	for leg, rr := range rrs {
		if rr.Code != http.StatusUnauthorized || strings.TrimSpace(rr.Body.String()) != errNoClusterPin {
			t.Fatalf("%s: got %d %q", leg, rr.Code, rr.Body.String())
		}
	}
	if _, err := os.Stat(pinPath(e.PinDir, "100")); !os.IsNotExist(err) {
		t.Fatalf("local pin created: %v", err)
	}

	e.RequireClusterPin = false
	writePinFile(t, e.ClusterPinDir, "100", "not a pin")
	_, logs := legs(e)
	if n := strings.Count(logs, "devid enroll: invalid cluster pin vm=100"); n != 2 {
		t.Fatalf("want one invalid-pin line per leg (2), got %d: %q", n, logs)
	}
}

// Finish re-reads the cluster pin under pinCreateMu: a pin written after
// the auth check still prevents the local create, and a mismatching one
// refuses.
func TestEnsurePinHonoursLateClusterPin(t *testing.T) {
	e := &Enroller{PinDir: t.TempDir(), ClusterPinDir: t.TempDir()}
	writePinFile(t, e.ClusterPinDir, "100", testHexPin)
	if err := e.ensurePinBeforeIssue("100", testHexPin); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(pinPath(e.PinDir, "100")); !os.IsNotExist(err) {
		t.Fatalf("local pin created beside a cluster pin: %v", err)
	}
	if err := e.ensurePinBeforeIssue("100", strings.Repeat("1", 64)); !isUnauthorized(err) || err.Error() != errClusterPinMismatch {
		t.Fatalf("want %q, got %v", errClusterPinMismatch, err)
	}
}

// A pin removed between the listing and the read is skipped, and a
// directory under a VM ID's name is no pin.
func TestClusterPinVanishedAndDirectory(t *testing.T) {
	e := &Enroller{ClusterPinDir: t.TempDir()}
	if err := os.Mkdir(filepath.Join(e.ClusterPinDir, "100"), 0o755); err != nil {
		t.Fatal(err)
	}
	var pin string
	var err error
	logs := captureLog(t, func() { pin, _, err = e.readClusterPin("100", false) })
	if err != nil || pin != "" || !strings.Contains(logs, "invalid cluster pin vm=100") {
		t.Fatalf("directory: pin=%q err=%v logs=%q", pin, err, logs)
	}
	// Dangling symlink: listed by ReadDir, ENOENT on read.
	if err := os.Symlink(filepath.Join(e.ClusterPinDir, "gone"), filepath.Join(e.ClusterPinDir, "399")); err != nil {
		t.Fatal(err)
	}
	if ids, err := e.clusterPinsMatching("390", testHexPin); err != nil || len(ids) != 0 {
		t.Fatalf("vanished pin: ids=%v err=%v", ids, err)
	}
}
