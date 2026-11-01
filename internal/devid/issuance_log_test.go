package devid

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dembaca/vtpm-mds/internal/inventory"
)

// enrollThroughHandlers runs start and finish against the real handlers and
// returns the finish response and what both legs logged. The nonce the guest
// would recover with credential activation is read from the session store, so the
// finish can be made to succeed or to fail on purpose.
func enrollThroughHandlers(t *testing.T, wrongResponse bool) (finish *httptest.ResponseRecorder, logs string, ekFP string) {
	t.Helper()
	ekCert, ekKey, roots := testEKMaterial(t)
	requestData, sig, _ := fakeSignedEnrollRequest(t, ekCert, rsaKeyToEKPub(ekKey))
	store := NewSessionStore(time.Minute)
	enroller := NewEnroller(mustTestDevIDCA(t), roots, store)

	var start *httptest.ResponseRecorder
	logs = captureLog(t, func() {
		start = postEnrollStart(t, enroller, ekCert, requestData, sig)
	})
	if start.Code != http.StatusOK {
		t.Fatalf("start: expected 200, got %d body=%q", start.Code, start.Body.String())
	}
	var sr EnrollStartResponse
	if err := json.Unmarshal(start.Body.Bytes(), &sr); err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	sess := store.sessions[sr.SessionID]
	store.mu.Unlock()
	if sess == nil {
		t.Fatal("no session after start")
	}
	response := append([]byte(nil), sess.Nonce...)
	if wrongResponse {
		response[0] ^= 0xff
	}
	body, err := json.Marshal(EnrollFinishRequest{
		SessionID:            sr.SessionID,
		ChallengeResponseB64: base64.StdEncoding.EncodeToString(response),
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/latest/devid/enroll/finish", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderEKCert, EncodeEKCertHeader(ekCert))
	vm := &inventory.VMConfig{VMID: "100", MACs: []string{"52:54:00:a1:b2:c3"}}
	req = req.WithContext(context.WithValue(req.Context(), inventory.VMConfigContextKey, vm))
	finish = httptest.NewRecorder()
	logs += captureLog(t, func() { enroller.HandleFinish(finish, req) })
	return finish, logs, EKFingerprint(ekCert)
}

func issuanceLines(logs string) []string {
	var out []string
	for _, l := range strings.Split(logs, "\n") {
		if strings.Contains(l, "devid issued") {
			out = append(out, l)
		}
	}
	return out
}

// A successful finish writes exactly one issuance line with the certificate's
// serial, the VM, the EK fingerprint and the SHA-1 of the leaf DER, and nothing
// secret.
func TestASuccessfulFinishIsLoggedOnce(t *testing.T) {
	finish, logs, ekFP := enrollThroughHandlers(t, false)
	if finish.Code != http.StatusOK {
		t.Fatalf("finish: expected 200, got %d body=%q", finish.Code, finish.Body.String())
	}
	lines := issuanceLines(logs)
	if len(lines) != 1 {
		t.Fatalf("expected one issuance line, got %d: %q", len(lines), logs)
	}
	var resp EnrollFinishResponse
	if err := json.Unmarshal(finish.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode([]byte(resp.DevIDCertPEM))
	if block == nil {
		t.Fatal("the response holds no certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	line := lines[0]
	sum := sha1.Sum(block.Bytes)
	leaf := "leaf=" + hex.EncodeToString(sum[:])
	for _, want := range []string{"serial=" + cert.SerialNumber.Text(16), "vm=100", "ek=" + ekFP, leaf} {
		if !strings.Contains(line, want) {
			t.Errorf("the line lacks %q: %q", want, line)
		}
	}
	for _, leak := range []string{"BEGIN CERTIFICATE", "session", resp.DevIDCertPEM[:40]} {
		if strings.Contains(logs, leak) {
			t.Errorf("the log holds %q: %q", leak, logs)
		}
	}
}

// A failed finish writes no issuance line, and answers as before.
func TestAFailedFinishIsNotLoggedAsAnIssuance(t *testing.T) {
	finish, logs, _ := enrollThroughHandlers(t, true)
	if finish.Code != http.StatusBadRequest {
		t.Fatalf("finish: expected 400, got %d body=%q", finish.Code, finish.Body.String())
	}
	if lines := issuanceLines(logs); len(lines) != 0 {
		t.Fatalf("a refused finish was logged as an issuance: %q", lines)
	}
	if !strings.Contains(logs, "devid enroll/finish:") {
		t.Errorf("the refusal must still be logged, got %q", logs)
	}
}
