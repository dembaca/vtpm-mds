package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/dembaca/vtpm-mds/internal/devid"
	"github.com/google/go-tpm/legacy/tpm2"
)

// Version is set via ldflags during build.
var Version = "dev"

func main() {
	tpmPath := flag.String("tpm", "/dev/tpmrm0", "TPM device path (the resource manager flushes what a killed run left loaded)")
	mdsURL := flag.String("mds", "http://169.254.169.254", "MDS base URL")
	outDir := flag.String("out", "/var/lib/vtpm-mds/devid", "output directory for DevID materials")
	cn := flag.String("cn", "", "platform identity Common Name (ignored by the service; subject is always CN=<vmid>)")
	spireData := flag.String("spire-data", "", "SPIRE agent data_dir; on PEM change, clear SVID cache here and try-restart spire-agent (empty skips)")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("devid-enroll %s\n", Version)
		os.Exit(0)
	}

	rwc, err := tpm2.OpenTPM(*tpmPath)
	if err != nil {
		log.Fatalf("open TPM %s: %v", *tpmPath, err)
	}
	defer rwc.Close()

	if err := devid.ClientEnrollOrRenew(rwc, devid.ClientOptions{
		MDSBaseURL:   *mdsURL,
		PlatformCN:   *cn,
		OutDir:       *outDir,
		SpireDataDir: *spireData,
	}); err != nil {
		log.Fatalf("DevID enroll failed: %v", err)
	}
	fmt.Printf("DevID enrollment check done; materials under %s\n", *outDir)
	os.Exit(0)
}
