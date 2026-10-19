package server

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGuestLimitExemptsHealthGETOnly(t *testing.T) {
	l := newGuestLimiter()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"status":"ok"}`)
	})
	h := guestLimitMiddleware(l, inner)

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
	if rec.Header().Get("Retry-After") != retryAfterSeconds {
		t.Fatalf("Retry-After %q", rec.Header().Get("Retry-After"))
	}
	if rec.Header().Get("Connection") != "close" {
		t.Fatalf("Connection %q", rec.Header().Get("Connection"))
	}

	health := httptest.NewRequest(http.MethodGet, "/health", nil)
	health.RemoteAddr = "10.0.0.5:1234"
	hrec := httptest.NewRecorder()
	h.ServeHTTP(hrec, health)
	if hrec.Code != http.StatusOK {
		t.Fatalf("GET /health want 200, got %d", hrec.Code)
	}

	postHealth := httptest.NewRequest(http.MethodPost, "/health", nil)
	postHealth.RemoteAddr = "10.0.0.5:1234"
	prec := httptest.NewRecorder()
	h.ServeHTTP(prec, postHealth)
	if prec.Code != http.StatusTooManyRequests {
		t.Fatalf("POST /health want 429 when flooded, got %d", prec.Code)
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

	other := httptest.NewRequest(http.MethodPut, "/latest/api/token", nil)
	other.RemoteAddr = "10.0.0.9:9"
	orec := httptest.NewRecorder()
	h.ServeHTTP(orec, other)
	if orec.Code != http.StatusOK {
		t.Fatalf("peer B want 200, got %d", orec.Code)
	}
}

func TestGuestLimitConnCapClosesFurtherConnection(t *testing.T) {
	l := newGuestLimiter()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := guestLimitMiddleware(l, inner)
	peer := "198.51.100.9:1"

	conns := make([]*stubConn, 0, maxConnsPerPeer)
	for i := 0; i < maxConnsPerPeer; i++ {
		c := &stubConn{addr: peer}
		conns = append(conns, c)
		l.trackConnState(c, http.StateNew)
	}
	l.mu.Lock()
	n := l.peer(peerKeyFromAddr(peer)).conns
	l.mu.Unlock()
	if n != maxConnsPerPeer {
		t.Fatalf("want %d counted conns, got %d", maxConnsPerPeer, n)
	}

	req := httptest.NewRequest(http.MethodGet, "/latest/meta-data/", nil)
	req.RemoteAddr = peer
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 at exactly %d conns, got %d", maxConnsPerPeer, rec.Code)
	}

	over := &stubConn{addr: peer}
	l.trackConnState(over, http.StateNew)
	if atomic.LoadInt32(&over.closed) != 1 {
		t.Fatal("17th connection should be closed at StateNew")
	}
	l.mu.Lock()
	n = l.peer(peerKeyFromAddr(peer)).conns
	l.mu.Unlock()
	if n != maxConnsPerPeer {
		t.Fatalf("over-cap conn must not increment count; got %d", n)
	}

	l.trackConnState(over, http.StateClosed)
	l.mu.Lock()
	n = l.peer(peerKeyFromAddr(peer)).conns
	l.mu.Unlock()
	if n != maxConnsPerPeer {
		t.Fatalf("closing rejected conn changed count to %d", n)
	}

	l.trackConnState(conns[0], http.StateClosed)
	next := &stubConn{addr: peer}
	l.trackConnState(next, http.StateNew)
	if atomic.LoadInt32(&next.closed) != 0 {
		t.Fatal("connection under cap should not be closed")
	}
}

type stubConn struct {
	net.Conn
	addr   string
	closed int32
}

func (c *stubConn) RemoteAddr() net.Addr { return stubAddr(c.addr) }
func (c *stubConn) Close() error {
	atomic.StoreInt32(&c.closed, 1)
	return nil
}

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

func TestScrubKeepsPeerWithOpenConnection(t *testing.T) {
	l := newGuestLimiter()
	now := time.Now()
	peer := "203.0.113.200"
	if !l.allowRequest(peer, now) {
		t.Fatal("seed")
	}
	c := &stubConn{addr: peer + ":1"}
	l.trackConnState(c, http.StateNew)
	l.scrubIdlePeers(now.Add(limitWindow + time.Second))
	if l.peerCount() != 1 {
		t.Fatalf("open connection must keep peer; got %d", l.peerCount())
	}
	l.trackConnState(c, http.StateClosed)
	l.scrubIdlePeers(now.Add(2*limitWindow + time.Second))
	if l.peerCount() != 0 {
		t.Fatalf("want 0 after close+scrub, got %d", l.peerCount())
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
