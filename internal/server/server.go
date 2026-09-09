package server

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dembaca/prox-mds/attest"
	"github.com/dembaca/prox-mds/identity"
	"github.com/dembaca/prox-mds/imds"
	"github.com/dembaca/prox-mds/internal/config"
	"github.com/dembaca/prox-mds/internal/devid"
	"github.com/dembaca/prox-mds/internal/inventory"
	"github.com/dembaca/prox-mds/internal/proxmox"
)

type Server struct {
	httpServer *http.Server
	cfg        *config.Config
	tokenStore *imds.TokenStore
	nonceStore *attest.NonceStore
	devid      *devid.Enroller
}

func New(cfg *config.Config) (*Server, error) {
	srv := &Server{
		cfg:        cfg,
		tokenStore: imds.NewTokenStore(cfg),
		nonceStore: attest.NewNonceStore(),
	}

	activeServerMu.Lock()
	activeServer = srv
	activeServerMu.Unlock()

	// Load VM inventory (YAML path preferred; otherwise Proxmox /etc/pve)
	if err := srv.RefreshVMConfigCache(); err != nil {
		log.Printf("Warning: Failed to load VM config cache on startup: %v", err)
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

		if enroller, err := loadDevIDEnroller(cfg); err != nil {
			log.Printf("Warning: DevID enrollment disabled: %v", err)
		} else if enroller != nil {
			srv.devid = enroller
			devid.Register(mux, enroller)
			log.Printf("DevID enrollment routes registered")
		}
	}

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Catch-all handler for debugging unmatched routes
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Unmatched route: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		http.NotFound(w, r)
	})

	srv.httpServer = &http.Server{
		Addr:         cfg.MDS.ListenAddr,
		Handler:      vmConfigMiddleware(loggingMiddleware(mux)),
		ConnContext:  connContext,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return srv, nil
}

func (s *Server) Start() error {
	// Parse listen address
	addr, err := net.ResolveTCPAddr("tcp", s.httpServer.Addr)
	if err != nil {
		return err
	}

	// Create custom listener to capture VM config information
	listener, err := newVMConnectionListener(addr)
	if err != nil {
		return err
	}

	return s.httpServer.Serve(listener)
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

// connectionMap stores VMIDs per connection
var connectionMap sync.Map

// connContext stores VM config information in the request context
func connContext(ctx context.Context, c net.Conn) context.Context {
	if vmConn, ok := c.(*vmConnection); ok {
		// Trigger VM config extraction if not already done
		vmConn.vmConfigOnce.Do(func() {
			log.Printf("[DEBUG connContext] Extracting VM config for connection from %s", c.RemoteAddr())
			vmConn.extractVMConfig()
		})
		if vmConn.vmConfig != nil {
			log.Printf("[DEBUG connContext] Setting VM config in context: VMID=%s", vmConn.vmid)
			return context.WithValue(ctx, inventory.VMConfigContextKey, vmConn.vmConfig)
		} else {
			log.Printf("[DEBUG connContext] No VM config found for connection from %s", c.RemoteAddr())
		}
	} else {
		log.Printf("[DEBUG connContext] Connection is not *vmConnection, type: %T", c)
	}
	return ctx
}

// vmConnectionListener wraps a TCP listener to capture VM config information
type vmConnectionListener struct {
	*net.TCPListener
}

func newVMConnectionListener(addr *net.TCPAddr) (*vmConnectionListener, error) {
	ln, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return nil, err
	}

	// Note: IP_PKTINFO is not supported on TCP sockets
	// We'll determine the VM config from the connection's remote IP via ARP lookup
	return &vmConnectionListener{TCPListener: ln}, nil
}

func (ln *vmConnectionListener) Accept() (net.Conn, error) {
	conn, err := ln.TCPListener.AcceptTCP()
	if err != nil {
		return nil, err
	}
	return &vmConnection{TCPConn: conn}, nil
}

// vmConnection wraps a TCP connection to extract VM config information
type vmConnection struct {
	*net.TCPConn
	vmConfigOnce sync.Once
	vmid         string
	vmConfig     *inventory.VMConfig
}

func (c *vmConnection) Read(b []byte) (int, error) {
	// Extract VM config on first read
	c.vmConfigOnce.Do(func() {
		c.extractVMConfig()
	})
	return c.TCPConn.Read(b)
}

func (c *vmConnection) extractVMConfig() {
	// For TCP connections, we determine the VM config from the remote IP
	// by looking up MAC address in ARP table and matching to VM config
	c.extractVMConfigByRouting()
}

func (c *vmConnection) extractVMConfigByRouting() {
	remoteAddr := c.TCPConn.RemoteAddr()
	if tcpAddr, ok := remoteAddr.(*net.TCPAddr); ok && tcpAddr.IP != nil {
		remoteIP := tcpAddr.IP.String()
		log.Printf("[DEBUG extractVMConfigByRouting] Starting VM config extraction for remote IP: %s", remoteIP)

		// Use ARP table to get MAC address, then find VM config
		macAddr := c.getMACFromARP(remoteIP)
		if macAddr != "" {
			log.Printf("[DEBUG extractVMConfigByRouting] Found MAC address: %s for IP: %s", macAddr, remoteIP)
			vmConfig := c.findVMConfigByMAC(macAddr)
			if vmConfig != nil {
				log.Printf("[DEBUG extractVMConfigByRouting] Found VM config: VMID=%s for MAC: %s", vmConfig.VMID, macAddr)
				c.vmid = vmConfig.VMID
				c.vmConfig = vmConfig
				connectionMap.Store(c.TCPConn, vmConfig.VMID)
				return
			} else {
				log.Printf("[DEBUG extractVMConfigByRouting] No VM config found for MAC: %s", macAddr)
			}
		} else {
			log.Printf("[DEBUG extractVMConfigByRouting] No MAC address found for IP: %s", remoteIP)
		}
		log.Printf("[DEBUG extractVMConfigByRouting] No VM config found for IP: %s", remoteIP)
	}
}

// getMACFromARP reads the ARP table from /proc/net/arp to get MAC address for an IP
func (c *vmConnection) getMACFromARP(ip string) string {
	log.Printf("[DEBUG getMACFromARPFile] Reading /proc/net/arp for IP: %s", ip)
	data, err := os.ReadFile("/proc/net/arp")
	if err != nil {
		log.Printf("[DEBUG getMACFromARPFile] Failed to read /proc/net/arp: %v", err)
		return ""
	}

	// Parse tab-separated format: IP address | HW type | Flags | HW address | Mask | Device
	lines := strings.Split(string(data), "\n")
	log.Printf("[DEBUG getMACFromARPFile] Parsing %d lines from /proc/net/arp", len(lines))
	for _, line := range lines {
		// Skip header line
		if strings.HasPrefix(line, "IP address") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 4 && fields[0] == ip {
			mac := strings.ToLower(fields[3])
			// Skip incomplete entries (0x0 MAC)
			if mac != "00:00:00:00:00:00" {
				log.Printf("[DEBUG getMACFromARPFile] Found MAC %s for IP %s (device: %s)", mac, ip, fields[5])
				return mac
			}
		}
	}
	log.Printf("[DEBUG getMACFromARP] No MAC address found in /proc/net/arp for IP: %s", ip)
	return ""
}

// vmConfigCache caches VM configs
var (
	vmConfigCache   inventory.VMConfigMap // VMID -> VMConfig
	vmConfigCacheMu sync.RWMutex
	activeServer    *Server
	activeServerMu  sync.RWMutex
)

// findVMConfigByMAC finds the VM config for the given MAC address
func (c *vmConnection) findVMConfigByMAC(mac string) *inventory.VMConfig {
	mac = strings.ToLower(strings.TrimSpace(mac))
	log.Printf("[DEBUG findVMConfigByMAC] Looking for VM config with MAC: %s", mac)

	vmConfigCacheMu.RLock()
	defer vmConfigCacheMu.RUnlock()

	vmid := inventory.GetVMIDByMAC(vmConfigCache, mac)
	if vmid != "" {
		vmConfig, ok := vmConfigCache[vmid]
		if ok && vmConfig != nil {
			log.Printf("[DEBUG findVMConfigByMAC] Found in cache: MAC %s -> VMID %s", mac, vmid)
			return vmConfig
		}
	}

	log.Printf("[DEBUG findVMConfigByMAC] MAC %s not found in cache", mac)
	return nil
}

// RefreshVMConfigCache reloads VM identity data for caller identification.
// Prefer YAML inventory (QEMU lab / non-Proxmox); fall back to Proxmox configs.
func (s *Server) RefreshVMConfigCache() error {
	vmConfigCacheMu.Lock()
	defer vmConfigCacheMu.Unlock()

	var (
		vmConfigMap inventory.VMConfigMap
		err         error
		source      string
	)

	if s.cfg.MDS.InventoryPath != "" {
		source = s.cfg.MDS.InventoryPath
		log.Printf("[DEBUG RefreshVMConfigCache] Loading YAML inventory from %s", source)
		vmConfigMap, err = inventory.LoadYAML(s.cfg.MDS.InventoryPath)
	} else {
		source = "proxmox:/etc/pve"
		log.Printf("[DEBUG RefreshVMConfigCache] Loading Proxmox VM configs")
		vmConfigMap, err = proxmox.ParseVMConfigs("")
	}
	if err != nil {
		log.Printf("[DEBUG RefreshVMConfigCache] Failed to load VM configs from %s: %v", source, err)
		return err
	}

	vmConfigCache = vmConfigMap

	macCount := 0
	for _, vmConfig := range vmConfigCache {
		macCount += len(vmConfig.MACs)
	}

	log.Printf("[DEBUG RefreshVMConfigCache] Cache refresh complete (%s): %d VMs, %d MAC addresses cached", source, len(vmConfigCache), macCount)
	return nil
}

// RefreshActiveVMConfigCache reloads inventory for the running server (SIGHUP).
func RefreshActiveVMConfigCache() error {
	activeServerMu.RLock()
	srv := activeServer
	activeServerMu.RUnlock()
	if srv == nil {
		return fmt.Errorf("server not initialized")
	}
	return srv.RefreshVMConfigCache()
}

func loadDevIDEnroller(cfg *config.Config) (*devid.Enroller, error) {
	certPath := cfg.MDS.DevIDCACert
	keyPath := cfg.MDS.DevIDCAKey
	if certPath == "" || keyPath == "" {
		log.Printf("Warning: devid_ca_cert/devid_ca_key not set; skipping DevID routes")
		return nil, nil
	}
	if _, err := os.Stat(certPath); err != nil {
		return nil, fmt.Errorf("DevID CA cert %s: %w", certPath, err)
	}
	if _, err := os.Stat(keyPath); err != nil {
		return nil, fmt.Errorf("DevID CA key %s: %w", keyPath, err)
	}
	ca, err := devid.LoadCA(certPath, keyPath)
	if err != nil {
		return nil, err
	}

	ekRoots := x509.NewCertPool()
	if cfg.MDS.EKCAChain != "" {
		pemBytes, err := os.ReadFile(cfg.MDS.EKCAChain)
		if err != nil {
			return nil, fmt.Errorf("read ek_ca_chain: %w", err)
		}
		if !ekRoots.AppendCertsFromPEM(pemBytes) {
			// Also try parsing a single DER-in-PEM cert manually for clearer errors.
			block, _ := pem.Decode(pemBytes)
			if block == nil {
				return nil, fmt.Errorf("ek_ca_chain: no certificates found")
			}
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("ek_ca_chain: %w", err)
			}
			ekRoots.AddCert(cert)
		}
	} else {
		log.Printf("Warning: ek_ca_chain empty; DevID EK verification will fail")
	}

	return devid.NewEnroller(ca, ekRoots, nil), nil
}

// vmConfigMiddleware extracts VM config from request context (already set by connContext)
// This middleware is kept for compatibility but the VM config is already in context
func vmConfigMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// VM config is already in context from connContext, just pass through
		next.ServeHTTP(w, r)
	})
}
