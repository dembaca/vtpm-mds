# Proposal

## Why

On 2026-10-24 a forced renewal on VM 399 — the first renewal through the
packaged `devid-enroll.service` of 0.3.0, with `DEVID_OUT` and
`DEVID_SPIRE_DATA` both `/var/lib/spire/agent` and `spire-agent` running as
`User=spire` — issued a new LDevID and then broke the agent in three ways:

1. **Ownership.** The client replaces each file through a temporary file and
   `rename`, so the new `devid.crt.pem`, `devid.priv.blob` and
   `devid.pub.blob` belonged to `root:root` with mode `0600`. The agent
   crashed with `unable to load DevID files: cannot load certificate(s): open
   …` until the files were handed back with `chown spire:spire`.
2. **Deadlock.** `devid-enroll.service` is a oneshot ordered
   `Before=spire-agent.service`, and on a PEM change it runs `systemctl
   try-restart spire-agent.service` and waits for it. The agent's start job
   waits for the oneshot to finish, so the two waited on each other until
   `TimeoutStartSec=120` killed the oneshot: the unit ended `failed` although
   the renewal had succeeded, and the agent was stopped for two minutes.
3. **Raw TPM device.** The client opens `/dev/tpm0` by default, which has no
   resource manager. The run killed in (2) left two transient objects loaded,
   and the agent's `tpm2.Load` failed with `out of memory for object
   contexts` until they were flushed with `tpm2_flushcontext -t`.

Every guest whose SPIRE agent does not run as root is affected by (1), and
every guest with the packaged units by (2) and (3). Until this is fixed, an
automatic renewal leaves such a guest without a working agent.

## What Changes

- Replacing a DevID file keeps the owner and group of the file it replaces; a
  new file takes those of the output directory. Modes stay `0600`.
- The client queues the SPIRE agent restart (`systemctl --no-block
  try-restart`) instead of waiting for it.
- The default TPM device becomes `/dev/tpmrm0`, so the kernel's resource
  manager flushes whatever a killed or crashed run left loaded.

## Impact

- `internal/devid/client.go`, `internal/devid/renew.go`, their tests,
  `cmd/devid-enroll/main.go` and `debian/devid-enroll.1`. Guest package
  only; nothing on the host side (`vtpm-mds`, `vtpm-mds-bind`) changes.
- Spec: `devid-enrollment` modifies "Write DevID Materials On The Guest" and
  "Guest Renews The LDevID Before SPIRE And When Due".
- Released as 0.3.1 of the guest package; hosts can stay on 0.3.0.
- Migration: a guest that 0.3.0 already left with root-owned DevID files
  keeps that owner after the upgrade, because the replaced file's owner wins.
  Hand the three files to the agent's user once, or move them aside so the
  next run gives them the directory's owner. VM 399 was repaired by hand on
  2026-10-24.
