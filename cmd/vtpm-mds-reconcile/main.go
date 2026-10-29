// vtpm-mds-reconcile keeps SPIRE node aliases in line with the VMs, pools and
// tags of the configured Proxmox clusters, and bans agents of destroyed VMs.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dembaca/vtpm-mds/internal/reconcile"
)

// Version is set via ldflags during build.
var Version = "dev"

func main() {
	configPath := flag.String("config", "/etc/vtpm-mds-reconcile/config.yaml", "configuration file")
	once := flag.Bool("once", false, "run one pass that never bans, then exit")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("vtpm-mds-reconcile %s\n", Version)
		return
	}

	cfg, err := reconcile.LoadConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	var targets []reconcile.Target
	for _, c := range cfg.Clusters {
		client, err := c.Client()
		if err != nil {
			log.Fatal(err)
		}
		if err := client.CheckPrivileges(ctx); err != nil {
			log.Fatalf("cluster %s: %v", c.Name, err)
		}
		targets = append(targets, reconcile.Target{
			Cluster: reconcile.Cluster{Name: c.Name, Fingerprints: c.Fingerprints},
			Reader:  client,
		})
	}
	spire, err := reconcile.DialSPIRE(cfg.SPIRESocket, cfg.TrustDomain)
	if err != nil {
		log.Fatal(err)
	}
	defer spire.Close()
	if _, err := spire.Entries(ctx); err != nil {
		log.Fatalf("spire socket %s: %v", cfg.SPIRESocket, err)
	}

	log.Printf("vtpm-mds-reconcile version %s, %d clusters, interval %s", Version, len(cfg.Clusters), cfg.Interval)
	r := &reconcile.Reconciler{
		TD:      cfg.TrustDomain,
		Targets: targets,
		SPIRE:   spire,
		Bans:    &reconcile.BanTracker{},
		Log:     log.Default(),
	}
	if *once {
		skipped, err := r.Pass(ctx, false)
		if err != nil {
			log.Printf("pass: %v", err)
		} else if skipped > 0 {
			log.Printf("pass: %d of %d clusters left unchanged", skipped, len(targets))
		}
		if err != nil || skipped > 0 {
			spire.Close()
			os.Exit(1)
		}
		return
	}
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	for {
		if _, err := r.Pass(ctx, true); err != nil {
			log.Printf("pass: %v", err)
		}
		select {
		case <-ctx.Done():
			log.Printf("vtpm-mds-reconcile stopped")
			return
		case <-ticker.C:
		}
	}
}
