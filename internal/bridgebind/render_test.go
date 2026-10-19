package bridgebind

import (
	"testing"

	"github.com/dembaca/vtpm-mds/internal/proxmox"
)

const header = "table bridge vtpm_mds_bind\n" +
	"delete table bridge vtpm_mds_bind\n" +
	"table bridge vtpm_mds_bind {\n" +
	"\tchain from_guests {\n" +
	"\t\ttype filter hook prerouting priority -200; policy accept;\n"

const footer = "\t}\n}\n"

func TestRenderWithoutGuestPortsIsAnEmptyChain(t *testing.T) {
	if got, want := Render("vmbr_imds", nil), header+footer; got != want {
		t.Fatalf("Render:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderBindsEachPortOnTheBridgeToItsMAC(t *testing.T) {
	nics := []proxmox.NIC{
		{VMID: "7", Index: 0, Bridge: "vmbr_imds"}, // no valid MAC
		{VMID: "300", Index: 1, Model: "e1000", MAC: "52:54:00:00:03:01", Bridge: "vmbr_imds"},
		{VMID: "399", Index: 0, Model: "virtio", MAC: "bc:24:11:75:da:c0", Bridge: "vmbr0"},
		{VMID: "399", Index: 1, Model: "virtio", MAC: "bc:24:11:06:1d:b2", Bridge: "vmbr_imds"},
	}
	want := header +
		"\t\tiifname \"tap7i0\" counter drop\n" +
		"\t\tiifname \"tap300i1\" ether saddr != 52:54:00:00:03:01 counter drop\n" +
		"\t\tiifname \"tap300i1\" ether type arp arp saddr ether != 52:54:00:00:03:01 counter drop\n" +
		"\t\tiifname \"tap399i1\" ether saddr != bc:24:11:06:1d:b2 counter drop\n" +
		"\t\tiifname \"tap399i1\" ether type arp arp saddr ether != bc:24:11:06:1d:b2 counter drop\n" +
		footer
	if got := Render("vmbr_imds", nics); got != want {
		t.Fatalf("Render:\n%s\nwant:\n%s", got, want)
	}
	if n := Bound("vmbr_imds", nics); n != 3 {
		t.Fatalf("Bound = %d, want 3", n)
	}
	if got := Render("vmbr_imds", nics); got != want {
		t.Fatal("a second render of the same input differs")
	}
}
