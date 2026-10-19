// vtpm-mds-bind keeps the nftables table that admits, on each guest port of
// the IMDS bridge, only the MAC in that guest's configuration. It runs beside
// vtpm-mds with CAP_NET_ADMIN, which the metadata daemon must not hold
// (bind-callers-to-their-bridge-port D-a).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dembaca/vtpm-mds/internal/bridgebind"
	"github.com/dembaca/vtpm-mds/internal/proxmox"
)

// Version is set via ldflags during build.
var Version = "dev"

func main() {
	bridge := flag.String("bridge", "vmbr_imds", "IMDS bridge whose guest ports are bound")
	pveRoot := flag.String("pve-root", proxmox.DefaultRoot, "Proxmox cluster filesystem")
	nft := flag.String("nft", "/usr/sbin/nft", "nft binary")
	runDir := flag.String("run-dir", os.Getenv("RUNTIME_DIRECTORY"), "directory for the last applied script")
	printOnly := flag.Bool("print", false, "print the table for the current configurations and exit")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("vtpm-mds-bind %s\n", Version)
		return
	}
	if *bridge == "" {
		log.Fatal("-bridge must not be empty")
	}
	if *printOnly {
		nics, err := proxmox.GuestNICs(*pveRoot)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Print(bridgebind.Render(*bridge, nics))
		return
	}

	log.Printf("vtpm-mds-bind version %s, bridge %s", Version, *bridge)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	b := &bridgebind.Binder{
		PVERoot: *pveRoot,
		Bridge:  *bridge,
		NFT:     bridgebind.Exec{Path: *nft},
		RunDir:  *runDir,
	}
	b.Run(ctx, bridgebind.Interval)
	log.Printf("vtpm-mds-bind stopped; table bridge %s left in place", bridgebind.Table)
}
