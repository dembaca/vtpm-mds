package proxmox

import (
	"reflect"
	"testing"
)

func TestGuestNICsReadsEveryModelWithIndexAndBridge(t *testing.T) {
	root := pveTree(t, "hogan", map[string]map[string]string{
		"hogan": {
			"399.conf": "name: pilot\nnet0: virtio=BC:24:11:75:DA:C0,bridge=vmbr0\nnet1: virtio=BC:24:11:06:1D:B2,bridge=vmbr_imds\n",
			"300.conf": "net1: e1000=52:54:00:00:03:01,bridge=vmbr_imds,firewall=1\n",
			"42.conf":  "net2: model=rtl8139,macaddr=52:54:00:00:00:42,bridge=vmbr_imds\n",
			"7.conf":   "net0: vmxnet3=nonsense,bridge=vmbr_imds\n",
		},
		"holger": {"500.conf": "net0: virtio=52:54:00:00:05:00,bridge=vmbr_imds\n"},
	})
	got, err := GuestNICs(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []NIC{
		{VMID: "7", Index: 0, Model: "", MAC: "", Bridge: "vmbr_imds"},
		{VMID: "42", Index: 2, Model: "rtl8139", MAC: "52:54:00:00:00:42", Bridge: "vmbr_imds"},
		{VMID: "300", Index: 1, Model: "e1000", MAC: "52:54:00:00:03:01", Bridge: "vmbr_imds"},
		{VMID: "399", Index: 0, Model: "virtio", MAC: "bc:24:11:75:da:c0", Bridge: "vmbr0"},
		{VMID: "399", Index: 1, Model: "virtio", MAC: "bc:24:11:06:1d:b2", Bridge: "vmbr_imds"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GuestNICs:\n got %+v\nwant %+v", got, want)
	}
	if tap := got[4].Tap(); tap != "tap399i1" {
		t.Fatalf("Tap() = %q", tap)
	}
}

func TestGuestNICsReadsTheLiveSectionOnly(t *testing.T) {
	root := pveTree(t, "hogan", map[string]map[string]string{
		"hogan": {"100.conf": "net0: virtio=52:54:00:00:01:00,bridge=vmbr_imds\n" +
			"[PENDING]\nnet0: virtio=52:54:00:00:01:99,bridge=vmbr_imds\nnet1: virtio=52:54:00:00:01:01,bridge=vmbr_imds\n" +
			"[snap1]\nnet2: virtio=52:54:00:00:01:02,bridge=vmbr_imds\n"},
	})
	got, err := GuestNICs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].MAC != "52:54:00:00:01:00" {
		t.Fatalf("GuestNICs = %+v, want only the live net0", got)
	}
}

func TestGuestNICsSkipsIDsThatCannotNameATap(t *testing.T) {
	root := pveTree(t, "hogan", map[string]map[string]string{
		"hogan": {
			"1000000000.conf": "net0: virtio=52:54:00:00:00:01,bridge=vmbr_imds\n",
			"999999999.conf":  "net31: virtio=52:54:00:00:00:02,bridge=vmbr_imds\nnet100: virtio=52:54:00:00:00:03,bridge=vmbr_imds\n",
		},
	})
	got, err := GuestNICs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Tap() != "tap999999999i31" || len(got[0].Tap()) > 15 {
		t.Fatalf("GuestNICs = %+v, want only net31 of 999999999", got)
	}
}

func TestGuestNICsFailsWithoutALocalLink(t *testing.T) {
	root := pveTree(t, "", map[string]map[string]string{
		"hogan": {"100.conf": "net0: virtio=52:54:00:00:01:00,bridge=vmbr_imds\n"},
	})
	if _, err := GuestNICs(root); err == nil {
		t.Fatal("GuestNICs without local: want an error")
	}
}
