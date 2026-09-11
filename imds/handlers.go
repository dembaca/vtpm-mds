package imds

import (
	"encoding/json"
	"fmt"
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

	instanceID := getInstanceID(r)

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

	ip := getLocalIP(r)

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, ip)
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
		"instanceId":         getInstanceID(r),
		"imageId":            "proxmox-unknown",
		"instanceType":       "vm",
		"region":             getRegion(r),
		"availabilityZone":   getAvailabilityZone(r),
		"privateIp":          getLocalIP(r),
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

// validateToken validates the IMDSv2 token
func validateToken(r *http.Request) bool {
	if store == nil {
		return false
	}
	token := r.Header.Get("X-Aws-Ec2-Metadata-Token")
	if token == "" {
		return false
	}
	return store.ValidateToken(token)
}

// Helper functions to extract metadata
func getInstanceID(r *http.Request) string {
	// Get VM config from request context (set by server middleware)
	vmConfig := inventory.GetVMConfigFromRequest(r)
	if vmConfig != nil && vmConfig.VMID != "" {
		return fmt.Sprintf("i-%s", vmConfig.VMID)
	}

	// Fallback to old behavior if VM config not available
	clientIP := getClientIP(r)
	return fmt.Sprintf("i-%s", strings.ReplaceAll(clientIP, ".", "-"))
}

func getHostname(r *http.Request) string {
	host := r.Host
	if host == "" {
		return "localhost.localdomain"
	}
	return strings.Split(host, ":")[0]
}

func getLocalIP(r *http.Request) string {
	// Get from incoming connection
	ip := r.RemoteAddr
	parts := strings.Split(ip, ":")
	if len(parts) > 0 {
		return parts[0]
	}
	return "127.0.0.1"
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

func getClientIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		return strings.Split(forwarded, ",")[0]
	}
	ip := r.RemoteAddr
	parts := strings.Split(ip, ":")
	if len(parts) > 0 {
		return parts[0]
	}
	return "127.0.0.1"
}
