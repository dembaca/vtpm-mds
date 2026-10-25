package devid

import (
	"bytes"
	"context"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dembaca/vtpm-mds/internal/inventory" // pragma: allowlist secret
)

func writePinFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o640); err != nil {
		t.Fatal(err)
	}
}

func TestPinnedToAnotherVMScan(t *testing.T) {
	const fp = "abcdef"
	dir := t.TempDir()
	writePinFile(t, dir, "399", "ABCDEF\n")   // case-insensitive match
	writePinFile(t, dir, "398", "\n")         // empty: ignored
	writePinFile(t, dir, ".390.tmp-00ff", fp) // in-flight create: ignored
	writePinFile(t, dir, "..x", fp)           // invalid VM ID: ignored
	writePinFile(t, dir, "400", "0123")       // another EK
	if err := os.Mkdir(filepath.Join(dir, "397"), 0o750); err != nil {
		t.Fatal(err)
	}
	e := &Enroller{PinDir: dir}

	if got, err := e.pinnedToAnotherVM("390", fp); err != nil || got != "399" {
		t.Fatalf("other VM pinned: got %q err=%v", got, err)
	}
	if got, err := e.pinnedToAnotherVM("399", fp); err != nil || got != "" {
		t.Fatalf("own pin only: got %q err=%v", got, err)
	}

	// The other VM's inventory pin, through the seam.
	e.InventoryPins = func() map[string]string { return map[string]string{"100": "AAA", "101": ""} }
	if got, err := e.pinnedToAnotherVM("101", "aaa"); err != nil || got != "100" {
		t.Fatalf("inventory pin: got %q err=%v", got, err)
	}
	if got, err := e.pinnedToAnotherVM("100", "aaa"); err != nil || got != "" {
		t.Fatalf("own inventory pin: got %q err=%v", got, err)
	}

	// Store order before inventory order.
	e.InventoryPins = func() map[string]string { return map[string]string{"200": fp} }
	if got, _ := e.pinnedToAnotherVM("390", fp); got != "399" {
		t.Fatalf("store first: got %q", got)
	}
	if got, _ := e.pinnedToAnotherVM("399", fp); got != "200" {
		t.Fatalf("then inventory: got %q", got)
	}

	// A missing directory is an empty store.
	e = &Enroller{PinDir: filepath.Join(dir, "missing")}
	if got, err := e.pinnedToAnotherVM("390", fp); err != nil || got != "" {
		t.Fatalf("missing dir: got %q err=%v", got, err)
	}
}

// A pin file counts even where its VM's own inventory pin overrides it.
func TestPinFileCountsBesideInventoryOverride(t *testing.T) {
	dir := t.TempDir()
	writePinFile(t, dir, "399", "abc\n")
	e := &Enroller{PinDir: dir, InventoryPins: func() map[string]string { return map[string]string{"399": "def"} }}
	if got, _ := e.pinnedToAnotherVM("390", "abc"); got != "399" {
		t.Fatalf("got %q", got)
	}
}

func postEnrollFinishAs(t *testing.T, enroller *Enroller, vmid string, ekHeader string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(EnrollFinishRequest{
		SessionID:            "no-such-session",
		ChallengeResponseB64: base64.StdEncoding.EncodeToString([]byte("x")),
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/latest/devid/enroll/finish", bytes.NewReader(body))
	req.Header.Set(HeaderEKCert, ekHeader)
	vm := &inventory.VMConfig{VMID: vmid}
	req = req.WithContext(context.WithValue(req.Context(), inventory.VMConfigContextKey, vm))
	rr := httptest.NewRecorder()
	enroller.HandleFinish(rr, req)
	return rr
}

func TestEnrollLegsRefuseEKPinnedToAnotherVM(t *testing.T) {
	for _, source := range []string{"store", "inventory"} {
		t.Run(source, func(t *testing.T) {
			ekCert, ekKey, roots := testEKMaterial(t)
			fp := EKFingerprint(ekCert)
			requestData, sig, _ := fakeSignedEnrollRequest(t, ekCert, rsaKeyToEKPub(ekKey))
			store := NewSessionStore(time.Minute)
			enroller := NewEnroller(mustTestDevIDCA(t), roots, store)
			dir := t.TempDir()
			enroller.PinDir = dir
			if source == "store" {
				writePinFile(t, dir, "399", fp+"\n")
			} else {
				enroller.InventoryPins = func() map[string]string { return map[string]string{"399": fp} }
			}

			var start, finish *httptest.ResponseRecorder
			logs := captureLog(t, func() {
				start = postEnrollStart(t, enroller, ekCert, requestData, sig) // as VM 100
				finish = postEnrollFinishAs(t, enroller, "100", EncodeEKCertHeader(ekCert))
			})
			for leg, rr := range map[string]*httptest.ResponseRecorder{"start": start, "finish": finish} {
				if rr.Code != http.StatusUnauthorized {
					t.Fatalf("%s: expected 401, got %d body=%q", leg, rr.Code, rr.Body.String())
				}
				if got := strings.TrimSpace(rr.Body.String()); got != errPinnedElsewhere {
					t.Fatalf("%s: body=%q want %q", leg, got, errPinnedElsewhere)
				}
			}
			store.mu.Lock()
			n := len(store.sessions)
			store.mu.Unlock()
			if n != 0 {
				t.Fatalf("challenge issued: %d sessions", n)
			}
			if _, err := os.Stat(pinPath(dir, "100")); !os.IsNotExist(err) {
				t.Fatalf("pin file for caller created: %v", err)
			}
			want := "devid enroll auth: EK certificate is pinned to another VM vm=100 pinned=399"
			lines := strings.Split(strings.TrimSpace(logs), "\n")
			if len(lines) != 2 {
				t.Fatalf("want one line per leg, got %q", logs)
			}
			for _, l := range lines {
				if !strings.HasSuffix(l, want) {
					t.Fatalf("log line %q, want suffix %q", l, want)
				}
			}
			if strings.Contains(strings.ToLower(logs), fp) {
				t.Fatalf("log contains the fingerprint: %q", logs)
			}
		})
	}
}

// The VM that holds the pin is not refused by the cross-VM check.
func TestPinHolderStillAuthenticates(t *testing.T) {
	ekCert, ekKey, roots := testEKMaterial(t)
	dir := t.TempDir()
	writePinFile(t, dir, "100", EKFingerprint(ekCert)+"\n")
	enroller := NewEnroller(&CA{}, roots, nil)
	enroller.PinDir = dir
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(HeaderEKCert, EncodeEKCertHeader(ekCert))
	req = req.WithContext(context.WithValue(req.Context(), inventory.VMConfigContextKey, &inventory.VMConfig{VMID: "100"}))
	if _, _, err := enroller.AuthenticateEnrollCaller(req, ekCert, rsaKeyToEKPub(ekKey), ""); err != nil {
		t.Fatalf("pin holder refused: %v", err)
	}
}

func TestUnlistablePinDirFailsClosed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can list a mode 0300 directory")
	}
	ekCert, ekKey, roots := testEKMaterial(t)
	requestData, sig, _ := fakeSignedEnrollRequest(t, ekCert, rsaKeyToEKPub(ekKey))
	enroller := NewEnroller(mustTestDevIDCA(t), roots, NewSessionStore(time.Minute))
	dir := filepath.Join(t.TempDir(), "ek-pins")
	if err := os.Mkdir(dir, 0o300); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o750) })
	enroller.PinDir = dir

	var start, finish *httptest.ResponseRecorder
	_ = captureLog(t, func() {
		start = postEnrollStart(t, enroller, ekCert, requestData, sig)
		finish = postEnrollFinishAs(t, enroller, "100", EncodeEKCertHeader(ekCert))
	})
	for leg, rr := range map[string]*httptest.ResponseRecorder{"start": start, "finish": finish} {
		if rr.Code != http.StatusInternalServerError || strings.TrimSpace(rr.Body.String()) != errPinStoreUnavailable {
			t.Fatalf("%s: got %d %q", leg, rr.Code, rr.Body.String())
		}
	}
	if err := enroller.ensurePinBeforeIssue("100", "abc"); !errors.Is(err, ErrPinStore) {
		t.Fatalf("ensurePinBeforeIssue: want ErrPinStore, got %v", err)
	}
}

// Two first finishes with one EK under two VM IDs: one pin, one LDevID (D-a2).
func TestConcurrentFirstFinishOneEKTwoVMs(t *testing.T) {
	ca := mustParseTestCA(t)
	// Hold each finish between scan and create until the other one arrives
	// too, or 100 ms pass: without pinCreateMu both would pass the scan.
	arrived := make(chan struct{}, 2)
	beforePinCreate = func() {
		arrived <- struct{}{}
		deadline := time.After(100 * time.Millisecond)
		for len(arrived) < 2 {
			select {
			case <-deadline:
				return
			case <-time.After(time.Millisecond):
			}
		}
	}
	t.Cleanup(func() { beforePinCreate = nil })
	for round := 0; round < 5; round++ {
		for len(arrived) > 0 {
			<-arrived
		}
		dir := t.TempDir()
		enroller := NewEnroller(ca, nil, NewSessionStore(time.Minute))
		enroller.PinDir = dir

		type result struct {
			pem []byte
			err error
		}
		vmids := []string{"390", "399"}
		out := make(chan result, len(vmids))
		// Both sessions exist before either finish starts.
		sids := make([]string, len(vmids))
		for i, vmid := range vmids {
			sr := mustMinimalSigningRequest(t, pkix.Name{})
			sid, err := enroller.Sessions.Put([]byte{byte('a' + i)}, *sr, vmid, "same-ek")
			if err != nil {
				t.Fatal(err)
			}
			sids[i] = sid
		}
		var wg sync.WaitGroup
		for i, vmid := range vmids {
			wg.Add(1)
			go func(sid, vmid string, nonce []byte) {
				defer wg.Done()
				pemBytes, err := enroller.Finish(sid, nonce, vmid, "same-ek", "")
				out <- result{pem: pemBytes, err: err}
			}(sids[i], vmid, []byte{byte('a' + i)})
		}
		wg.Wait()
		close(out)
		var issued, refused int
		for r := range out {
			switch {
			case r.err == nil:
				if block, _ := pem.Decode(r.pem); block == nil {
					t.Fatal("expected PEM")
				}
				issued++
			case isUnauthorized(r.err) && responseBody(r.err) == errPinnedElsewhere:
				if r.pem != nil {
					t.Fatal("refused finish returned PEM")
				}
				refused++
			default:
				t.Fatalf("unexpected: %v", r.err)
			}
		}
		if issued != 1 || refused != 1 {
			t.Fatalf("round %d: issued=%d refused=%d", round, issued, refused)
		}
		holders := 0
		for _, vmid := range vmids {
			if got, _ := readStoredPin(dir, vmid); got == "same-ek" {
				holders++
			}
		}
		if holders != 1 {
			t.Fatalf("round %d: %d pin files hold the EK", round, holders)
		}
	}
}
