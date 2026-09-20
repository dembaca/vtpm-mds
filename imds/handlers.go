package imds

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/dembaca/vtpm-mds/internal/inventory"
)

// Store references (initialized by server)
var store *TokenStore

// SetStore sets the global token store
func SetStore(s *TokenStore) {
	store = s
}

// IdentityBearingPaths enumerates the request paths whose handlers report the
// caller's own instance identity. When mds.require_vm_identity is true,
// exactly these are refused to a caller that the vm-inventory capability
// could not bind to a VM record, and no others: the remaining metadata paths
// echo the caller's own request, report its own peer address or serve a fixed
// string, so none of them asserts an identity the service vouches for.
//
// GET /latest/identity is served by the identity package, which imports this
// one. It is listed here so the refused set lives in one place, as
// instance-metadata's "Refuse An Instance Identity To Callers With No VM
// Record" requires.
var IdentityBearingPaths = []string{
	"/latest/meta-data/instance-id",
	"/latest/dynamic/instance-identity/document",
	"/latest/dynamic/instance-identity/signature",
	"/latest/identity",
}

// RequiresVMIdentity reports whether path is one of IdentityBearingPaths, and
// therefore whether its registration must be wrapped in RequireVMIdentity.
func RequiresVMIdentity(path string) bool {
	for _, p := range IdentityBearingPaths {
		if p == path {
			return true
		}
	}
	return false
}

// RequireVMIdentity wraps a handler that reports the caller's instance
// identity so that a caller with no bound VM record is refused.
//
// The refusal is answered with http.NotFound — status 404 and the body
// "404 page not found" — which is byte for byte what the catch-all handler
// returns for an unregistered route, so an unbound caller cannot tell a
// refused endpoint from one that does not exist. Each refusal is logged with
// the caller's peer address, which is the side of the connection an operator
// can trust.
//
// The session token is validated first, so the two failures compose
// predictably: a caller with no valid token is passed to the handler, which
// answers 401 whether or not it is bound. The identity check then runs before
// the handler derives any instance value, so a refused request never reads
// the inventory or the peer address for a value it will not send.
//
// When require is false the handler is returned unwrapped and nothing is
// refused.
func RequireVMIdentity(require bool, s *TokenStore, next http.Handler) http.Handler {
	if !require {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !storeValidatesToken(s, r) {
			next.ServeHTTP(w, r)
			return
		}
		if vmConfig := inventory.GetVMConfigFromRequest(r); vmConfig == nil || vmConfig.VMID == "" {
			log.Printf("Refusing %s %s: no VM record bound to caller %s", r.Method, r.URL.Path, r.RemoteAddr)
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// HandleCreateToken handles IMDSv2 token creation
func HandleCreateToken(s *TokenStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse TTL from request header (optional, IMDSv2 compatible)
		_ = r.Header.Get("X-aws-ec2-metadata-token-ttl-seconds")

		// Generate token
		token, err := s.GenerateToken()
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Write token as plain text (IMDSv2 compatible)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, token.Token)
	}
}

// HandleMetaDataIndex handles the metadata index page
func HandleMetaDataIndex(w http.ResponseWriter, r *http.Request) {
	if !validateToken(r) {
		w.Header().Set("X-Aws-Ec2-Metadata-Token", "required")
		http.Error(w, "IMDSv2 token required", http.StatusUnauthorized)
		return
	}

	paths := []string{
		"instance-id",
		"local-hostname",
		"local-ipv4",
		"public-ipv4",
		"placement/",
		"services/",
	}

	w.Header().Set("Content-Type", "text/plain")
	for _, path := range paths {
		fmt.Fprintln(w, path)
	}
}

// HandleInstanceID returns the instance ID
func HandleInstanceID(w http.ResponseWriter, r *http.Request) {
	if !validateToken(r) {
		w.Header().Set("X-Aws-Ec2-Metadata-Token", "required")
		http.Error(w, "IMDSv2 token required", http.StatusUnauthorized)
		return
	}

	instanceID := InstanceID(r)

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, instanceID)
}

// HandleHostname returns the local hostname
func HandleHostname(w http.ResponseWriter, r *http.Request) {
	if !validateToken(r) {
		w.Header().Set("X-Aws-Ec2-Metadata-Token", "required")
		http.Error(w, "IMDSv2 token required", http.StatusUnauthorized)
		return
	}

	hostname := getHostname(r)

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, hostname)
}

// HandleLocalIPv4 returns the local IPv4 address
func HandleLocalIPv4(w http.ResponseWriter, r *http.Request) {
	if !validateToken(r) {
		w.Header().Set("X-Aws-Ec2-Metadata-Token", "required")
		http.Error(w, "IMDSv2 token required", http.StatusUnauthorized)
		return
	}

	ip := LocalIPv4(r)

	w.Header().Set("Content-Type", "text/plain")
	if ip != "" {
		fmt.Fprint(w, ip)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

// HandlePublicIPv4 returns the public IPv4 address
func HandlePublicIPv4(w http.ResponseWriter, r *http.Request) {
	if !validateToken(r) {
		w.Header().Set("X-Aws-Ec2-Metadata-Token", "required")
		http.Error(w, "IMDSv2 token required", http.StatusUnauthorized)
		return
	}

	ip := getPublicIP(r)

	w.Header().Set("Content-Type", "text/plain")
	if ip != "" {
		fmt.Fprint(w, ip)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

// HandleAvailabilityZone returns the availability zone
func HandleAvailabilityZone(w http.ResponseWriter, r *http.Request) {
	if !validateToken(r) {
		w.Header().Set("X-Aws-Ec2-Metadata-Token", "required")
		http.Error(w, "IMDSv2 token required", http.StatusUnauthorized)
		return
	}

	az := getAvailabilityZone(r)

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, az)
}

// HandleDomain returns the domain
func HandleDomain(w http.ResponseWriter, r *http.Request) {
	if !validateToken(r) {
		w.Header().Set("X-Aws-Ec2-Metadata-Token", "required")
		http.Error(w, "IMDSv2 token required", http.StatusUnauthorized)
		return
	}

	domain := getDomain(r)

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, domain)
}

// HandleInstanceIdentityDocument returns EC2-style instance identity document
func HandleInstanceIdentityDocument(w http.ResponseWriter, r *http.Request) {
	if !validateToken(r) {
		w.Header().Set("X-Aws-Ec2-Metadata-Token", "required")
		http.Error(w, "IMDSv2 token required", http.StatusUnauthorized)
		return
	}

	doc := map[string]interface{}{
		"instanceId":         InstanceID(r),
		"imageId":            "proxmox-unknown",
		"instanceType":       "vm",
		"region":             getRegion(r),
		"availabilityZone":   getAvailabilityZone(r),
		"privateIp":          LocalIPv4(r),
		"devpayProductCodes": nil,
		"version":            "2017-09-30",
		"billingProducts":    nil,
		"accountId":          "012345678901",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(doc)
}

// HandleInstanceIdentitySignature returns a PKCS#7 signature
func HandleInstanceIdentitySignature(w http.ResponseWriter, r *http.Request) {
	if !validateToken(r) {
		w.Header().Set("X-Aws-Ec2-Metadata-Token", "required")
		http.Error(w, "IMDSv2 token required", http.StatusUnauthorized)
		return
	}

	// TODO: Implement actual PKCS#7 signing with CA
	signature := "dGVzdC1zaWduYXR1cmU=" // base64 placeholder

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, signature)
}

// validateToken validates the IMDSv2 token against the package-level store.
func validateToken(r *http.Request) bool {
	return storeValidatesToken(store, r)
}

// storeValidatesToken validates the IMDSv2 token against an explicit store,
// so a wrapper that is handed the server's store does not depend on the
// package-level one having been set.
func storeValidatesToken(s *TokenStore, r *http.Request) bool {
	if s == nil {
		return false
	}
	token := r.Header.Get("X-Aws-Ec2-Metadata-Token")
	if token == "" {
		return false
	}
	return s.ValidateToken(token)
}

// InstanceID returns the instance id for a request: "i-" followed by the VM
// id of the record the vm-inventory capability bound to the connection.
//
// When no record is bound it synthesizes "i-" followed by the connection's
// peer address with every "." replaced by "-". That path is only reachable
// with mds.require_vm_identity set to false, because RequireVMIdentity
// refuses an unbound caller before the handler runs; the synthesized id names
// the connection, not an instance, and is not an identity.
//
// No request header is consulted, so a caller cannot name itself. This is the
// one derivation: the identity package calls it rather than keeping a copy.
func InstanceID(r *http.Request) string {
	// Get VM config from request context (set by server middleware)
	vmConfig := inventory.GetVMConfigFromRequest(r)
	if vmConfig != nil && vmConfig.VMID != "" {
		return fmt.Sprintf("i-%s", vmConfig.VMID)
	}

	return fmt.Sprintf("i-%s", strings.ReplaceAll(getClientIP(r), ".", "-"))
}

func getHostname(r *http.Request) string {
	host := r.Host
	if host == "" {
		return "localhost.localdomain"
	}
	return strings.Split(host, ":")[0]
}

// PeerIP parses a RemoteAddr-shaped string (as set by net/http on
// http.Request.RemoteAddr) into the connection's peer IP. It uses
// net.SplitHostPort first, since RemoteAddr for a TCP peer is "host:port"
// with an IPv6 host bracketed; when that fails (no port present, which
// tests and some non-TCP callers may produce) it falls back to treating the
// whole value as a bare host. The result is validated with net.ParseIP, so a
// value that is neither shape yields nil rather than a text fragment.
//
// This is the one place RemoteAddr-style values are parsed; imds and
// identity both call it rather than splitting on the first colon.
func PeerIP(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	return net.ParseIP(host)
}

// LocalIPv4 returns the IPv4 address of the connection's peer, or the empty
// string when the peer has no IPv4 address. It is what local-ipv4 serves,
// what the identity document reports as privateIp and what the workload
// identity document reports as its "ip" claim; no request header is
// consulted.
func LocalIPv4(r *http.Request) string {
	ip := PeerIP(r.RemoteAddr)
	if ip == nil {
		return ""
	}
	v4 := ip.To4()
	if v4 == nil {
		return ""
	}
	return v4.String()
}

func getPublicIP(r *http.Request) string {
	// TODO: Extract from Proxmox VM config
	return ""
}

func getAvailabilityZone(r *http.Request) string {
	// TODO: Extract from Proxmox cluster config
	return "proxmox"
}

func getRegion(r *http.Request) string {
	// TODO: Extract from Proxmox cluster config
	return "local"
}

func getDomain(r *http.Request) string {
	return "localdomain"
}

// getClientIP returns the address of the connection's peer. It deliberately
// consults no request header: the service listens directly on the link-local
// metadata address with nothing in front of it, so a forwarded-for header is
// caller-supplied data and never evidence of the caller's address. The branch
// that preferred it was removed by refuse-unbound-metadata-callers.
func getClientIP(r *http.Request) string {
	ip := PeerIP(r.RemoteAddr)
	if ip == nil {
		return "127.0.0.1"
	}
	return ip.String()
}
