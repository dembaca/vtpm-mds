package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dembaca/vtpm-mds/internal/config"
	"github.com/dembaca/vtpm-mds/internal/inventory"
)

// signatureTestServer builds the server exactly as main does, so the routes
// under test are the ones the daemon registers rather than a test double's.
// The YAML inventory keeps New from reading /etc/pve on the test host.
func signatureTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.MDS.ListenAddr = "127.0.0.1:0"
	cfg.MDS.EnableTPMAttestation = false
	cfg.MDS.InventoryPath = "../../testdata/vm-inventory/lab.yaml"

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New returned an error: %v", err)
	}
	token, err := srv.tokenStore.GenerateToken("test")
	if err != nil {
		t.Fatalf("failed to generate a session token: %v", err)
	}
	return srv, token.Token
}

type capturedResponse struct {
	status      int
	contentType string
	body        string
}

func serve(srv *Server, req *http.Request) capturedResponse {
	rec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(rec, req)
	return capturedResponse{rec.Code, rec.Header().Get("Content-Type"), rec.Body.String()}
}

// TestInstanceIdentitySignaturePathsAreNotServed verifies that no signature
// over the identity document is served: .../signature answers exactly what an
// unregistered path answers — status, Content-Type and body — for a bound
// caller with a token, an unbound caller with a token and a caller with no
// token, and so do the two AWS signature paths this service never served
// (remove-the-constant-instance-identity-signature, task 2.3).
func TestInstanceIdentitySignaturePathsAreNotServed(t *testing.T) {
	srv, token := signatureTestServer(t)

	callers := []struct {
		name string
		req  func(path string) *http.Request
	}{
		{"bound caller with a token", func(path string) *http.Request {
			req := httptest.NewRequest("GET", path, nil)
			req.RemoteAddr = "10.0.0.5:40000"
			req.Header.Set("X-Aws-Ec2-Metadata-Token", token)
			return req.WithContext(context.WithValue(req.Context(),
				inventory.VMConfigContextKey, &inventory.VMConfig{VMID: "100"}))
		}},
		{"unbound caller with a token", func(path string) *http.Request {
			req := httptest.NewRequest("GET", path, nil)
			req.RemoteAddr = "10.0.0.6:40000"
			req.Header.Set("X-Aws-Ec2-Metadata-Token", token)
			return req
		}},
		{"caller with no token", func(path string) *http.Request {
			req := httptest.NewRequest("GET", path, nil)
			req.RemoteAddr = "10.0.0.5:40000"
			return req
		}},
	}

	for _, c := range callers {
		want := serve(srv, c.req("/latest/no-such-thing"))
		if want.status != http.StatusNotFound {
			t.Fatalf("%s: the catch-all answered %d, expected 404", c.name, want.status)
		}
		for _, path := range []string{
			"/latest/dynamic/instance-identity/signature",
			"/latest/dynamic/instance-identity/pkcs7",
			"/latest/dynamic/instance-identity/rsa2048",
		} {
			if got := serve(srv, c.req(path)); got != want {
				t.Errorf("%s: GET %s = %+v, want the catch-all's %+v (is a signature route registered again?)",
					c.name, path, got, want)
			}
		}
	}
}

// TestInstanceIdentityDocumentIsStillServed guards the non-goal: removing the
// signature must leave the document's route registered and served to a bound
// caller, so the test above cannot pass merely because the whole dynamic tree
// went missing.
func TestInstanceIdentityDocumentIsStillServed(t *testing.T) {
	srv, token := signatureTestServer(t)

	req := httptest.NewRequest("GET", "/latest/dynamic/instance-identity/document", nil)
	req.RemoteAddr = "10.0.0.5:40000"
	req.Header.Set("X-Aws-Ec2-Metadata-Token", token)
	req = req.WithContext(context.WithValue(req.Context(),
		inventory.VMConfigContextKey, &inventory.VMConfig{VMID: "100"}))

	got := serve(srv, req)
	if got.status != http.StatusOK || got.contentType != "application/json" {
		t.Fatalf("document: got %d %q, want 200 application/json", got.status, got.contentType)
	}
}
