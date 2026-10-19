package devid

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dembaca/vtpm-mds/internal/inventory" // pragma: allowlist secret
)

func TestCreateStoredPinExclusiveRace(t *testing.T) {
	dir := t.TempDir()
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- createStoredPinExclusive(dir, "100", "abc")
		}()
	}
	wg.Wait()
	close(errs)
	var created, exists int
	for err := range errs {
		if err == nil {
			created++
		} else if errors.Is(err, errAlreadyPinned) {
			exists++
		} else {
			t.Fatalf("unexpected: %v", err)
		}
	}
	if created != 1 || exists != 7 {
		t.Fatalf("created=%d exists=%d", created, exists)
	}
	got, err := readStoredPin(dir, "100")
	if err != nil || got != "abc" {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestAuthenticateStoredPin(t *testing.T) {
	cert, key, roots := testEKMaterial(t)
	other, _, _ := testEKMaterial(t)
	dir := t.TempDir()
	enroller := NewEnroller(&CA{}, roots, nil)
	enroller.PinDir = dir
	ekPub := rsaKeyToEKPub(key)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(HeaderEKCert, EncodeEKCertHeader(cert))
	vm := &inventory.VMConfig{VMID: "100"}
	req = req.WithContext(context.WithValue(req.Context(), inventory.VMConfigContextKey, vm))

	if _, _, err := enroller.AuthenticateEnrollCaller(req, cert, ekPub, ""); err != nil {
		t.Fatalf("no pin yet: %v", err)
	}
	if err := createStoredPinExclusive(dir, "100", EKFingerprint(other)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := enroller.AuthenticateEnrollCaller(req, cert, ekPub, ""); !isUnauthorized(err) || err.Error() != errStoredPinMismatch {
		t.Fatalf("want stored pin mismatch, got %v", err)
	}
	vm.EKSHA256 = EKFingerprint(cert)
	if _, _, err := enroller.AuthenticateEnrollCaller(req, cert, ekPub, ""); err != nil {
		t.Fatalf("inventory wins: %v", err)
	}
}

func TestEnsurePinBeforeIssueUnwritable(t *testing.T) {
	fileAsDir := filepath.Join(t.TempDir(), "notadir")
	if err := os.WriteFile(fileAsDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := &Enroller{PinDir: fileAsDir}
	err := e.ensurePinBeforeIssue("100", "abc")
	if err == nil || !errors.Is(err, ErrPinStore) {
		t.Fatalf("want ErrPinStore, got %v", err)
	}
}

func TestEnsurePinResetAllowsNewEK(t *testing.T) {
	dir := t.TempDir()
	e := &Enroller{PinDir: dir}
	if err := e.ensurePinBeforeIssue("100", "aaa"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(pinPath(dir, "100")); err != nil {
		t.Fatal(err)
	}
	if err := e.ensurePinBeforeIssue("100", "bbb"); err != nil {
		t.Fatal(err)
	}
	got, _ := readStoredPin(dir, "100")
	if got != "bbb" {
		t.Fatalf("got %q", got)
	}
}

func TestReadStoredPinEmptyPresent(t *testing.T) {
	dir := t.TempDir()
	got, err := readStoredPin(dir, "100")
	if err != nil || got != "" {
		t.Fatalf("missing: got %q err=%v", got, err)
	}
	if err := os.WriteFile(pinPath(dir, "100"), []byte("  AbC\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	got, err = readStoredPin(dir, "100")
	if err != nil || got != "abc" {
		t.Fatalf("present: got %q err=%v", got, err)
	}
}

func TestValidateVMIDRejectsPathEscape(t *testing.T) {
	for _, v := range []string{"", "..", "../x", "a/b", "a\\b", ".hidden", "x/../y"} {
		if err := validateVMID(v); err == nil || !errors.Is(err, ErrPinStore) {
			t.Fatalf("vmid %q: want ErrPinStore, got %v", v, err)
		}
	}
	if err := validateVMID("100"); err != nil {
		t.Fatal(err)
	}
}

func TestFinishWritesPinBeforeIssue(t *testing.T) {
	ca := mustParseTestCA(t)
	sr := mustMinimalSigningRequest(t, pkix.Name{})
	dir := t.TempDir()
	enroller := NewEnroller(ca, x509.NewCertPool(), NewSessionStore(time.Minute))
	enroller.PinDir = dir
	sid, err := enroller.Sessions.Put([]byte("nonce"), *sr, "100", "ekfp")
	if err != nil {
		t.Fatal(err)
	}
	pemBytes, err := enroller.Finish(sid, []byte("nonce"), "100", "ekfp", "")
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if block, _ := pem.Decode(pemBytes); block == nil {
		t.Fatal("expected PEM")
	}
	got, err := readStoredPin(dir, "100")
	if err != nil || got != "ekfp" {
		t.Fatalf("pin=%q err=%v", got, err)
	}
	sid2, err := enroller.Sessions.Put([]byte("nonce2"), *sr, "101", "other")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := enroller.Finish(sid2, []byte("nonce2"), "101", "other", "inventory-pin"); err != nil {
		t.Fatal(err)
	}
	got, err = readStoredPin(dir, "101")
	if err != nil || got != "" {
		t.Fatalf("inventory pin must not write store: %q err=%v", got, err)
	}
}

func TestFinishUnwritablePinDirNoPEM(t *testing.T) {
	ca := mustParseTestCA(t)
	sr := mustMinimalSigningRequest(t, pkix.Name{})
	fileAsDir := filepath.Join(t.TempDir(), "notadir")
	if err := os.WriteFile(fileAsDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	enroller := NewEnroller(ca, x509.NewCertPool(), NewSessionStore(time.Minute))
	enroller.PinDir = fileAsDir
	sid, err := enroller.Sessions.Put([]byte("nonce"), *sr, "100", "ekfp")
	if err != nil {
		t.Fatal(err)
	}
	pemBytes, err := enroller.Finish(sid, []byte("nonce"), "100", "ekfp", "")
	if err == nil || !errors.Is(err, ErrPinStore) {
		t.Fatalf("want ErrPinStore, got pem=%v err=%v", pemBytes != nil, err)
	}
	if pemBytes != nil {
		t.Fatal("must not return PEM when pin store fails")
	}
}

func TestConcurrentFinishSameEKBothGetCert(t *testing.T) {
	ca := mustParseTestCA(t)
	dir := t.TempDir()
	enroller := NewEnroller(ca, x509.NewCertPool(), NewSessionStore(time.Minute))
	enroller.PinDir = dir

	type result struct {
		pem []byte
		err error
	}
	out := make(chan result, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sr := mustMinimalSigningRequest(t, pkix.Name{})
			nonce := []byte{byte('a' + i)}
			sid, err := enroller.Sessions.Put(nonce, *sr, "100", "same-ek")
			if err != nil {
				out <- result{err: err}
				return
			}
			pemBytes, err := enroller.Finish(sid, nonce, "100", "same-ek", "")
			out <- result{pem: pemBytes, err: err}
		}(i)
	}
	wg.Wait()
	close(out)
	var ok int
	for r := range out {
		if r.err != nil {
			t.Fatalf("Finish: %v", r.err)
		}
		if block, _ := pem.Decode(r.pem); block == nil {
			t.Fatal("expected PEM")
		}
		ok++
	}
	if ok != 2 {
		t.Fatalf("got %d certs", ok)
	}
	got, err := readStoredPin(dir, "100")
	if err != nil || got != "same-ek" {
		t.Fatalf("pin=%q err=%v", got, err)
	}
}

func TestSecondFinishDoesNotOverwritePin(t *testing.T) {
	ca := mustParseTestCA(t)
	dir := t.TempDir()
	enroller := NewEnroller(ca, x509.NewCertPool(), NewSessionStore(time.Minute))
	enroller.PinDir = dir
	sr := mustMinimalSigningRequest(t, pkix.Name{})
	sid, err := enroller.Sessions.Put([]byte("n1"), *sr, "100", "first")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := enroller.Finish(sid, []byte("n1"), "100", "first", ""); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(pinPath(dir, "100"))
	sid2, err := enroller.Sessions.Put([]byte("n2"), *sr, "100", "first")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := enroller.Finish(sid2, []byte("n2"), "100", "first", ""); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(pinPath(dir, "100"))
	if string(before) != string(after) {
		t.Fatalf("pin overwritten: %q -> %q", before, after)
	}
}

func TestPinWrittenWhenIssueDevIDFails(t *testing.T) {
	caCert, _ := mustTestCA(t)
	wrongKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw})
	keyDER, err := x509.MarshalPKCS8PrivateKey(wrongKey)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	badCA, err := ParseCA(caPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	enroller := NewEnroller(badCA, x509.NewCertPool(), NewSessionStore(time.Minute))
	enroller.PinDir = dir
	sr := mustMinimalSigningRequest(t, pkix.Name{})
	sid, err := enroller.Sessions.Put([]byte("nonce"), *sr, "100", "ekfp")
	if err != nil {
		t.Fatal(err)
	}
	pemBytes, err := enroller.Finish(sid, []byte("nonce"), "100", "ekfp", "")
	if err == nil {
		t.Fatal("expected IssueDevID failure")
	}
	if pemBytes != nil {
		t.Fatal("must not return PEM")
	}
	got, rerr := readStoredPin(dir, "100")
	if rerr != nil || got != "ekfp" {
		t.Fatalf("pin should exist before sign: got %q err=%v", got, rerr)
	}
}

func TestEmptyPinFileMeansNoPin(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(pinPath(dir, "100"), []byte("\n\t  "), 0o640); err != nil {
		t.Fatal(err)
	}
	got, err := readStoredPin(dir, "100")
	if err != nil || got != "" {
		t.Fatalf("empty/whitespace pin must mean no pin: %q err=%v", got, err)
	}
	e := &Enroller{PinDir: dir}
	if err := e.ensurePinBeforeIssue("100", "newfp"); err != nil {
		t.Fatal(err)
	}
	got, _ = readStoredPin(dir, "100")
	if got != "newfp" {
		t.Fatalf("got %q", got)
	}
}
