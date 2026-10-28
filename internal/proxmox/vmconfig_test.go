package proxmox

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/dembaca/vtpm-mds/internal/inventory"
)

// pveTree builds a directory that plays /etc/pve: one qemu-server directory
// per node, the given config files under each, and local linked to localNode
// unless localNode is empty.
func pveTree(t *testing.T, localNode string, nodes map[string]map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for node, files := range nodes {
		dir := filepath.Join(root, "nodes", node, "qemu-server")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
				t.Fatalf("write %s: %v", name, err)
			}
		}
	}
	if localNode != "" {
		if err := os.Symlink(filepath.Join("nodes", localNode), filepath.Join(root, "local")); err != nil {
			t.Fatalf("link local: %v", err)
		}
	}
	return root
}

func parse(t *testing.T, root string) inventory.VMConfigMap {
	t.Helper()
	m, err := ParseVMConfigs(root)
	if err != nil {
		t.Fatalf("ParseVMConfigs: %v", err)
	}
	return m
}

func TestAGuestConfigBecomesAVMRecord(t *testing.T) {
	m := parse(t, pveTree(t, "node-a", map[string]map[string]string{
		"node-a": {"100.conf": "name: guest-a\nnet0: virtio=52:54:00:A1:B2:C3,bridge=vmbr0\n"},
	}))
	vm := m["100"]
	if vm == nil || vm.Name != "guest-a" || !slices.Equal(vm.MACs, []string{"52:54:00:a1:b2:c3"}) {
		t.Fatalf("unexpected record: %#v", vm)
	}
}

func TestANonVirtioNICYieldsNoMAC(t *testing.T) {
	m := parse(t, pveTree(t, "node-a", map[string]map[string]string{
		"node-a": {"100.conf": "net0: e1000=52:54:00:A1:B2:C3,bridge=vmbr0\n"},
	}))
	vm := m["100"]
	if vm == nil || len(vm.MACs) != 0 {
		t.Fatalf("expected a record with no MACs, got %#v", vm)
	}
	if inventory.GetVMIDByMAC(m, "52:54:00:a1:b2:c3") != "" {
		t.Fatal("a non-virtio NIC bound a caller")
	}
}

func TestNoProxmoxNodeDirectory(t *testing.T) {
	root := pveTree(t, "", map[string]map[string]string{
		"node-a": {"100.conf": "name: guest-a\n"},
	})
	_, err := ParseVMConfigs(root)
	want := filepath.Join(root, "local", "qemu-server")
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("expected an error naming %s, got %v", want, err)
	}
}

func TestAnUnreadableConfigFileIsSkipped(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads files regardless of mode")
	}
	root := pveTree(t, "node-a", map[string]map[string]string{
		"node-a": {"100.conf": "name: guest-a\n", "101.conf": "name: guest-b\n"},
	})
	if err := os.Chmod(filepath.Join(root, "nodes", "node-a", "qemu-server", "101.conf"), 0); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	m := parse(t, root)
	if m["100"] == nil || m["101"] != nil {
		t.Fatalf("expected only 100, got %v", m)
	}
}

func TestThisNodeIsReadNotTheFirstNodeByName(t *testing.T) {
	m := parse(t, pveTree(t, "bbb", map[string]map[string]string{
		"aaa": {"100.conf": "name: on-aaa\nnet0: virtio=52:54:00:00:00:0a\n"},
		"bbb": {"200.conf": "name: on-bbb\nnet0: virtio=52:54:00:00:00:0b\n"},
	}))
	if len(m) != 1 || m["200"] == nil || m["200"].Name != "on-bbb" {
		t.Fatalf("expected only bbb's guest, got %v", m)
	}
	if inventory.GetVMIDByMAC(m, "52:54:00:00:00:0a") != "" {
		t.Fatal("a guest of another node was bound")
	}
}

func TestASnapshotDoesNotBindARemovedNIC(t *testing.T) {
	m := parse(t, pveTree(t, "node-a", map[string]map[string]string{
		"node-a": {"100.conf": `name: guest-b
net0: virtio=52:54:00:00:00:01,bridge=vmbr0

[before-change]
name: guest-old
net0: virtio=52:54:00:00:00:02,bridge=vmbr0
snaptime: 1792000000
`},
	}))
	vm := m["100"]
	if vm == nil || vm.Name != "guest-b" || !slices.Equal(vm.MACs, []string{"52:54:00:00:00:01"}) {
		t.Fatalf("snapshot leaked into the record: %#v", vm)
	}
	if _, ok := vm.RawConfig["snaptime"]; ok {
		t.Fatal("a snapshot key reached the raw configuration")
	}
	if inventory.GetVMIDByMAC(m, "52:54:00:00:00:02") != "" {
		t.Fatal("a MAC only present in a snapshot bound a caller")
	}
}

func TestPendingAndSpecialSectionsAreNotLiveConfiguration(t *testing.T) {
	m := parse(t, pveTree(t, "node-a", map[string]map[string]string{
		"node-a": {"100.conf": `net0: virtio=52:54:00:00:00:01,bridge=vmbr0

[PENDING]
net1: virtio=52:54:00:00:00:03,bridge=vmbr0

[special:cloudinit]
ipconfig0: ip=dhcp
`},
	}))
	vm := m["100"]
	if vm == nil || !slices.Equal(vm.MACs, []string{"52:54:00:00:00:01"}) {
		t.Fatalf("expected only the live NIC, got %#v", vm)
	}
	if _, ok := vm.RawConfig["ipconfig0"]; ok {
		t.Fatal("a special section key reached the raw configuration")
	}
}

func TestAHandWrittenPublicAddressIsNotStored(t *testing.T) {
	for _, line := range []string{"public-ipv4: 203.0.113.7", "public-ipv4=203.0.113.7"} {
		m := parse(t, pveTree(t, "node-a", map[string]map[string]string{
			"node-a": {"100.conf": "name: guest-a\n" + line + "\n"},
		}))
		if v, ok := m["100"].RawConfig["public-ipv4"]; ok {
			t.Fatalf("%q was stored as %q", line, v)
		}
		if m["100"].RawConfig["name"] != "guest-a" {
			t.Fatalf("%q: other keys were lost: %v", line, m["100"].RawConfig)
		}
	}
}

func record(t *testing.T, lines string) *inventory.VMConfig {
	t.Helper()
	m := parse(t, pveTree(t, "node-a", map[string]map[string]string{
		"node-a": {"100.conf": "name: guest-a\n" + lines},
	}))
	return m["100"]
}

// logged runs f and returns everything it wrote to the standard logger.
func logged(t *testing.T, f func()) string {
	t.Helper()
	var buf strings.Builder
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	f()
	return buf.String()
}

// A tag with the prefix becomes an instance tag; one without is none of our
// business, and neither is logged.
func TestATagBecomesAnInstanceTag(t *testing.T) {
	var vm *inventory.VMConfig
	out := logged(t, func() { vm = record(t, "tags: vtpm_mds.io.owner.team-a;web\n") })
	if got := vm.Tags; len(got) != 1 || got["owner"] != "team-a" {
		t.Fatalf("got tags %v, want owner=team-a", got)
	}
	if out != "" {
		t.Errorf("an ordinary tag was logged: %q", out)
	}
	if vm.RawConfig["tags"] != "vtpm_mds.io.owner.team-a;web" {
		t.Errorf("the tags key itself must still be stored, got %q", vm.RawConfig["tags"])
	}
}

// The first dot after the key ends it; the value keeps every dot after it.
func TestAValueKeepsItsDots(t *testing.T) {
	vm := record(t, "tags: vtpm_mds.io.rack.b.2.07\n")
	if vm.Tags["rack"] != "b.2.07" {
		t.Fatalf("got %v, want rack=b.2.07", vm.Tags)
	}
}

// The prefix is matched without regard to case, key and value keep theirs.
func TestCaseIsKeptAndThePrefixIsMatchedWithoutIt(t *testing.T) {
	vm := record(t, "tags: VTPM_MDS.IO.Owner.Team-A\n")
	if vm.Tags["Owner"] != "Team-A" || len(vm.Tags) != 1 {
		t.Fatalf("got %v, want Owner=Team-A", vm.Tags)
	}
}

// All three separators Proxmox or a hand edit may use.
func TestTagsAreSplitAtSemicolonCommaAndSpace(t *testing.T) {
	vm := record(t, "tags: web, vtpm_mds.io.a.1 vtpm_mds.io.b.2;db\n")
	if len(vm.Tags) != 2 || vm.Tags["a"] != "1" || vm.Tags["b"] != "2" {
		t.Fatalf("got %v, want a=1 and b=2", vm.Tags)
	}
}

// Two tags for one key declare nothing for it, and the other keys are unaffected.
func TestTwoTagsWithOneKeyDeclareNothing(t *testing.T) {
	var vm *inventory.VMConfig
	out := logged(t, func() {
		vm = record(t, "tags: vtpm_mds.io.owner.a;vtpm_mds.io.owner.b;vtpm_mds.io.rack.7\n")
	})
	if _, ok := vm.Tags["owner"]; ok {
		t.Errorf("two owner tags must declare none, got %q", vm.Tags["owner"])
	}
	if vm.Tags["rack"] != "7" {
		t.Errorf("rack: got %q, want 7", vm.Tags["rack"])
	}
	if !strings.Contains(out, "VM 100") || !strings.Contains(out, `"owner"`) {
		t.Errorf("the log must name the VM and the key, got %q", out)
	}
}

// An invalid tag next to a valid one for the key still counts as two, so a typo
// is never resolved silently in the good tag's favour.
func TestAnInvalidTagNextToAValidOneDeclaresNothing(t *testing.T) {
	vm := record(t, "tags: vtpm_mds.io.owner.team-a;vtpm_mds.io.owner.Team!A\n")
	if _, ok := vm.Tags["owner"]; ok {
		t.Errorf("got owner %q, want none", vm.Tags["owner"])
	}
}

// A single invalid tag is ignored, and another VM's valid tag is unaffected.
func TestAnInvalidTagIsIgnored(t *testing.T) {
	m := parse(t, pveTree(t, "node-a", map[string]map[string]string{
		"node-a": {
			"100.conf": "tags: vtpm_mds.io.owner.\n",
			"101.conf": "tags: vtpm_mds.io.owner.team-a\n",
			"102.conf": "tags: vtpm_mds.io.owner\n",
			"103.conf": "tags: vtpm_mds.io.bad!key.x\n",
			"104.conf": "tags: vtpm_mds.io..x\n",
		},
	}))
	for _, id := range []string{"100", "102", "103", "104"} {
		if len(m[id].Tags) != 0 {
			t.Errorf("VM %s has an invalid tag, got %v", id, m[id].Tags)
		}
	}
	if m["101"].Tags["owner"] != "team-a" {
		t.Errorf("VM 101 was affected: %v", m["101"].Tags)
	}
}

// The limits of a key and a value.
func TestKeyAndValueLimits(t *testing.T) {
	longKey := strings.Repeat("k", 64)
	longValue := strings.Repeat("v", 256)
	vm := record(t, "tags: vtpm_mds.io."+longKey+".x;vtpm_mds.io.a."+longValue+";vtpm_mds.io.ok."+strings.Repeat("v", 255)+"\n")
	if len(vm.Tags) != 1 || len(vm.Tags["ok"]) != 255 {
		t.Fatalf("got %d tags, want only ok with 255 characters", len(vm.Tags))
	}
}

// A tag in a snapshot section does not count.
func TestATagInASnapshotDoesNotCount(t *testing.T) {
	m := parse(t, pveTree(t, "node-a", map[string]map[string]string{
		"node-a": {"100.conf": "name: guest-a\n\n[before-change]\ntags: vtpm_mds.io.owner.old\n"},
	}))
	if len(m["100"].Tags) != 0 {
		t.Errorf("the snapshot's tag was read: %v", m["100"].Tags)
	}
}

// A VM without any instance tag carries none, which is nil and not empty.
func TestAVMWithoutInstanceTagsCarriesNone(t *testing.T) {
	if vm := record(t, "tags: web;db\n"); vm.Tags != nil {
		t.Errorf("got %v, want nil", vm.Tags)
	}
}

// The tags that once declared a zone and a region place nothing, give no
// instance tag, and are logged once each so a forgotten one can be found.
func TestAFormerPlacementTagPlacesNothingAndIsLogged(t *testing.T) {
	var vm *inventory.VMConfig
	out := logged(t, func() { vm = record(t, "tags: mds.zone.rack-b;MDS.Region.region-1;web\n") })
	if vm.Tags != nil {
		t.Errorf("got tags %v, want none", vm.Tags)
	}
	for _, want := range []string{`"mds.zone.rack-b"`, `"MDS.Region.region-1"`} {
		if !strings.Contains(out, want) {
			t.Errorf("the log lacks %s: %q", want, out)
		}
	}
	if _, ok := vm.RawConfig["zone"]; ok {
		t.Errorf("a zone reached the record: %q", vm.RawConfig["zone"])
	}
}

// A hand-written zone or region line has no meaning: it is stored like any other
// key, in either form, and nothing reads it.
func TestAHandWrittenZoneAndRegionHaveNoMeaning(t *testing.T) {
	for _, lines := range []string{"zone: rack-x\nregion: region-x\n", "zone=rack-x\nregion=region-x\n"} {
		vm := record(t, lines)
		if vm.RawConfig["zone"] != "rack-x" || vm.RawConfig["region"] != "region-x" {
			t.Errorf("%q: stored %v", lines, vm.RawConfig)
		}
		if vm.Tags != nil {
			t.Errorf("%q: got tags %v", lines, vm.Tags)
		}
	}
}

func hostTree(t *testing.T, node, corosync string) string {
	t.Helper()
	root := pveTree(t, node, map[string]map[string]string{node: {}})
	if corosync != "" {
		if err := os.WriteFile(filepath.Join(root, "corosync.conf"), []byte(corosync), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const corosyncConf = `logging {
  to_syslog: yes
}

totem {
  cluster_name: %s
  config_version: 3
  version: 2
}
`

// az and region are the two keys that are always present.
func azRegion(t inventory.Topology) [2]string {
	return [2]string{t["availability-zone"], t["region"]}
}

func cluster(name string) string { return strings.Replace(corosyncConf, "%s", name, 1) }

// The host's zone and region are this node and this cluster, and node and cluster
// are served as keys of their own.
func TestTheHostTopologyIsThisNodeAndThisCluster(t *testing.T) {
	got := HostTopology(hostTree(t, "node-a", cluster("cluster-1")))
	want := inventory.Topology{"availability-zone": "node-a", "region": "cluster-1", "node": "node-a", "cluster": "cluster-1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// Without a cluster file the region is local, there is no cluster key, and
// nothing fails.
func TestAHostWithoutAClusterFileHasRegionLocalAndNoCluster(t *testing.T) {
	got := HostTopology(hostTree(t, "node-a", ""))
	want := inventory.Topology{"availability-zone": "node-a", "region": "local", "node": "node-a"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// A cluster file that names no cluster does the same.
func TestAClusterFileWithoutAClusterNameHasRegionLocal(t *testing.T) {
	got := HostTopology(hostTree(t, "node-a", "totem {\n  version: 2\n}\n"))
	if got["region"] != "local" {
		t.Fatalf("got region %q, want local", got["region"])
	}
	if _, ok := got["cluster"]; ok {
		t.Fatalf("got a cluster key %q, want none", got["cluster"])
	}
}

// A node name and a cluster name are served as spelled, capitals included.
func TestHostNamesKeepTheirCase(t *testing.T) {
	got := HostTopology(hostTree(t, "Node-A", cluster("Cluster-1")))
	if got["availability-zone"] != "Node-A" || got["region"] != "Cluster-1" || got["node"] != "Node-A" || got["cluster"] != "Cluster-1" {
		t.Fatalf("got %+v", got)
	}
}

// A name that is not valid is replaced by the default, and gives no node or
// cluster key.
func TestAnInvalidHostNameFallsBackToTheDefault(t *testing.T) {
	got := HostTopology(hostTree(t, "node-a", cluster("bad!name")))
	if got["region"] != "local" {
		t.Fatalf("got region %q, want local", got["region"])
	}
	if _, ok := got["cluster"]; ok {
		t.Fatalf("got a cluster key %q, want none", got["cluster"])
	}
	// A root without a local link keeps the default zone and has no node.
	noLink := t.TempDir()
	if got := HostTopology(noLink); !reflect.DeepEqual(got, inventory.DefaultTopology()) {
		t.Fatalf("got %+v, want %+v", got, inventory.DefaultTopology())
	}
}

// nodeConfig writes the node's own config file with the given description
// lines, each stored the way Proxmox stores a description: as '#' plus the URL
// encoding of the line.
func nodeConfig(t *testing.T, root, node string, lines ...string) {
	t.Helper()
	var b strings.Builder
	for _, l := range lines {
		b.WriteString("#" + strings.NewReplacer(":", "%3A", " ", "%20").Replace(l) + "\n")
	}
	b.WriteString("wakeonlan: aa:bb:cc:dd:ee:ff\n")
	if err := os.WriteFile(filepath.Join(root, "nodes", node, "config"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func described(t *testing.T, clusterName string, lines ...string) inventory.Topology {
	t.Helper()
	conf := ""
	if clusterName != "" {
		conf = cluster(clusterName)
	}
	root := hostTree(t, "node-a", conf)
	nodeConfig(t, root, "node-a", lines...)
	return HostTopology(root)
}

// The description of the node gives the host's zone and region ahead of the
// names, and the lines that declare nothing are ignored.
func TestTheNodeDescriptionGivesTheHostTopology(t *testing.T) {
	got := described(t, "cluster-1", "Location: AZ4.M1.02.03", "topology.vtpm_mds.io/region: region-a", "topology.vtpm_mds.io/zone: AZ4")
	if azRegion(got) != [2]string{"AZ4", "region-a"} {
		t.Fatalf("got %+v", got)
	}
}

// A description that names one key leaves the others to the names.
func TestADescriptionWithOneKeyTakesTheOthersFromTheNames(t *testing.T) {
	got := described(t, "cluster-1", "topology.vtpm_mds.io/zone: AZ4")
	if azRegion(got) != [2]string{"AZ4", "cluster-1"} {
		t.Fatalf("got %+v", got)
	}
}

// A node with no config file, which is the usual case, uses the names.
func TestANodeWithoutAConfigFileUsesTheNames(t *testing.T) {
	got := HostTopology(hostTree(t, "node-a", cluster("cluster-1")))
	if azRegion(got) != [2]string{"node-a", "cluster-1"} {
		t.Fatalf("got %+v", got)
	}
}

// An invalid value in the description is ignored, and the names stand in.
func TestAnInvalidDescriptionValueFallsBackToTheNames(t *testing.T) {
	var got inventory.Topology
	out := logged(t, func() {
		got = described(t, "cluster-1", "topology.vtpm_mds.io/zone: bad!zone", "topology.vtpm_mds.io/region: region-a")
	})
	if azRegion(got) != [2]string{"node-a", "region-a"} {
		t.Fatalf("got %+v", got)
	}
	if !strings.Contains(out, "bad!zone") {
		t.Errorf("the log must name the invalid value, got %q", out)
	}
}

// Two lines for one key declare nothing, as two tags do.
func TestTwoDescriptionLinesForOneKeyDeclareNothing(t *testing.T) {
	var got inventory.Topology
	out := logged(t, func() {
		got = described(t, "", "topology.vtpm_mds.io/zone: AZ3", "topology.vtpm_mds.io/zone: AZ4", "topology.vtpm_mds.io/region: region-a")
	})
	if azRegion(got) != [2]string{"node-a", "region-a"} {
		t.Fatalf("got %+v", got)
	}
	if !strings.Contains(out, `"availability-zone"`) {
		t.Errorf("the log must name the key, got %q", out)
	}
}

// A description can add a key of its own, beside the defaults.
func TestADescriptionAddsAKeyOfItsOwn(t *testing.T) {
	got := described(t, "cluster-1", "topology.vtpm_mds.io/rack: rack-01", "topology.vtpm_mds.io/host-id: h-01")
	if got["rack"] != "rack-01" || got["host-id"] != "h-01" {
		t.Fatalf("got %+v", got)
	}
	if got["node"] != "node-a" || got["cluster"] != "cluster-1" {
		t.Fatalf("the defaults were lost: %+v", got)
	}
}

// Without a line there is no rack.
func TestAKeyWithNoLineHasNoValue(t *testing.T) {
	if _, ok := HostTopology(hostTree(t, "node-a", cluster("c")))["rack"]; ok {
		t.Fatal("got a rack, want none")
	}
}

// The long form of the zone is the zone.
func TestTheLongFormOfTheZoneIsTheZone(t *testing.T) {
	got := described(t, "", "topology.vtpm_mds.io/availability-zone: room-a")
	if got["availability-zone"] != "room-a" {
		t.Fatalf("got %+v", got)
	}
	if _, ok := got["zone"]; ok {
		t.Fatalf("zone must not be a key of its own: %+v", got)
	}
}

// The short and the long form together are one key twice: nothing is declared.
func TestTheShortAndTheLongFormTogetherDeclareNothing(t *testing.T) {
	var got inventory.Topology
	out := logged(t, func() {
		got = described(t, "", "topology.vtpm_mds.io/zone: room-a", "topology.vtpm_mds.io/availability-zone: attic")
	})
	if got["availability-zone"] != "node-a" {
		t.Fatalf("got %+v, want the node name", got)
	}
	if !strings.Contains(out, "availability-zone") {
		t.Errorf("the log must name the key, got %q", out)
	}
}

// A description overrides node and cluster.
func TestADescriptionOverridesNodeAndCluster(t *testing.T) {
	got := described(t, "cluster-1", "topology.vtpm_mds.io/node: node-x", "topology.vtpm_mds.io/cluster: lab")
	if got["node"] != "node-x" || got["cluster"] != "lab" {
		t.Fatalf("got %+v", got)
	}
}

// Keys are case-sensitive and only exactly zone is the short form: Zone is a key
// of its own, and the prefix is matched as spelled.
func TestKeysAreCaseSensitive(t *testing.T) {
	got := described(t, "", "topology.vtpm_mds.io/Zone: x", "TOPOLOGY.VTPM_MDS.IO/rack: y")
	if got["Zone"] != "x" {
		t.Errorf("Zone must be a key of its own: %+v", got)
	}
	if got["availability-zone"] != "node-a" {
		t.Errorf("Zone must not be the zone: %+v", got)
	}
	if _, ok := got["rack"]; ok {
		t.Errorf("the prefix in capitals must not match: %+v", got)
	}
}

// A line with an invalid key or value is ignored and logged; a valid one beside
// it is read.
func TestALineWithAnInvalidKeyOrValueIsIgnored(t *testing.T) {
	var got inventory.Topology
	out := logged(t, func() {
		got = described(t, "", "topology.vtpm_mds.io/ra ck: x", "topology.vtpm_mds.io/row: a/b", "topology.vtpm_mds.io/rack: rack-01")
	})
	if got["rack"] != "rack-01" {
		t.Fatalf("got %+v", got)
	}
	if _, ok := got["row"]; ok {
		t.Errorf("row has an invalid value: %+v", got)
	}
	for _, want := range []string{"ra ck", "a/b"} {
		if !strings.Contains(out, want) {
			t.Errorf("the log lacks %q: %q", want, out)
		}
	}
}

// At most 64 keys are read; the later ones in byte order are ignored and the log
// says so.
func TestKeysBeyondTheLimitAreIgnored(t *testing.T) {
	var lines []string
	for i := 0; i < 70; i++ {
		lines = append(lines, fmt.Sprintf("topology.vtpm_mds.io/k%02d: v", i))
	}
	var got inventory.Topology
	out := logged(t, func() { got = described(t, "", lines...) })
	read := 0
	for key := range got {
		if strings.HasPrefix(key, "k") {
			read++
		}
	}
	if read != 64 {
		t.Fatalf("read %d keys, want 64", read)
	}
	if _, ok := got["k63"]; !ok {
		t.Errorf("k63 is within the first 64 in byte order: %+v", got)
	}
	if _, ok := got["k64"]; ok {
		t.Errorf("k64 is beyond the limit")
	}
	if !strings.Contains(out, "64") {
		t.Errorf("the log must name the limit, got %q", out)
	}
}

// The former prefix is not read: a description that carries it alone leaves the
// host's topology to the names.
func TestTheFormerDescriptionPrefixIsNotRead(t *testing.T) {
	got := described(t, "cluster-1", "topology.kubernetes.io/zone: AZ4", "topology.kubernetes.io/region: region-a")
	if azRegion(got) != [2]string{"node-a", "cluster-1"} {
		t.Fatalf("got %+v", got)
	}
}
