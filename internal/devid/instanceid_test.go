package devid

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"log"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-tpm/legacy/tpm2"
)

// fakeIMDS serves the two requests the client makes for instance-id, as the
// service does: PUT /latest/api/token, then GET instance-id with that token.
type fakeIMDS struct {
	mu          sync.Mutex
	instanceID  string // answer to instance-id
	idStatus    int    // non-zero: instance-id answers this status instead
	tokenStatus int    // non-zero: the token request answers this status
	tokens      int
	reads       int
	other       []string // any other request
}

const fakeIMDSToken = "tok-123"

func (f *fakeIMDS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.Method == http.MethodPut && r.URL.Path == "/latest/api/token":
		f.tokens++
		if r.Header.Get("X-aws-ec2-metadata-token-ttl-seconds") == "" {
			http.Error(w, "missing TTL", http.StatusBadRequest)
			return
		}
		if f.tokenStatus != 0 {
			http.Error(w, "token refused", f.tokenStatus)
			return
		}
		_, _ = w.Write([]byte(fakeIMDSToken))
	case r.Method == http.MethodGet && r.URL.Path == "/latest/meta-data/instance-id":
		f.reads++
		if r.Header.Get("X-Aws-Ec2-Metadata-Token") != fakeIMDSToken {
			w.Header().Set("X-Aws-Ec2-Metadata-Token", "required")
			http.Error(w, "IMDSv2 token required", http.StatusUnauthorized)
			return
		}
		if f.idStatus != 0 {
			http.Error(w, "404 page not found", f.idStatus)
			return
		}
		_, _ = w.Write([]byte(f.instanceID))
	default:
		f.other = append(f.other, r.Method+" "+r.URL.Path)
		http.Error(w, "unexpected", http.StatusTeapot)
	}
}

func (f *fakeIMDS) counts() (tokens, reads int, other []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tokens, f.reads, append([]string(nil), f.other...)
}

// noInstanceIDSleep removes the pause between tries and counts the pauses.
func noInstanceIDSleep(t *testing.T) *int {
	t.Helper()
	n := new(int)
	old := instanceIDSleep
	instanceIDSleep = func() { *n++ }
	t.Cleanup(func() { instanceIDSleep = old })
	return n
}

// Task 1.3a: instance-id is read with a session token, tried three times, and
// only an answer with the i- prefix names a VM.
func TestReadVMID(t *testing.T) {
	cases := []struct {
		name       string
		imds       *fakeIMDS
		want       string
		wantErr    string
		wantReads  int
		wantTokens int
		wantSleeps int
	}{
		{name: "same VM", imds: &fakeIMDS{instanceID: "i-390"}, want: "390", wantReads: 1, wantTokens: 1},
		{name: "404", imds: &fakeIMDS{idStatus: http.StatusNotFound}, wantErr: "HTTP 404", wantReads: 3, wantTokens: 3, wantSleeps: 2},
		{name: "no i- prefix", imds: &fakeIMDS{instanceID: "vm-390"}, wantErr: "no i- prefix", wantReads: 1, wantTokens: 1},
		{name: "i- alone", imds: &fakeIMDS{instanceID: "i-"}, wantErr: "no i- prefix", wantReads: 1, wantTokens: 1},
		{name: "token refused", imds: &fakeIMDS{instanceID: "i-390", tokenStatus: http.StatusForbidden}, wantErr: "session token", wantTokens: 3, wantSleeps: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sleeps := noInstanceIDSleep(t)
			imds := tc.imds
			srv := httptest.NewServer(imds)
			defer srv.Close()

			got, err := ReadVMID(srv.URL)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want one containing %q", err, tc.wantErr)
				}
			} else if err != nil || got != tc.want {
				t.Fatalf("ReadVMID = %q, %v; want %q", got, err, tc.want)
			}
			tokens, reads, other := imds.counts()
			if tokens != tc.wantTokens || reads != tc.wantReads || len(other) != 0 {
				t.Errorf("token requests %d, instance-id reads %d, other %v; want %d, %d, none",
					tokens, reads, other, tc.wantTokens, tc.wantReads)
			}
			if *sleeps != tc.wantSleeps {
				t.Errorf("%d pauses between tries, want %d", *sleeps, tc.wantSleeps)
			}
		})
	}
}

// failingTPM refuses every command and counts them.
type failingTPM struct{ writes int }

func (f *failingTPM) Write(p []byte) (int, error) {
	f.writes++
	return 0, errors.New("no TPM in this test")
}

func (f *failingTPM) Read(p []byte) (int, error) { return 0, errors.New("no TPM in this test") }

// bufferLog sends the standard logger to a buffer for the test.
func bufferLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(old) })
	return &buf
}

// writeTestCert writes a certificate PEM for key with subject CN=cn, valid
// from notBefore to notAfter, to dir/devid.crt.pem. The client does not
// verify the issuer, so a throwaway CA signs it.
func writeTestCert(t *testing.T, dir string, key any, cn string, notBefore, notAfter time.Time) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ca := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-devid-ca"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, leaf, ca, key, caKey)
	if err != nil {
		t.Fatal(err)
	}
	pemData := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(filepath.Join(dir, "devid.crt.pem"), pemData, 0o600); err != nil {
		t.Fatal(err)
	}
}

// snapshot returns the contents of the three materials in dir.
func snapshot(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for _, name := range []string{"devid.crt.pem", "devid.priv.blob", "devid.pub.blob"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		out[name] = data
	}
	return out
}

func assertUnchanged(t *testing.T, dir string, before map[string][]byte) {
	t.Helper()
	for name, data := range snapshot(t, dir) {
		if !bytes.Equal(data, before[name]) {
			t.Errorf("%s changed", name)
		}
	}
}

// Task 1.3a: without a certificate there is no CN to compare, so the client
// makes no metadata request and goes straight to the TPM.
func TestNoCertificateMakesNoMetadataRequest(t *testing.T) {
	noInstanceIDSleep(t)
	logs := bufferLog(t)
	imds := &fakeIMDS{instanceID: "i-390"}
	srv := httptest.NewServer(imds)
	defer srv.Close()

	tpm := &failingTPM{}
	err := ClientEnrollOrRenew(tpm, ClientOptions{MDSBaseURL: srv.URL, OutDir: t.TempDir()})
	if err == nil {
		t.Fatal("expected the run to fail without a TPM")
	}
	if tokens, reads, other := imds.counts(); tokens+reads+len(other) != 0 {
		t.Errorf("metadata requests without a certificate: %d token, %d instance-id, %v", tokens, reads, other)
	}
	if tpm.writes == 0 {
		t.Error("the run did not reach the TPM")
	}
	if !strings.Contains(logs.String(), "renewing") || !strings.Contains(logs.String(), ": no certificate") {
		t.Errorf("log does not name the reason: %q", logs.String())
	}
}

// Task 1.3a: a fresh certificate and an instance-id that cannot be read fail
// the run before the TPM is touched, with no enroll request and the files as
// they were.
func TestUnreadableInstanceIDChangesNothing(t *testing.T) {
	cases := map[string]*fakeIMDS{
		"404":           {idStatus: http.StatusNotFound},
		"no i- prefix":  {instanceID: "390"},
		"token refused": {instanceID: "i-390", tokenStatus: http.StatusForbidden},
	}
	for name, imds := range cases {
		t.Run(name, func(t *testing.T) {
			noInstanceIDSleep(t)
			logs := bufferLog(t)
			srv := httptest.NewServer(imds)
			defer srv.Close()

			dir := t.TempDir()
			key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now()
			writeTestCert(t, dir, &key.PublicKey, "390", now.Add(-time.Hour), now.Add(90*24*time.Hour))
			for _, name := range []string{"devid.priv.blob", "devid.pub.blob"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte("blob"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			before := snapshot(t, dir)

			tpm := &failingTPM{}
			if err := ClientEnrollOrRenew(tpm, ClientOptions{MDSBaseURL: srv.URL, OutDir: dir}); err == nil {
				t.Fatal("expected the run to fail")
			}
			if tpm.writes != 0 {
				t.Errorf("%d TPM commands sent before instance-id was known", tpm.writes)
			}
			if _, _, other := imds.counts(); len(other) != 0 {
				t.Errorf("unexpected requests: %v", other)
			}
			if strings.Contains(logs.String(), "renewing") {
				t.Errorf("a run that changes nothing logged a renewal: %q", logs.String())
			}
			assertUnchanged(t, dir, before)
		})
	}
}

// Task 1.1: which TPM2_Load answers count as foreign, as values (what go-tpm's
// legacy package returns) and as pointers.
func TestIsForeignLoadError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"integrity", tpm2.ParameterError{Code: tpm2.RCIntegrity, Parameter: tpm2.RC2}, true},
		{"binding", tpm2.ParameterError{Code: tpm2.RCBinding, Parameter: tpm2.RC1}, true},
		{"sensitive", tpm2.Error{Code: tpm2.RCSensitive}, true},
		{"integrity pointer", &tpm2.ParameterError{Code: tpm2.RCIntegrity, Parameter: tpm2.RC2}, true},
		{"sensitive pointer", &tpm2.Error{Code: tpm2.RCSensitive}, true},
		{"wrapped integrity", errors.Join(errors.New("load"), tpm2.ParameterError{Code: tpm2.RCIntegrity}), true},
		{"size", tpm2.ParameterError{Code: tpm2.RCSize, Parameter: tpm2.RC1}, false},
		{"integrity on a handle", tpm2.HandleError{Code: tpm2.RCIntegrity, Handle: tpm2.RC1}, false},
		{"retry", tpm2.Warning{Code: tpm2.RCRetry}, false},
		{"object memory", tpm2.Warning{Code: tpm2.RCObjectMemory}, false},
		{"failure", tpm2.Error{Code: tpm2.RCFailure}, false},
		{"transport", errors.New("write /dev/tpmrm0: broken pipe"), false},
	}
	for _, tc := range cases {
		if got := isForeignLoadError(tc.err); got != tc.want {
			t.Errorf("%s: isForeignLoadError(%v) = %v, want %v", tc.name, tc.err, got, tc.want)
		}
	}
}

// Task 1.3a: one try, token and read together, is cut off after the per-try
// limit, so three tries stay near ten seconds.
func TestReadVMIDTryTimesOut(t *testing.T) {
	sleeps := noInstanceIDSleep(t)
	old := instanceIDTimeout
	instanceIDTimeout = 50 * time.Millisecond
	t.Cleanup(func() { instanceIDTimeout = old })
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer srv.Close()
	defer close(release)

	start := time.Now()
	if _, err := ReadVMID(srv.URL); err == nil {
		t.Fatal("ReadVMID succeeded against a service that never answers")
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("three tries took %v", d)
	}
	if *sleeps != 2 {
		t.Errorf("%d pauses between tries, want 2", *sleeps)
	}
}
