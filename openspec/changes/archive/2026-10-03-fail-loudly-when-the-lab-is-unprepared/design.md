# Design

## Context

See proposal.md — Why.

Three scripts, one pattern: a precondition that is checked too late, or not at
all, or checked and then ignored.

`gen-lab-pki.sh` has the check — `if [[ -f /var/lib/swtpm-localca/issuercert.pem ]]`
— and an `else` that warns and continues. Because `set -euo pipefail` is in
force, continuing is a deliberate choice, not an oversight; it was simply the
wrong one.

`setup-guest-tpm.sh` starts swtpm with `--ctrl` for QEMU. swtpm's `--server`
is a separate option and can be given at the same time, so one instance can
serve both a control socket and a command socket. The test wants the latter.

`e2e-devid-guest.sh` is the orchestrator and the natural place for a
precondition gate; today it has none.

## Goals / Non-Goals

**Goals:**

- A lab run that cannot succeed fails in its first seconds, naming what is
  missing and how to get it.
- `TestEnrollAgainstSwtpm` runs on a prepared lab host instead of skipping.
- The EK chain is built from the CA the host actually uses.

**Non-Goals:**

- Installing anything. The scripts report what is missing and point at
  `cloud-install.sh`; they do not apt-get on the operator's behalf. Which host
  may run `cloud-install.sh` at all is the subject of a separate change.
- Making the lab run on a Proxmox host. That is
  `confine-the-qemu-lab-to-its-own-host`.
- Changing the DevID CA. The lab's self-signed CA stays; only the EK trust
  chain is taken from the host.
- Turning the skip in `TestEnrollAgainstSwtpm` into a failure. A developer
  machine with no lab must still be able to run `go test ./...`.

## Decisions

### Read `/etc/swtpm_setup.conf` rather than hardcoding a second path

`swtpm_setup` resolves its CA through `create_certs_tool_config` in
`/etc/swtpm_setup.conf`; that file is the only authority on which CA issued
the EK certificate the guest will present. The script parses it for the config
path, parses that for `issuercert`, and falls back to the default
`/var/lib/swtpm-localca/issuercert.pem` when neither is set.

Hardcoding `/etc/ssl/certs/proxmox_tpm_ca.crt` as an additional candidate was
rejected: it fixes one host and leaves the next site to rediscover the
problem.

### Fail, do not warn

An EK chain that cannot be built is fatal. Everything downstream — the service
that loads `ek_ca_chain`, the swtpm test, the guest enrollment — depends on it,
and each of them reports the absence as something else: a `401`, a file-not-
found, a timeout. The one place that knows the real reason is this script.

### A second swtpm with its own state, not a second socket on the guest's

**This decision was written the other way round and disproved during
implementation; it is corrected here rather than left as a wrong record.**

The original reasoning was: add `--server type=unixio,path=…/swtpm.sock` to the
guest's existing invocation, because two instances over one state directory
would race and corrupt NVRAM. The first half does not work. A guest whose swtpm
carries both `--ctrl` and `--server` makes QEMU fail at start with
`tpm-emulator: Failed to send CMD_SET_DATAFD`. Isolated on copies of the state:
`--ctrl` alone runs, `--ctrl` plus `--flags startup-clear` runs, `--ctrl` plus
`--server` does not.

So `setup-guest-tpm.sh` leaves the guest's instance exactly as it was and starts
a **second** swtpm over a **separate** state directory
(`vms/<name>-hosttest/tpm`), serving the command socket the test opens. The
NVRAM objection still holds and is still honoured — it was an argument against
sharing a state directory, not against a second process. One process per state
directory, verifiable with `ps`.

The host-test instance needs `--flags not-need-init,startup-clear`: nothing
else sends `TPM2_Startup` on that socket, and an unstarted TPM answers
`TPM_RC_INITIALIZE` to every command, which the test reports as
`device is not a TPM 2.0` and skips on.

Both TPMs take their EK certificate from the same swtpm local CA, so one
`ek-chain.pem` covers the guest and the host test.

The alternative — pointing the test at the control socket — does not work
either: a control socket speaks swtpm's control protocol, not TPM commands.

### Precondition gate at the top of the orchestrator

`e2e-devid-guest.sh` checks every binary it and its children invoke —
`qemu-system-x86_64`, `qemu-img`, `cloud-localds`, `swtpm`, `swtpm_setup`,
`ip`, `curl` — plus a readable OVMF code file, before the first side effect.
A missing one is reported with the tool name and the suggestion to run
`scripts/qemu-lab/cloud-install.sh`.

Checking inside each child script was rejected: the operator would then
discover the second missing tool only after fixing the first, and the
orchestrator is where the whole dependency set is known.

### Discover OVMF, keep the overrides

The firmware search tries `/usr/share/OVMF/`, then
`/usr/share/pve-edk2-firmware/`, then `/usr/share/edk2/ovmf/`, and the
existing `OVMF_CODE` and `OVMF_VARS_TEMPLATE` environment variables continue to
win when set. The precondition gate reports the paths it tried when nothing is
found.

## Risks / Trade-offs

- **[Risk] Making `gen-lab-pki.sh` fatal breaks a workflow that relied on it
  completing without an EK chain** → Accepted, and that workflow was producing
  a lab in which enrollment cannot succeed. The error names the file it could
  not find and the config it consulted.
- **[Risk] Parsing `/etc/swtpm_setup.conf` with shell text processing is
  fragile** → Mitigation: the format is `key = value` lines; the parse is
  tolerant of spacing and ignores comments, and every failure path falls back
  to the documented default rather than to an empty string.
- **[Risk] The added `--server` socket is a second way into the guest's TPM
  while the guest is running** → Mitigation: it is a unix socket under the
  lab's run directory with the same ownership as the control socket, on a host
  that is already a single-tenant lab. Noted because it is a real widening.
- **[Trade-off] The precondition gate duplicates knowledge of what the child
  scripts call** → Accepted; a task verifies the list against the scripts, and
  the cost of drift is a late failure, which is what exists today anyway.

## Migration Plan

None. Development tooling; nothing is deployed. An operator whose lab was
half-prepared will now be told on the next run.

## Open Questions

None.
