package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/dembaca/prox-mds/internal/config"
	"github.com/dembaca/prox-mds/attest"
	"github.com/dembaca/prox-mds/identity"
	"github.com/dembaca/prox-mds/imds"
)

type Server struct {
	httpServer  *http.Server
	cfg         *config.Config
	tokenStore  *imds.TokenStore
	nonceStore  *attest.NonceStore
}

func New(cfg *config.Config) (*Server, error) {
	srv := &Server{
		cfg: cfg,
		tokenStore: imds.NewTokenStore(cfg),
		nonceStore: attest.NewNonceStore(),
	}

	// Initialize handlers with stores
	imds.SetStore(srv.tokenStore)
	attest.SetStore(srv.nonceStore)
	identity.SetStore(srv.tokenStore)

	mux := http.NewServeMux()
	
	// IMDSv2 token endpoint
	mux.HandleFunc("PUT /latest/api/token", imds.HandleCreateToken(srv.tokenStore))
	
	// Metadata endpoints (with token middleware)
	if cfg.MDS.EnableEC2Compat {
		mux.HandleFunc("GET /latest/meta-data/instance-id", imds.HandleInstanceID)
		mux.HandleFunc("GET /latest/meta-data/local-hostname", imds.HandleHostname)
		mux.HandleFunc("GET /latest/meta-data/public-ipv4", imds.HandlePublicIPv4)
		mux.HandleFunc("GET /latest/meta-data/local-ipv4", imds.HandleLocalIPv4)
		mux.HandleFunc("GET /latest/meta-data/placement/availability-zone", imds.HandleAvailabilityZone)
		mux.HandleFunc("GET /latest/meta-data/services/domain", imds.HandleDomain)
		mux.HandleFunc("GET /latest/meta-data/", imds.HandleMetaDataIndex)
		mux.HandleFunc("GET /latest/dynamic/instance-identity/document", imds.HandleInstanceIdentityDocument)
		mux.HandleFunc("GET /latest/dynamic/instance-identity/signature", imds.HandleInstanceIdentitySignature)
	}
	
	// TPM attestation endpoints
	if cfg.MDS.EnableTPMAttestation {
		mux.HandleFunc("GET /latest/attest/nonce", attest.HandleNonce)
		mux.HandleFunc("POST /latest/attest", attest.HandleAttest)
		mux.HandleFunc("GET /latest/identity", identity.HandleIdentity(srv.tokenStore))
		mux.HandleFunc("GET /.well-known/jwks.json", identity.HandleJWKS)
	}
	
	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	srv.httpServer = &http.Server{
		Addr:         cfg.MDS.ListenAddr,
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return srv, nil
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// loggingMiddleware logs HTTP requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		lw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lw, r)
		
		log.Printf(
			"%s %s %d %v",
			r.Method,
			r.URL.Path,
			lw.statusCode,
			time.Since(start),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

