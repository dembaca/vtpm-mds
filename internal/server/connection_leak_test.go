package server

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// liveObjects is the number of live heap objects after two collections.
func liveObjects() uint64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapObjects
}

// Opening and closing many connections from a bound caller must not make the heap
// grow with their number: nothing may keep a closed connection reachable. The
// caller is bound the way it is in production, by ARP and the VM config cache; the
// ARP table is a file of the test's own that maps the loopback address to the MAC
// of a VM in the lab inventory.
func TestClosedConnectionsAreNotRetained(t *testing.T) {
	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)

	srv, token := signatureTestServer(t)
	vmConfigCacheMu.RLock()
	savedCache := vmConfigCache
	vmConfigCacheMu.RUnlock()
	t.Cleanup(func() {
		vmConfigCacheMu.Lock()
		vmConfigCache = savedCache
		vmConfigCacheMu.Unlock()
	})
	if err := srv.RefreshVMConfigCache(); err != nil {
		t.Fatalf("RefreshVMConfigCache: %v", err)
	}
	mac := ""
	vmConfigCacheMu.RLock()
	for _, vm := range vmConfigCache {
		if len(vm.MACs) > 0 {
			mac = vm.MACs[0]
			break
		}
	}
	vmConfigCacheMu.RUnlock()
	if mac == "" {
		t.Fatal("the lab inventory has no VM with a MAC")
	}

	arp := filepath.Join(t.TempDir(), "arp")
	table := fmt.Sprintf("IP address       HW type     Flags       HW address            Mask     Device\n127.0.0.1        0x1         0x2         %s     *        lo\n", mac)
	if err := os.WriteFile(arp, []byte(table), 0o644); err != nil {
		t.Fatal(err)
	}
	oldArp := arpTablePath
	arpTablePath = arp
	defer func() { arpTablePath = oldArp }()

	ln, err := newVMConnectionListener(&net.TCPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	go srv.httpServer.Serve(ln)
	defer srv.httpServer.Close()

	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}, Timeout: 5 * time.Second}
	get := func() {
		resp, err := client.Get("http://" + ln.Addr().String() + "/health")
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	// Guard the premise: the caller really is bound to a VM, or a leak of bound
	// connections could not show.
	req, _ := http.NewRequest("GET", "http://"+ln.Addr().String()+"/latest/meta-data/instance-id", nil)
	req.Header.Set("X-Aws-Ec2-Metadata-Token", token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "i-100" {
		t.Fatalf("the caller is not bound to VM 100: got %d %q", resp.StatusCode, body)
	}

	const warmup, n = 200, 3000
	for i := 0; i < warmup; i++ {
		get()
	}
	before := liveObjects()
	for i := 0; i < n; i++ {
		get()
	}
	time.Sleep(200 * time.Millisecond) // let the server side close its ends
	after := liveObjects()

	var grew uint64
	if after > before {
		grew = after - before
	}
	t.Logf("live heap objects grew by %d over %d connections", grew, n)
	// A retained connection costs several objects, so a leak of one per
	// connection would add several times n. Allow half of n for noise.
	if grew > n/2 {
		t.Fatalf("live heap objects grew by %d over %d connections", grew, n)
	}
}
