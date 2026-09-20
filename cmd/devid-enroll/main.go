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
	tpmPath := flag.String("tpm", "/dev/tpm0", "TPM device path")
	mdsURL := flag.String("mds", "http://169.254.169.254", "MDS base URL")
	outDir := flag.String("out", "/var/lib/vtpm-mds/devid", "output directory for DevID materials")
	cn := flag.String("cn", "", "platform identity Common Name")
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

	if err := devid.ClientEnroll(rwc, *mdsURL, *cn, *outDir); err != nil {
		log.Fatalf("DevID enroll failed: %v", err)
	}
	fmt.Printf("DevID enrollment complete; materials written to %s\n", *outDir)
	os.Exit(0)
}
