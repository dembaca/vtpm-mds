package imds

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/dembaca/vtpm-mds/internal/inventory"
)

// InstanceTags returns the instance tags of the VM record bound to the request,
// and nil for a caller with no bound record or a VM that has none. It never
// consults the connection or a header: tags come from the record only.
func InstanceTags(r *http.Request) map[string]string {
	vmConfig := inventory.GetVMConfigFromRequest(r)
	if vmConfig == nil || len(vmConfig.Tags) == 0 {
		return nil
	}
	return vmConfig.Tags
}

// writeNoTag answers a tags path that has nothing to serve: 404 with an empty
// text/plain body, as public-ipv4 does, which keeps it distinct from the
// catch-all handler's "404 page not found".
func writeNoTag(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusNotFound)
}

// HandleTags answers GET /latest/meta-data/tags/ with the single directory
// EC2 gives instance tags, instance.
func HandleTags(w http.ResponseWriter, r *http.Request) {
	if !validateToken(r) {
		w.Header().Set("X-Aws-Ec2-Metadata-Token", "required")
		http.Error(w, "IMDSv2 token required", http.StatusUnauthorized)
		return
	}
	if InstanceTags(r) == nil {
		writeNoTag(w)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintln(w, "instance")
}

// HandleInstanceTagKeys lists the keys of the caller's instance tags, one per
// line in ascending byte order.
func HandleInstanceTagKeys(w http.ResponseWriter, r *http.Request) {
	if !validateToken(r) {
		w.Header().Set("X-Aws-Ec2-Metadata-Token", "required")
		http.Error(w, "IMDSv2 token required", http.StatusUnauthorized)
		return
	}
	tags := InstanceTags(r)
	if tags == nil {
		writeNoTag(w)
		return
	}
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	w.Header().Set("Content-Type", "text/plain")
	for _, k := range keys {
		fmt.Fprintln(w, k)
	}
}

// HandleInstanceTag serves the value of one instance tag, with no trailing
// newline.
func HandleInstanceTag(w http.ResponseWriter, r *http.Request) {
	if !validateToken(r) {
		w.Header().Set("X-Aws-Ec2-Metadata-Token", "required")
		http.Error(w, "IMDSv2 token required", http.StatusUnauthorized)
		return
	}
	value, ok := InstanceTags(r)[r.PathValue("key")]
	if !ok {
		writeNoTag(w)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, value)
}
