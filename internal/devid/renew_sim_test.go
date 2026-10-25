//go:build tpmsim

// These tests run ClientEnrollOrRenew and LoadDevID against go-tpm-tools'
// in-process TPM simulator, behind the same build tag as srk_sim_test.go:
//
//	CGO_CFLAGS=-I<openssl>/include CGO_LDFLAGS=-L<openssl>/lib \
//	  go test -tags tpmsim ./internal/devid/ -run 'Renew|LoadDevID' -v
//
// They check when the guest renews and which DevID key it uses
// (renew-when-the-devid-key-is-not-this-tpms, tasks 1.1 to 1.5). The simulator
// holds three objects at once, like swtpm, so a run that needed a fourth would
// fail here with TPM_RC_OBJECT_MEMORY. It is a third TPM implementation beside
// swtpm/libtpms on hogan and in the lab, so a pass shows the logic, not those.
package devid

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-tpm-tools/simulator"
	"github.com/google/go-tpm/legacy/tpm2"
)

// loadAnswer forwards every command to rw except TPM2_Load, which it answers
// with the response code rc without passing it on.
type loadAnswer struct {
	rw      io.ReadWriter
	rc      uint32
	pending []byte
	loads   int
	cmds    int
}

func (l *loadAnswer) Write(p []byte) (int, error) {
	l.cmds++
	if len(p) >= 10 && binary.BigEndian.Uint32(p[6:10]) == uint32(tpm2.CmdLoad) {
		l.loads++
		resp := make([]byte, 10)
		binary.BigEndian.PutUint16(resp[0:2], uint16(tpm2.TagNoSessions))
		binary.BigEndian.PutUint32(resp[2:6], 10)
		binary.BigEndian.PutUint32(resp[6:10], l.rc)
		l.pending = resp
		return len(p), nil
	}
	return l.rw.Write(p)
}

func (l *loadAnswer) Read(p []byte) (int, error) {
	if l.pending != nil {
		n := copy(p, l.pending)
		l.pending = nil
		return n, nil
	}
	return l.rw.Read(p)
}

// Response codes as a TPM puts them on the wire.
const (
	rcIntegrityP1 = 0x080 | 0x040 | 0x01F | 0x100 // format 1, parameter 1
	rcBindingP1   = 0x080 | 0x040 | 0x025 | 0x100
	rcSensitive   = 0x100 | 0x055 // format 0
	rcSizeP1      = 0x080 | 0x040 | 0x015 | 0x100
	rcHandleH1    = 0x080 | 0x00B | 0x100 // format 1, handle 1
	rcFailure     = 0x100 | 0x001
	rcRetry       = 0x900 | 0x022 // warning
	rcObjectMem   = 0x900 | 0x002
)

// provisionEKCert writes a certificate for this simulator's EK into the RSA EK
// certificate NV index, as a vTPM's manufacturer does, and returns the roots
// that verify it.
func provisionEKCert(t *testing.T, sim io.ReadWriter) *x509.CertPool {
	t.Helper()
	ek, err := DefaultKeyFactory().CreateEK(sim)
	if err != nil {
		t.Fatalf("CreateEK: %v", err)
	}
	tpm2.FlushContext(sim, ek.Handle)
	ekKey, err := ek.Public.Key()
	if err != nil {
		t.Fatal(err)
	}
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ek-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	ekTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "test-ek"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment,
	}
	der, err := x509.CreateCertificate(rand.Reader, ekTmpl, caCert, ekKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	attrs := tpm2.AttrOwnerWrite | tpm2.AttrOwnerRead | tpm2.AttrAuthRead | tpm2.AttrPPRead
	if err := tpm2.NVDefineSpace(sim, tpm2.HandleOwner, EKRSACertificateHandle, "", "", nil, attrs, uint16(len(der))); err != nil {
		t.Fatalf("define EK certificate index: %v", err)
	}
	const chunk = 512
	for off := 0; off < len(der); off += chunk {
		end := min(off+chunk, len(der))
		if err := tpm2.NVWrite(sim, tpm2.HandleOwner, EKRSACertificateHandle, "", der[off:end], uint16(off)); err != nil {
			t.Fatalf("write EK certificate: %v", err)
		}
	}
	roots := x509.NewCertPool()
	roots.AddCert(caCert)
	return roots
}

// enrollServer is the metadata service as the guest sees it: instance-id with
// a session token, and enroll/start and enroll/finish through a real Enroller
// that issues for one VM ID. The MAC and EK header checks of the real handlers
// are left out; they are the host's and tested there.
type enrollServer struct {
	*httptest.Server
	mu     sync.Mutex
	starts []*tpm2.Public // DevID key of each enroll/start
}

func newEnrollServer(t *testing.T, roots *x509.CertPool, vmid string) *enrollServer {
	t.Helper()
	e := NewEnroller(mustParseTestCA(t), roots, nil)
	s := &enrollServer{}
	mux := http.NewServeMux()
	imds := &fakeIMDS{instanceID: "i-" + vmid}
	mux.Handle("/latest/api/token", imds)
	mux.Handle("/latest/meta-data/instance-id", imds)
	mux.HandleFunc("POST /latest/devid/enroll/start", func(w http.ResponseWriter, r *http.Request) {
		var req EnrollStartRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		data, _ := base64.StdEncoding.DecodeString(req.RequestB64)
		sig, _ := base64.StdEncoding.DecodeString(req.SignatureB64)
		var sr SigningRequest
		if err := sr.UnmarshalBinary(data); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		s.starts = append(s.starts, sr.DevIDKey)
		s.mu.Unlock()
		res, err := e.Start(data, sig, vmid, "")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(EnrollStartResponse{
			SessionID:         res.SessionID,
			CredentialBlobB64: base64.StdEncoding.EncodeToString(res.CredentialBlob),
			SecretB64:         base64.StdEncoding.EncodeToString(res.Secret),
		})
	})
	mux.HandleFunc("POST /latest/devid/enroll/finish", func(w http.ResponseWriter, r *http.Request) {
		var req EnrollFinishRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp, _ := base64.StdEncoding.DecodeString(req.ChallengeResponseB64)
		pemData, err := e.Finish(req.SessionID, resp, vmid, "", "")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(EnrollFinishResponse{DevIDCertPEM: string(pemData)})
	})
	s.Server = httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s
}

func (s *enrollServer) startCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.starts)
}

func (s *enrollServer) lastStartKey() *tpm2.Public {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.starts[len(s.starts)-1]
}

// writeBlobs writes a DevID key's blobs to dir.
func writeBlobs(t *testing.T, dir string, key *KeyInfo) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "devid.pub.blob"), key.PublicBlob, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "devid.priv.blob"), key.PrivateBlob, 0o600); err != nil {
		t.Fatal(err)
	}
}

// writtenCertKey returns the public key of the leaf in dir/devid.crt.pem.
func writtenCertKey(t *testing.T, dir string) any {
	t.Helper()
	cert, err := LoadCertPEMFile(filepath.Join(dir, "devid.crt.pem"))
	if err != nil || cert == nil {
		t.Fatalf("written certificate: %v", err)
	}
	return cert.PublicKey
}

func sameKey(t *testing.T, a, b tpm2.Public) bool {
	t.Helper()
	ka, err := a.Key()
	if err != nil {
		t.Fatal(err)
	}
	kb, err := b.Key()
	if err != nil {
		t.Fatal(err)
	}
	return ka.(*rsa.PublicKey).Equal(kb)
}

func fresh() (time.Time, time.Time) {
	now := time.Now()
	return now.Add(-time.Hour), now.Add(90 * 24 * time.Hour)
}

// Task 1.3: blobs made in another TPM plus a fresh certificate for them, as in
// a clone that got an empty vTPM, give a new key and a request; the materials
// that run writes then fit, and the next run makes no request. Nothing needs a
// fourth object slot, and nothing stays loaded.
func TestRenewWithBlobsFromAnotherTPM(t *testing.T) {
	f := DefaultKeyFactory()

	other, err := simulator.Get()
	if err != nil {
		t.Fatalf("start the other TPM simulator: %v", err)
	}
	foreign, err := f.CreateDevID(other)
	if err != nil {
		other.Close()
		t.Fatal(err)
	}
	other.Close()

	sim := newSim(t)
	srv := newEnrollServer(t, provisionEKCert(t, sim), "390")
	dir := t.TempDir()
	writeBlobs(t, dir, foreign)
	foreignKey, err := foreign.Public.Key()
	if err != nil {
		t.Fatal(err)
	}
	nb, na := fresh()
	writeTestCert(t, dir, foreignKey, "390", nb, na)
	logs := bufferLog(t)

	opts := ClientOptions{MDSBaseURL: srv.URL, OutDir: dir, Factory: f}
	if err := ClientEnrollOrRenew(sim, opts); err != nil {
		t.Fatalf("renewal with foreign blobs: %v", err)
	}
	if got := srv.startCount(); got != 1 {
		t.Fatalf("%d enroll requests, want 1", got)
	}
	if sameKey(t, *srv.lastStartKey(), foreign.Public) {
		t.Error("the request reused the foreign key")
	}
	if !strings.Contains(logs.String(), ": key does not load in this TPM") {
		t.Errorf("log does not name the reason: %q", logs.String())
	}
	pubBlob, _ := os.ReadFile(filepath.Join(dir, "devid.pub.blob"))
	if bytes.Equal(pubBlob, foreign.PublicBlob) {
		t.Error("devid.pub.blob still holds the foreign key")
	}
	newPub, err := tpm2.DecodePublic(pubBlob)
	if err != nil {
		t.Fatal(err)
	}
	if !certHoldsKey(&x509.Certificate{PublicKey: writtenCertKey(t, dir)}, newPub) {
		t.Error("the new certificate is not over the new key")
	}
	if got := transientHandles(t, sim); len(got) != 0 {
		t.Errorf("transient handles left after the renewal: %v", got)
	}

	// The written materials fit: the next run makes no request and changes
	// nothing.
	logs.Reset()
	before := snapshot(t, dir)
	if err := ClientEnrollOrRenew(sim, opts); err != nil {
		t.Fatalf("run with fitting materials: %v", err)
	}
	if got := srv.startCount(); got != 1 {
		t.Errorf("a run with fitting materials made a request (%d in all)", got)
	}
	assertUnchanged(t, dir, before)
	if strings.Contains(logs.String(), "renewing") {
		t.Errorf("a run with fitting materials logged a renewal: %q", logs.String())
	}
	if got := transientHandles(t, sim); len(got) != 0 {
		t.Errorf("transient handles left after a run that renewed nothing: %v", got)
	}
}

// Task 1.2 and 1.5: each reason, with blobs that load in this TPM. The loaded
// key is reused unless a blob is missing, the reason is logged once, and a
// fitting set makes no request.
func TestRenewReasonsWithLoadableBlobs(t *testing.T) {
	f := DefaultKeyFactory()
	sim := newSim(t)
	srv := newEnrollServer(t, provisionEKCert(t, sim), "390")

	key, err := f.CreateDevID(sim)
	if err != nil {
		t.Fatal(err)
	}
	FlushTransient(sim, key.Handle)
	blobKey, err := key.Public.Key()
	if err != nil {
		t.Fatal(err)
	}
	otherKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	nb, na := fresh()

	cases := []struct {
		name     string
		setup    func(t *testing.T, dir string)
		reason   string // empty: no renewal
		reuseKey bool
	}{
		{"fitting", func(t *testing.T, dir string) {
			writeTestCert(t, dir, blobKey, "390", nb, na)
		}, "", true},
		{"no certificate", func(t *testing.T, dir string) {}, "no certificate", true},
		{"unreadable certificate", func(t *testing.T, dir string) {
			_ = os.WriteFile(filepath.Join(dir, "devid.crt.pem"), []byte("garbage"), 0o600)
		}, "unreadable certificate", true},
		{"due", func(t *testing.T, dir string) {
			writeTestCert(t, dir, blobKey, "390", now.Add(-80*24*time.Hour), now.Add(10*24*time.Hour))
		}, "due", true},
		{"certificate is for another VM", func(t *testing.T, dir string) {
			writeTestCert(t, dir, blobKey, "399", nb, na)
		}, "certificate is for another VM", true},
		{"certificate does not match key", func(t *testing.T, dir string) {
			writeTestCert(t, dir, &otherKey.PublicKey, "390", nb, na)
		}, "certificate does not match key", true},
		{"missing blob", func(t *testing.T, dir string) {
			writeTestCert(t, dir, blobKey, "390", nb, na)
			_ = os.Remove(filepath.Join(dir, "devid.pub.blob"))
		}, "missing blob", false},
		{"empty blob", func(t *testing.T, dir string) {
			writeTestCert(t, dir, blobKey, "390", nb, na)
			_ = os.WriteFile(filepath.Join(dir, "devid.priv.blob"), nil, 0o600)
		}, "missing blob", false},
		// Two reasons: the first in the order of D-c is the one logged.
		{"other VM and another key", func(t *testing.T, dir string) {
			writeTestCert(t, dir, &otherKey.PublicKey, "399", nb, na)
		}, "certificate is for another VM", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeBlobs(t, dir, key)
			tc.setup(t, dir)
			before := snapshot(t, dir)
			starts := srv.startCount()
			logs := bufferLog(t)

			if err := ClientEnrollOrRenew(sim, ClientOptions{MDSBaseURL: srv.URL, OutDir: dir, Factory: f}); err != nil {
				t.Fatalf("run: %v", err)
			}
			out := logs.String()
			if got := transientHandles(t, sim); len(got) != 0 {
				t.Errorf("transient handles left: %v", got)
			}
			if tc.reason == "" {
				if srv.startCount() != starts {
					t.Error("fitting materials made an enroll request")
				}
				assertUnchanged(t, dir, before)
				if strings.Contains(out, "renewing") {
					t.Errorf("fitting materials logged a renewal: %q", out)
				}
				return
			}
			if srv.startCount() != starts+1 {
				t.Fatalf("%d enroll requests, want 1", srv.startCount()-starts)
			}
			if n := strings.Count(out, "renewing"); n != 1 || !strings.Contains(out, ": "+tc.reason) {
				t.Errorf("log %q: want one renewal line with reason %q", out, tc.reason)
			}
			after := snapshot(t, dir)
			if tc.reuseKey {
				if !sameKey(t, *srv.lastStartKey(), key.Public) {
					t.Error("the request did not reuse the loaded key")
				}
				if !bytes.Equal(after["devid.pub.blob"], key.PublicBlob) || !bytes.Equal(after["devid.priv.blob"], key.PrivateBlob) {
					t.Error("the blobs of the reused key changed")
				}
			} else if sameKey(t, *srv.lastStartKey(), key.Public) {
				t.Error("a missing blob reused the old key")
			}
			newPub, err := tpm2.DecodePublic(after["devid.pub.blob"])
			if err != nil {
				t.Fatal(err)
			}
			if !certHoldsKey(&x509.Certificate{PublicKey: writtenCertKey(t, dir)}, newPub) {
				t.Error("the written certificate is not over the written key")
			}
		})
	}
}

// Task 1.1: which TPM2_Load answers make the blobs foreign, through the real
// command path of go-tpm. A blob that does not decode is foreign without a TPM
// command; a load under another parent is foreign by the simulator's own
// answer.
func TestLoadDevIDForeignByResponseCode(t *testing.T) {
	f := DefaultKeyFactory()
	sim := newSim(t)
	key, err := f.CreateDevID(sim)
	if err != nil {
		t.Fatal(err)
	}
	FlushTransient(sim, key.Handle)

	cases := []struct {
		name    string
		rc      uint32
		foreign bool
	}{
		{"TPM_RC_INTEGRITY", rcIntegrityP1, true},
		{"TPM_RC_BINDING", rcBindingP1, true},
		{"TPM_RC_SENSITIVE", rcSensitive, true},
		{"TPM_RC_SIZE", rcSizeP1, false},
		{"TPM_RC_HANDLE", rcHandleH1, false},
		{"TPM_RC_FAILURE", rcFailure, false},
		{"TPM_RC_RETRY", rcRetry, false},
		{"TPM_RC_OBJECT_MEMORY", rcObjectMem, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rw := &loadAnswer{rw: sim, rc: tc.rc}
			got, err := f.LoadDevID(rw, key.PublicBlob, key.PrivateBlob)
			if err == nil {
				FlushTransient(sim, got.Handle)
				t.Fatal("load succeeded through an injected error")
			}
			if rw.loads == 0 {
				t.Fatal("TPM2_Load was never sent")
			}
			if errors.Is(err, ErrDevIDForeign) != tc.foreign {
				t.Errorf("errors.Is(%v, ErrDevIDForeign) = %v, want %v", err, !tc.foreign, tc.foreign)
			}
			if !tc.foreign && !errors.Is(err, ErrDevIDLoad) {
				t.Errorf("%v is neither foreign nor ErrDevIDLoad", err)
			}
			if got := transientHandles(t, sim); len(got) != 0 {
				t.Errorf("transient handles left: %v", got)
			}
		})
	}

	t.Run("public blob does not decode", func(t *testing.T) {
		rw := &loadAnswer{rw: sim}
		_, err := f.LoadDevID(rw, []byte{0x00, 0x01, 0x02}, key.PrivateBlob)
		if !errors.Is(err, ErrDevIDForeign) {
			t.Errorf("err = %v, want ErrDevIDForeign", err)
		}
		if rw.cmds != 0 {
			t.Errorf("%d TPM commands for a blob that does not decode", rw.cmds)
		}
	})

	t.Run("another parent", func(t *testing.T) {
		otherParent := *f
		otherParent.SRKTemplate = uniqueTemplate(256, 0x01)
		_, err := otherParent.LoadDevID(sim, key.PublicBlob, key.PrivateBlob)
		if !errors.Is(err, ErrDevIDForeign) {
			t.Errorf("err = %v, want ErrDevIDForeign", err)
		}
		t.Logf("the simulator's answer to a load under another parent: %v", err)
	})

	t.Run("loads", func(t *testing.T) {
		got, err := f.LoadDevID(sim, key.PublicBlob, key.PrivateBlob)
		if err != nil {
			t.Fatal(err)
		}
		FlushTransient(sim, got.Handle)
	})
}

// Task 1.4: a TPM warning on the load fails the run, makes no request and
// leaves the materials unchanged, also when a renewal would otherwise be due.
func TestRenewTPMWarningChangesNothing(t *testing.T) {
	f := DefaultKeyFactory()
	sim := newSim(t)
	srv := newEnrollServer(t, provisionEKCert(t, sim), "390")
	key, err := f.CreateDevID(sim)
	if err != nil {
		t.Fatal(err)
	}
	FlushTransient(sim, key.Handle)
	blobKey, err := key.Public.Key()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	nb, na := fresh()

	for name, valid := range map[string][2]time.Time{
		"fresh": {nb, na},
		"due":   {now.Add(-80 * 24 * time.Hour), now.Add(10 * 24 * time.Hour)},
	} {
		for _, rc := range []uint32{rcRetry, rcObjectMem} {
			t.Run(fmt.Sprintf("%s/rc %#x", name, rc), func(t *testing.T) {
				dir := t.TempDir()
				writeBlobs(t, dir, key)
				writeTestCert(t, dir, blobKey, "390", valid[0], valid[1])
				before := snapshot(t, dir)
				logs := bufferLog(t)

				rw := &loadAnswer{rw: sim, rc: rc}
				err := ClientEnrollOrRenew(rw, ClientOptions{MDSBaseURL: srv.URL, OutDir: dir, Factory: f})
				if err == nil {
					t.Fatalf("rc %#x: the run succeeded", rc)
				}
				if errors.Is(err, ErrDevIDForeign) {
					t.Errorf("rc %#x read as foreign: %v", rc, err)
				}
				if srv.startCount() != 0 {
					t.Errorf("rc %#x: an enroll request was made", rc)
				}
				assertUnchanged(t, dir, before)
				if strings.Contains(logs.String(), "renewing") {
					t.Errorf("rc %#x: a run that changed nothing logged a renewal: %q", rc, logs.String())
				}
				if got := transientHandles(t, sim); len(got) != 0 {
					t.Errorf("transient handles left: %v", got)
				}
			})
		}
	}
}
