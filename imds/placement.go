package imds

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/dembaca/vtpm-mds/internal/inventory"
)

// HandlePlacementKeys answers GET /latest/meta-data/placement/ with the keys of the
// host's topology, one per line in ascending byte order. The topology is the
// node's, so every caller reads the same list.
func HandlePlacementKeys(w http.ResponseWriter, r *http.Request) {
	if !validateToken(r) {
		w.Header().Set("X-Aws-Ec2-Metadata-Token", "required")
		http.Error(w, "IMDSv2 token required", http.StatusUnauthorized)
		return
	}
	topology := inventory.HostTopology()
	keys := make([]string, 0, len(topology))
	for k := range topology {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	w.Header().Set("Content-Type", "text/plain")
	for _, k := range keys {
		fmt.Fprintln(w, k)
	}
}

// HandlePlacementKey serves one key of the host's topology, with no trailing
// newline. A key the topology does not have is the empty 404 of every metadata
// path with nothing to serve, never the metadata index.
func HandlePlacementKey(w http.ResponseWriter, r *http.Request) {
	if !validateToken(r) {
		w.Header().Set("X-Aws-Ec2-Metadata-Token", "required")
		http.Error(w, "IMDSv2 token required", http.StatusUnauthorized)
		return
	}
	value, ok := inventory.HostTopology()[r.PathValue("key")]
	if !ok {
		writeNoTag(w)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, value)
}

// HandlePlacementRedirect answers GET /latest/meta-data/placement, without the
// trailing slash, with a 301 to the directory. It needs no token, as the redirect
// of /latest/meta-data does not.
func HandlePlacementRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/latest/meta-data/placement/", http.StatusMovedPermanently)
}
