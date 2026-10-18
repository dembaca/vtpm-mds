package server

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dembaca/vtpm-mds/imds" // pragma: allowlist secret
)

// Guest throttle defaults (throttle-guest-requests D-c).
const (
	limitWindow          = 60 * time.Second
	maxTokenPUTsPerPeer  = 30
	maxRequestsPerPeer   = 120
	maxConnsPerPeer      = 16
	maxEnrollPerPeer     = 6
)

type limitKind string

const (
	limitKindRequest limitKind = "request"
	limitKindToken   limitKind = "token"
	limitKindEnroll  limitKind = "enroll"
	limitKindConn    limitKind = "conn"
)

// guestLimiter tracks per-peer rate and connection budgets.
type guestLimiter struct {
	mu    sync.Mutex
	peers map[string]*peerBudget
}

type peerBudget struct {
	requests []time.Time
	tokens   []time.Time
	enrolls  []time.Time
	conns    int
	logged   map[limitKind]time.Time
}

func newGuestLimiter() *guestLimiter {
	return &guestLimiter{peers: make(map[string]*peerBudget)}
}

func (l *guestLimiter) peer(key string) *peerBudget {
	p, ok := l.peers[key]
	if !ok {
		p = &peerBudget{logged: make(map[limitKind]time.Time)}
		l.peers[key] = p
	}
	return p
}

func peerKeyFromAddr(remoteAddr string) string {
	if ip := imds.PeerIP(remoteAddr); ip != nil {
		return ip.String()
	}
	return "unknown"
}

func prune(times []time.Time, now time.Time) []time.Time {
	cut := now.Add(-limitWindow)
	i := 0
	for _, t := range times {
		if t.After(cut) {
			times[i] = t
			i++
		}
	}
	return times[:i]
}

func (l *guestLimiter) trip(peer string, kind limitKind) {
	p := l.peer(peer)
	if last, ok := p.logged[kind]; ok && time.Since(last) < limitWindow {
		return
	}
	p.logged[kind] = time.Now()
	log.Printf("rate limit: peer=%s kind=%s", peer, kind)
}

func (l *guestLimiter) allowRequest(peer string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	p := l.peer(peer)
	p.requests = prune(p.requests, now)
	if len(p.requests) >= maxRequestsPerPeer {
		l.trip(peer, limitKindRequest)
		return false
	}
	p.requests = append(p.requests, now)
	return true
}

func (l *guestLimiter) allowToken(peer string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	p := l.peer(peer)
	p.tokens = prune(p.tokens, now)
	if len(p.tokens) >= maxTokenPUTsPerPeer {
		l.trip(peer, limitKindToken)
		return false
	}
	p.tokens = append(p.tokens, now)
	return true
}

func (l *guestLimiter) allowEnroll(peer string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	p := l.peer(peer)
	p.enrolls = prune(p.enrolls, now)
	if len(p.enrolls) >= maxEnrollPerPeer {
		l.trip(peer, limitKindEnroll)
		return false
	}
	p.enrolls = append(p.enrolls, now)
	return true
}

func (l *guestLimiter) underConnCap(peer string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	p := l.peer(peer)
	// ConnState has already counted this connection as StateNew. More than
	// maxConnsPerPeer means this request is on a connection beyond the cap.
	if p.conns > maxConnsPerPeer {
		l.trip(peer, limitKindConn)
		return false
	}
	return true
}

// trackConnState updates concurrent-connection counts for the peer.
func (l *guestLimiter) trackConnState(c net.Conn, state http.ConnState) {
	peer := peerKeyFromAddr(c.RemoteAddr().String())
	l.mu.Lock()
	defer l.mu.Unlock()
	p := l.peer(peer)
	switch state {
	case http.StateNew:
		p.conns++
	case http.StateClosed, http.StateHijacked:
		if p.conns > 0 {
			p.conns--
		}
	}
}

func writeTooManyRequests(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Retry-After", "1")
	w.WriteHeader(http.StatusTooManyRequests)
	fmt.Fprint(w, "Too many requests\n")
}

func isEnrollPath(path string) bool {
	return strings.HasPrefix(path, "/latest/devid/enroll/")
}

// guestLimitMiddleware applies per-peer budgets. GET /health is exempt.
func guestLimitMiddleware(l *guestLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		peer := peerKeyFromAddr(r.RemoteAddr)
		now := time.Now()

		if !l.underConnCap(peer) {
			writeTooManyRequests(w)
			return
		}
		if !l.allowRequest(peer, now) {
			writeTooManyRequests(w)
			return
		}
		if r.Method == http.MethodPut && r.URL.Path == "/latest/api/token" {
			if !l.allowToken(peer, now) {
				writeTooManyRequests(w)
				return
			}
		}
		if r.Method == http.MethodPost && isEnrollPath(r.URL.Path) {
			if !l.allowEnroll(peer, now) {
				writeTooManyRequests(w)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
