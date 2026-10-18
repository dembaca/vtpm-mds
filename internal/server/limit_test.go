package server

import (
	"io"
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
