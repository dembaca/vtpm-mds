package server

import (
	"testing"

	"github.com/dembaca/vtpm-mds/internal/inventory" // pragma: allowlist secret
)

func TestInventoryPinsFromCache(t *testing.T) {
	vmConfigCacheMu.Lock()
	prev := vmConfigCache
	vmConfigCache = inventory.VMConfigMap{
		"100": {VMID: "100", EKSHA256: "aaa"},
		"101": {VMID: "101"},
		"102": nil,
	}
	vmConfigCacheMu.Unlock()
	t.Cleanup(func() {
		vmConfigCacheMu.Lock()
		vmConfigCache = prev
		vmConfigCacheMu.Unlock()
	})
	got := inventoryPins()
	if len(got) != 1 || got["100"] != "aaa" {
		t.Fatalf("got %v", got)
	}
}
