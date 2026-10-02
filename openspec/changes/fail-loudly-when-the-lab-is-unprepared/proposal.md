# Proposal

## Why

The QEMU lab prepares an environment for tests, and when it cannot, it says so
too quietly and too late. Three instances, all hit in one session on
2026-10-02 while verifying the DevID enrollment changes:

**`gen-lab-pki.sh` never produces an EK chain on this host, and exits 0.** It
looks for `/var/lib/swtpm-localca/issuercert.pem`, the default swtpm local CA
layout. `hogan` names its own CA configuration in `/etc/swtpm_setup.conf`
(`create_certs_tool_config = /etc/swtpm-bgl-proxmox-localca.conf`, whose
`issuercert` is `/etc/ssl/certs/proxmox_tpm_ca.crt`). The script prints
`WARNING: swtpm-localca issuer missing; create a guest TPM first`, writes no
`ek-chain.pem`, and reports `Lab PKI ready`. The failure surfaces later as a
test that cannot read `/etc/vtpm-mds/ek-chain.pem`, or — worse — as an
enrollment rejected with `401`.

**Nothing creates the socket `TestEnrollAgainstSwtpm` opens.** The test opens
`/var/lib/mds-lab/run/swtpm.sock` and skips when it is absent.
`setup-guest-tpm.sh` starts swtpm with `--ctrl` only, at
`guest100.tpm.sock`, for QEMU — a control socket, which accepts no TPM
commands. No script in the repository has ever created a command socket, so
the test has always skipped. A test that can only skip is not coverage, and
its skip reads like a pass in a summary.

**`e2e-devid-guest.sh` checks none of its tools.** It ran PKI generation, host
bridge setup, a 597 MB image check and vTPM manufacturing before failing at
`create-guest.sh` line 170 with `cloud-localds: command not found`. Its
dependencies are installed by `cloud-install.sh`, which it never calls and
never names. It also hardcodes `/usr/share/OVMF/OVMF_CODE_4M.fd`, absent on a
host whose firmware lives in `/usr/share/pve-edk2-firmware/`.

The common fault is not the missing pieces — it is that each is discovered
minutes of work after the point where it could have been reported.

## What Changes

- `gen-lab-pki.sh` reads `/etc/swtpm_setup.conf` to find the CA configuration
  actually in use, takes `issuercert` from it, and **exits non-zero** when it
  cannot assemble an EK chain. It no longer reports success without one.
- `setup-guest-tpm.sh` additionally starts a swtpm **command** socket at the
  path `TestEnrollAgainstSwtpm` opens, alongside the existing QEMU control
  socket, so the test runs instead of skipping.
- `e2e-devid-guest.sh` checks for every external tool it needs before doing
  any work, and names `cloud-install.sh` in the error when one is missing.
- The OVMF firmware path is discovered from the host rather than hardcoded,
  with the existing `OVMF_CODE` / `OVMF_VARS_TEMPLATE` overrides unchanged.
- No change to the daemon, the client, the packaging, or any capability.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This change alters test and lab tooling under `scripts/qemu-lab/`; no
behaviour of the service changes, so `.openspec.yaml` sets `skip_specs: true`.

## Impact

- `scripts/qemu-lab/gen-lab-pki.sh`, `setup-guest-tpm.sh`,
  `e2e-devid-guest.sh`, `start-guest.sh`.
- `internal/devid/enroll_swtpm_test.go` only if the socket path needs to move;
  the preference is to make the setup produce the path the test already uses.
- Anyone running the lab. A run that would have failed late now fails in the
  first seconds, with the reason.
- Task 4.2 of `bind-ek-certificate-to-endorsement-key` and of
  `enforce-authenticated-ldevid-subject`: both were satisfied on 2026-10-02
  only by building the socket and the EK chain by hand, and both record that.
  After this change the documented path produces them.
