package server

import (
	"io"
	"net"
	"strconv"
	"time"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGuestLimitExemptsHealth(t *testing.T) {
	l := newGuestLimiter()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"status":"ok"}`)
	})
	h := guestLimitMiddleware(l, inner)

	// Exhaust request budget.
	for i := 0; i < maxRequestsPerPeer; i++ {
		req := httptest.NewRequest(http.MethodGet, "/latest/meta-data/", nil)
		req.RemoteAddr = "10.0.0.5:1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("setup request %d: %d", i, rec.Code)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/latest/meta-data/", nil)
	req.RemoteAddr = "10.0.0.5:1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429 after flood, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != "Too many requests\n" {
		t.Fatalf("body %q", body)
	}
	if rec.Header().Get("Retry-After") != "1" {
		t.Fatalf("Retry-After %q", rec.Header().Get("Retry-After"))
	}

	health := httptest.NewRequest(http.MethodGet, "/health", nil)
	health.RemoteAddr = "10.0.0.5:1234"
	hrec := httptest.NewRecorder()
	h.ServeHTTP(hrec, health)
	if hrec.Code != http.StatusOK {
		t.Fatalf("health want 200, got %d", hrec.Code)
	}
}

func TestGuestLimitTokenPUTBudget(t *testing.T) {
	l := newGuestLimiter()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := guestLimitMiddleware(l, inner)

	for i := 0; i < maxTokenPUTsPerPeer; i++ {
		req := httptest.NewRequest(http.MethodPut, "/latest/api/token", nil)
		req.RemoteAddr = "10.0.0.8:9"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("token %d: %d", i, rec.Code)
		}
	}
	req := httptest.NewRequest(http.MethodPut, "/latest/api/token", nil)
	req.RemoteAddr = "10.0.0.8:9"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429, got %d body %q", rec.Code, rec.Body.String())
	}

	// Other peer unaffected.
	other := httptest.NewRequest(http.MethodPut, "/latest/api/token", nil)
	other.RemoteAddr = "10.0.0.9:9"
	orec := httptest.NewRecorder()
	h.ServeHTTP(orec, other)
	if orec.Code != http.StatusOK {
		t.Fatalf("peer B want 200, got %d", orec.Code)
	}
}

func TestGuestLimitConnCap(t *testing.T) {
	l := newGuestLimiter()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := guestLimitMiddleware(l, inner)
	peer := "198.51.100.9:1"
	// Simulate maxConnsPerPeer+1 accepted connections.
	for i := 0; i < maxConnsPerPeer+1; i++ {
		l.trackConnState(&stubConn{addr: peer}, http.StateNew)
	}
	req := httptest.NewRequest(http.MethodGet, "/latest/meta-data/", nil)
	req.RemoteAddr = peer
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429 over conn cap, got %d", rec.Code)
	}
	// Close one; under cap again for HTTP (still need request budget).
	l.trackConnState(&stubConn{addr: peer}, http.StateClosed)
	// New peer for a clean request budget check of conn alone:
	l2 := newGuestLimiter()
	h2 := guestLimitMiddleware(l2, inner)
	for i := 0; i < maxConnsPerPeer; i++ {
		l2.trackConnState(&stubConn{addr: peer}, http.StateNew)
	}
	req2 := httptest.NewRequest(http.MethodGet, "/latest/meta-data/", nil)
	req2.RemoteAddr = peer
	rec2 := httptest.NewRecorder()
	h2.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("want 200 at exactly %d conns, got %d", maxConnsPerPeer, rec2.Code)
	}
}

type stubConn struct {
	net.Conn
	addr string
}

func (c *stubConn) RemoteAddr() net.Addr { return stubAddr(c.addr) }

type stubAddr string

func (a stubAddr) Network() string { return "tcp" }
func (a stubAddr) String() string  { return string(a) }

func TestScrubIdlePeersRemovesEmptyEntries(t *testing.T) {
	l := newGuestLimiter()
	now := time.Now()
	for i := 0; i < 50; i++ {
		peer := "203.0.113." + strconv.Itoa(i)
		if !l.allowRequest(peer, now) {
			t.Fatalf("seed peer %s", peer)
		}
	}
	if l.peerCount() != 50 {
		t.Fatalf("want 50 peers, got %d", l.peerCount())
	}
	l.scrubIdlePeers(now.Add(limitWindow + time.Second))
	if l.peerCount() != 0 {
		t.Fatalf("want 0 peers after scrub, got %d", l.peerCount())
	}
}

func TestGuestLimitEnrollBudget(t *testing.T) {
	l := newGuestLimiter()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := guestLimitMiddleware(l, inner)

	for i := 0; i < maxEnrollPerPeer; i++ {
		path := "/latest/devid/enroll/start"
		if i%2 == 1 {
			path = "/latest/devid/enroll/finish"
		}
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.RemoteAddr = "10.0.0.7:1"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("enroll %d: %d", i, rec.Code)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/latest/devid/enroll/start", nil)
	req.RemoteAddr = "10.0.0.7:1"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429, got %d", rec.Code)
	}
}
