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
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dembaca/vtpm-mds/imds"
	"github.com/dembaca/vtpm-mds/internal/config"
	"github.com/dembaca/vtpm-mds/internal/devid"
	"github.com/dembaca/vtpm-mds/internal/inventory"
	"github.com/dembaca/vtpm-mds/internal/proxmox"
)

type Server struct {
	httpServer *http.Server
	cfg        *config.Config
	tokenStore *imds.TokenStore
	devid      *devid.Enroller
	limiter    *guestLimiter

	readyMu         sync.Mutex
	inventoryLoaded bool
	vmCount         int
	lastReload      time.Time
	lastErrorClass  string
	caReadable      bool
	caCheckedAt     time.Time
	// now is time.Now in production; tests inject a clock for the CA cache.
	now func() time.Time

	poll inventoryPoller
}

func New(cfg *config.Config) (*Server, error) {
	srv := &Server{
		cfg:        cfg,
		tokenStore: imds.NewTokenStore(cfg),
		limiter:    newGuestLimiter(),
		now:        time.Now,
	}

	activeServerMu.Lock()
	activeServer = srv
	activeServerMu.Unlock()

	// Load VM inventory (YAML path preferred; otherwise Proxmox /etc/pve)
	if err := srv.reloadNow(); err != nil {
		log.Printf("Warning: Failed to load VM config cache on startup: %v", err)
	}

	// Initialize handlers with stores
	imds.SetStore(srv.tokenStore)

	mux := http.NewServeMux()

	// handle registers a route, wrapping it in the unbound-caller refusal
	// when — and only when — its path is one of imds.IdentityBearingPaths,
	// the enumerated set of handlers that report the caller's own instance
	// identity. Registering through it is what puts a handler on the right
	// side of that line: the set is data, not a decision repeated per route.
	handle := func(pattern string, h http.HandlerFunc) {
		path := pattern
		if i := strings.IndexByte(pattern, ' '); i >= 0 {
			path = pattern[i+1:]
		}
		if imds.RequiresVMIdentity(path) {
			mux.Handle(pattern, imds.RequireVMIdentity(cfg.MDS.RequireVMIdentity, srv.tokenStore, h))
			return
		}
		mux.HandleFunc(pattern, h)
	}

	// IMDSv2 token endpoint. Deliberately unauthenticated and never refused:
	// an unbound caller can still mint a token.
	mux.HandleFunc("PUT /latest/api/token", imds.HandleCreateToken(srv.tokenStore))

	// Metadata endpoints (with token middleware)
	if cfg.MDS.EnableEC2Compat {
		handle("GET /latest/meta-data/instance-id", imds.HandleInstanceID)
		handle("GET /latest/meta-data/local-hostname", imds.HandleHostname)
		handle("GET /latest/meta-data/public-ipv4", imds.HandlePublicIPv4)
		handle("GET /latest/meta-data/local-ipv4", imds.HandleLocalIPv4)
		handle("GET /latest/meta-data/placement/availability-zone", imds.HandleAvailabilityZone)
		handle("GET /latest/meta-data/placement/region", imds.HandleRegion)
		handle("GET /latest/meta-data/placement/{$}", imds.HandlePlacementKeys)
		mux.HandleFunc("GET /latest/meta-data/placement", imds.HandlePlacementRedirect)
		handle("GET /latest/meta-data/placement/{key}", imds.HandlePlacementKey)
		handle("GET /latest/meta-data/services/domain", imds.HandleDomain)
		handle("GET /latest/meta-data/tags/{$}", imds.HandleTags)
		handle("GET /latest/meta-data/tags/instance", imds.HandleInstanceTagKeys)
		handle("GET /latest/meta-data/tags/instance/{key}", imds.HandleInstanceTag)
		handle("GET /latest/meta-data/", imds.HandleMetaDataIndex)
		handle("GET /latest/dynamic/instance-identity/document", imds.HandleInstanceIdentityDocument)
		// No signature path is registered: .../signature, .../pkcs7 and
		// .../rsa2048 all fall through to the catch-all. The document is
		// unsigned, and a signature no consumer can verify against a key
		// this service holds would only look like evidence.
	}

	// DevID enrollment. The setting is still named enable_tpm_attestation
	// because renaming a configuration key is a migration of its own; the
	// attestation and JWT identity endpoints it also used to gate were
	// removed, so today it gates enrollment alone.
	if cfg.MDS.EnableTPMAttestation {
		if enroller, err := loadDevIDEnroller(cfg); err != nil {
			log.Printf("Warning: DevID enrollment disabled: %v", err)
		} else if enroller != nil {
			srv.devid = enroller
			devid.Register(mux, enroller)
			log.Printf("DevID enrollment routes registered")
		}
	}

	// Health check (readiness; detail only for unbound callers — D-h)
	mux.HandleFunc("GET /health", srv.handleHealth)

	// Catch-all handler for debugging unmatched routes
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Unmatched route: %s %q from %s", r.Method, r.URL.Path, r.RemoteAddr)
		http.NotFound(w, r)
	})

	srv.httpServer = &http.Server{
		Addr:         cfg.MDS.ListenAddr,
		Handler:      vmConfigMiddleware(loggingMiddleware(guestLimitMiddleware(srv.limiter, mux))),
		ConnContext:  connContext,
		ConnState:    srv.limiter.trackConnState,
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

	s.startInventoryPoller(inventoryPollInterval)
	return s.httpServer.Serve(listener)
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.stopInventoryPoller()
	return s.httpServer.Shutdown(ctx)
}

// loggingMiddleware writes one access line per request: method, path, status,
// duration, the connection's peer address and the VM it is bound to, or "-". The
// peer is the connection's address and never a header; the path is quoted, so a
// path that holds a line break cannot write a second journal line.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		lw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lw, r)

		log.Printf(
			"%s %q %d %v %s %s",
			r.Method,
			r.URL.Path,
			lw.statusCode,
			time.Since(start),
			peerForLog(r),
			vmForLog(r),
		)
	})
}

// peerForLog is the IP address of the connection's peer, without the port, or
// "-" when it cannot be parsed.
func peerForLog(r *http.Request) string {
	if ip := imds.PeerIP(r.RemoteAddr); ip != nil {
		return ip.String()
	}
	return "-"
}

// vmForLog is the ID of the VM record the connection is bound to, or "-".
func vmForLog(r *http.Request) string {
	if vm := inventory.GetVMConfigFromRequest(r); vm != nil && vm.VMID != "" {
		return vm.VMID
	}
	return "-"
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// debugOn says whether the lines marked [DEBUG are written. It is set once, at
// start, from the -debug flag.
var debugOn atomic.Bool

// SetDebug switches the [DEBUG lines on or off. main calls it once, before the
// server is created.
func SetDebug(on bool) { debugOn.Store(on) }

// debugf writes a line that is marked [DEBUG by its caller, but only when -debug
// was given. Everything that reports a start, a reload, a refusal or a failure is
// written with log.Printf instead and so is not gated.
func debugf(format string, args ...any) {
	if debugOn.Load() {
		log.Printf(format, args...)
	}
}

// connContext stores VM config information in the request context
func connContext(ctx context.Context, c net.Conn) context.Context {
	if vmConn, ok := c.(*vmConnection); ok {
		// Trigger VM config extraction if not already done
		vmConn.vmConfigOnce.Do(func() {
			debugf("[DEBUG connContext] Extracting VM config for connection from %s", c.RemoteAddr())
			vmConn.extractVMConfig()
		})
		if vmConn.vmConfig != nil {
			debugf("[DEBUG connContext] Setting VM config in context: VMID=%s", vmConn.vmid)
			return context.WithValue(ctx, inventory.VMConfigContextKey, vmConn.vmConfig)
		} else {
			debugf("[DEBUG connContext] No VM config found for connection from %s", c.RemoteAddr())
		}
	} else {
		log.Printf("Connection is not a vmConnection (type %T), no VM can be bound to it", c)
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
		debugf("[DEBUG extractVMConfigByRouting] Starting VM config extraction for remote IP: %s", remoteIP)

		// Use ARP table to get MAC address, then find VM config
		macAddr := c.getMACFromARP(remoteIP)
		if macAddr != "" {
			debugf("[DEBUG extractVMConfigByRouting] Found MAC address: %s for IP: %s", macAddr, remoteIP)
			vmConfig := c.findVMConfigByMAC(macAddr)
			if vmConfig != nil {
				debugf("[DEBUG extractVMConfigByRouting] Found VM config: VMID=%s for MAC: %s", vmConfig.VMID, macAddr)
				c.vmid = vmConfig.VMID
				c.vmConfig = vmConfig
				return
			} else {
				debugf("[DEBUG extractVMConfigByRouting] No VM config found for MAC: %s", macAddr)
			}
		} else {
			debugf("[DEBUG extractVMConfigByRouting] No MAC address found for IP: %s", remoteIP)
		}
		debugf("[DEBUG extractVMConfigByRouting] No VM config found for IP: %s", remoteIP)
	}
}

// arpTablePath is the ARP table the caller's MAC is read from. It is a variable
// only so that a test can supply a table of its own; a test that swaps it must
// not run in parallel with another that reads it.
var arpTablePath = "/proc/net/arp"

// getMACFromARP reads the ARP table from /proc/net/arp to get MAC address for an IP
func (c *vmConnection) getMACFromARP(ip string) string {
	debugf("[DEBUG getMACFromARPFile] Reading %s for IP: %s", arpTablePath, ip)
	data, err := os.ReadFile(arpTablePath)
	if err != nil {
		log.Printf("Failed to read %s: %v", arpTablePath, err)
		return ""
	}

	// Parse tab-separated format: IP address | HW type | Flags | HW address | Mask | Device
	lines := strings.Split(string(data), "\n")
	debugf("[DEBUG getMACFromARPFile] Parsing %d lines from %s", len(lines), arpTablePath)
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
				debugf("[DEBUG getMACFromARPFile] Found MAC %s for IP %s (device: %s)", mac, ip, fields[5])
				return mac
			}
		}
	}
	debugf("[DEBUG getMACFromARP] No MAC address found in %s for IP: %s", arpTablePath, ip)
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
	debugf("[DEBUG findVMConfigByMAC] Looking for VM config with MAC: %s", mac)

	vmConfigCacheMu.RLock()
	defer vmConfigCacheMu.RUnlock()

	vmid := inventory.GetVMIDByMAC(vmConfigCache, mac)
	if vmid != "" {
		vmConfig, ok := vmConfigCache[vmid]
		if ok && vmConfig != nil {
			debugf("[DEBUG findVMConfigByMAC] Found in cache: MAC %s -> VMID %s", mac, vmid)
			return vmConfig
		}
	}

	debugf("[DEBUG findVMConfigByMAC] MAC %s not found in cache", mac)
	return nil
}

// RefreshVMConfigCache reloads VM identity data for caller identification.
// Prefer YAML inventory (QEMU lab / non-Proxmox); fall back to Proxmox configs.
func (s *Server) RefreshVMConfigCache() error {
	vmConfigCacheMu.Lock()
	defer vmConfigCacheMu.Unlock()

	var (
		vmConfigMap inventory.VMConfigMap
		topology    inventory.Topology
		err         error
		source      string
	)

	if s.cfg.MDS.InventoryPath != "" {
		source = s.cfg.MDS.InventoryPath
		debugf("[DEBUG RefreshVMConfigCache] Loading YAML inventory from %s", source)
		vmConfigMap, err = inventory.LoadYAML(s.cfg.MDS.InventoryPath)
		topology = inventory.DefaultTopology()
	} else {
		source = "proxmox:/etc/pve"
		debugf("[DEBUG RefreshVMConfigCache] Loading Proxmox VM configs")
		vmConfigMap, err = proxmox.ParseVMConfigs("")
		topology = proxmox.HostTopology("")
	}
	if err != nil {
		log.Printf("Failed to load VM configs from %s: %v", source, err)
		s.recordInventoryLoadFailure()
		return err
	}

	vmConfigCache = vmConfigMap
	inventory.SetHostTopology(topology)

	macCount := 0
	for _, vmConfig := range vmConfigCache {
		macCount += len(vmConfig.MACs)
	}

	log.Printf("Cache refresh complete (%s): %d VMs, %d MAC addresses cached", source, len(vmConfigCache), macCount)
	s.recordInventoryLoadSuccess(len(vmConfigCache))
	return nil
}

const (
	errClassInventoryLoadFailed = "inventory_load_failed"
	errClassDevIDCAUnreadable   = "devid_ca_unreadable"
	caReadableCacheTTL          = 5 * time.Second
)

func (s *Server) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s *Server) recordInventoryLoadSuccess(n int) {
	s.readyMu.Lock()
	defer s.readyMu.Unlock()
	s.inventoryLoaded = true
	s.vmCount = n
	s.lastReload = s.clock().UTC()
	s.lastErrorClass = ""
}

func (s *Server) recordInventoryLoadFailure() {
	s.readyMu.Lock()
	defer s.readyMu.Unlock()
	s.lastErrorClass = errClassInventoryLoadFailed
}

type healthDetail struct {
	Status          string `json:"status"`
	InventoryLoaded bool   `json:"inventory_loaded"`
	VMCount         int    `json:"vm_count"`
	DevIDCAReadable *bool  `json:"devid_ca_readable,omitempty"`
	LastReload      string `json:"last_reload"`
	LastError       string `json:"last_error"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	status, detail := s.readiness()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if inventory.IsBound(inventory.GetVMConfigFromRequest(r)) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
		return
	}
	_ = json.NewEncoder(w).Encode(detail)
}

func (s *Server) readiness() (status string, detail healthDetail) {
	s.readyMu.Lock()
	loaded := s.inventoryLoaded
	vmCount := s.vmCount
	lastReload := s.lastReload
	lastErr := s.lastErrorClass
	s.readyMu.Unlock()

	attestation := s.cfg.MDS.EnableTPMAttestation
	var caOK *bool
	if attestation {
		ok := s.devIDCAReadableCached()
		caOK = &ok
		if !ok && lastErr == "" {
			lastErr = errClassDevIDCAUnreadable
		}
	}

	status = "ok"
	if !loaded {
		status = "degraded"
	}
	if attestation && (caOK == nil || !*caOK) {
		status = "degraded"
	}
	if lastErr != "" {
		status = "degraded"
	}

	reloadStr := ""
	if !lastReload.IsZero() {
		reloadStr = lastReload.Format(time.RFC3339)
	}
	detail = healthDetail{
		Status:          status,
		InventoryLoaded: loaded,
		VMCount:         vmCount,
		DevIDCAReadable: caOK,
		LastReload:      reloadStr,
		LastError:       lastErr,
	}
	return status, detail
}

func (s *Server) devIDCAReadableCached() bool {
	now := s.clock()
	s.readyMu.Lock()
	if !s.caCheckedAt.IsZero() && now.Sub(s.caCheckedAt) < caReadableCacheTTL {
		ok := s.caReadable
		s.readyMu.Unlock()
		return ok
	}
	s.readyMu.Unlock()

	ok := s.checkDevIDCAReadable()
	s.readyMu.Lock()
	s.caReadable = ok
	s.caCheckedAt = now
	s.readyMu.Unlock()
	return ok
}

func (s *Server) checkDevIDCAReadable() bool {
	// Enrollment must actually be loaded; a failed start leaves devid nil
	// while enable_tpm_attestation is still true.
	if s.devid == nil {
		return false
	}
	certPath := s.cfg.MDS.DevIDCACert
	keyPath := s.cfg.MDS.DevIDCAKey
	if cred := credentialCAKeyPath(); cred != "" {
		keyPath = cred
	}
	if certPath == "" || keyPath == "" {
		return false
	}
	return readableFile(certPath) && readableFile(keyPath)
}

func readableFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

// RefreshActiveVMConfigCache reloads inventory for the running server (SIGHUP).
func RefreshActiveVMConfigCache() error {
	activeServerMu.RLock()
	srv := activeServer
	activeServerMu.RUnlock()
	if srv == nil {
		return fmt.Errorf("server not initialized")
	}
	return srv.reloadNow()
}

// credentialCAKeyName is the systemd credential the unit passes the DevID CA
// key as (LoadCredential=devid_ca.key:...).
const credentialCAKeyName = "devid_ca.key"

// credentialCAKeyPath returns the DevID CA key delivered as a systemd
// credential, or "" when there is none. An empty credential counts as none.
func credentialCAKeyPath() string {
	dir := os.Getenv("CREDENTIALS_DIRECTORY")
	if dir == "" {
		return ""
	}
	path := filepath.Join(dir, credentialCAKeyName)
	if fi, err := os.Stat(path); err != nil || fi.Size() == 0 {
		return ""
	}
	return path
}

func loadDevIDEnroller(cfg *config.Config) (*devid.Enroller, error) {
	certPath := cfg.MDS.DevIDCACert
	keyPath := cfg.MDS.DevIDCAKey
	// A credential replaces the configured path, which is then never touched:
	// running as vtpm-mds the service cannot even stat it.
	if cred := credentialCAKeyPath(); cred != "" {
		keyPath = cred
		log.Printf("DevID CA key from systemd credential")
	}
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
