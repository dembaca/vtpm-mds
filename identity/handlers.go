package identity

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dembaca/vtpm-mds/imds"
	"github.com/golang-jwt/jwt/v5"
)

// JWKS represents a JSON Web Key Set
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWK represents a JSON Web Key
type JWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// IdentityClaims represents the JWT claims for workload identity
type IdentityClaims struct {
	jwt.RegisteredClaims
	InstanceID string            `json:"instance_id"`
	Hostname   string            `json:"hostname"`
	IP         string            `json:"ip"`
	Level      string            `json:"level"`
	EKHash     string            `json:"ek_hash,omitempty"`
	PCRs       map[string]string `json:"pcrs,omitempty"`
	Attest     bool              `json:"attest"`
}

// Store reference (initialized by server)
var tokenStore *imds.TokenStore

// SetStore sets the global token store
func SetStore(ts *imds.TokenStore) {
	tokenStore = ts
}

// HandleIdentity issues a signed JWT/JWS identity document
func HandleIdentity(store *imds.TokenStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Validate IMDSv2 token
		if !validateToken(r) {
			w.Header().Set("X-Aws-Ec2-Metadata-Token", "required")
			http.Error(w, "IMDSv2 token required", http.StatusUnauthorized)
			return
		}

		// TODO: Check if this request has completed attestation
		// For now, issue a basic identity

		// The claims name the VM record bound to the connection, derived
		// exactly as GET /latest/meta-data/instance-id derives its value, and
		// the "ip" claim is the connection's peer address. No claim is taken
		// from a request header that names the caller. An unbound caller is
		// refused before this handler runs, by the imds.RequireVMIdentity
		// wrapper the route is registered with.
		instanceID := imds.InstanceID(r)
		clientIP := imds.LocalIPv4(r)
		hostname := getHostname(r)

		claims := IdentityClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    "vtpm-mds",
				Subject:   instanceID,
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
				ID:        fmt.Sprintf("%d", time.Now().Unix()),
			},
			InstanceID: instanceID,
			Hostname:   hostname,
			IP:         clientIP,
			Level:      "unattested", // TODO: Set based on attestation status
			Attest:     false,
		}

		// TODO: Sign with TPM-sealed key
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte("placeholder-secret-key"))
		if err != nil {
			http.Error(w, "Failed to generate identity", http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"token":   tokenString,
			"claims":  claims,
			"jwks_uri": "/.well-known/jwks.json",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// HandleJWKS serves the JSON Web Key Set
func HandleJWKS(w http.ResponseWriter, r *http.Request) {
	// TODO: Load actual JWKS from file
	// For now, return placeholder
	jwks := JWKS{
		Keys: []JWK{
			{
				Kty: "RSA",
				Kid: "key-1",
				Use: "sig",
				Alg: "RS256",
				N:   "placeholder-modulus",
				E:   "AQAB",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jwks)
}

// validateToken validates the IMDSv2 token
func validateToken(r *http.Request) bool {
	if tokenStore == nil {
		return false
	}
	token := r.Header.Get("X-Aws-Ec2-Metadata-Token")
	if token == "" {
		return false
	}
	return tokenStore.ValidateToken(token)
}

// getHostname returns the Host header the caller used, truncated at the first
// colon. It is the address the caller reached the service on, not the guest's
// own hostname, which the service does not know.
//
// The instance id and the peer address are not derived here: they come from
// imds.InstanceID and imds.LocalIPv4, so there is one implementation rather
// than the second copy that let this endpoint ignore the inventory.
func getHostname(r *http.Request) string {
	host := r.Host
	if host == "" {
		return "localhost.localdomain"
	}
	return strings.Split(host, ":")[0]
}


