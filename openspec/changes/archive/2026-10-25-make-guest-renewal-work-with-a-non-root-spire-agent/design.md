# Design

## Context

See proposal.md — Why. The three defects were seen together on VM 399 on
2026-10-24 (bgl-infra TASK 2.3a). The agent there runs as `User=spire` with
its data directory, owned by `spire:spire`, as the DevID output directory;
the packaged oneshot runs as root.

## Goals / Non-Goals

**Goals:** a renewal by the packaged oneshot leaves the agent able to read its
files, finishes with the unit `success` while the agent restarts after it,
and leaves no transient object in the TPM however the run ends.

**Non-Goals:** the server; the packaged default paths
(`/var/lib/vtpm-mds/devid`, `/opt/spire/data/agent`), which a guest sets in
`/etc/default/devid-enroll`; file modes; signal handling inside the client;
SPIRE configuration.

## Decisions

- **D-a. Keep ownership on replace.** Before the `rename`, a client running
  as root gives the temporary file the owner and group of the file it
  replaces, or, when there is none, of the output directory. A non-root
  client cannot do that and leaves ownership as created. A failing `chown`
  fails the write, because a file the agent cannot read is the defect this
  fixes. The directory is the right fallback: a first enrollment into the
  agent's data directory then produces files the agent owns, and into a
  directory the client created (`0700`, root) files root owns, as before.
- **D-b. Queue the restart.** `systemctl --no-block try-restart
  spire-agent.service`. The SVID cache is already cleared, so the restart
  that systemd runs once the oneshot has finished attests the new LDevID.
  Outside the oneshot (a manual run) the restart still happens, just without
  the client waiting for it. Keeping `Before=spire-agent.service` matters
  for boot, where the agent must start after the files exist.
- **D-c. Resource manager by default.** `-tpm` defaults to `/dev/tpmrm0`.
  The kernel flushes a process's transient objects and sessions when it
  closes the device, including on `SIGKILL`, and arbitrates with the SPIRE
  agent, which uses the same device. A guest without `/dev/tpmrm0` (no TPM 2.0
  or a kernel before 4.12) can still pass `-tpm /dev/tpm0`. Flushing on
  `SIGTERM` in the client would cover only one of the ways a run ends.

## Risks / Trade-offs

- D-a trusts whoever owns the file or directory at the output path. That is
  the agent's own data directory, which root configured. The owner is read
  with `lstat`, so a symlink at the file path lends its own owner, not its
  target's, and `rename` replaces the link itself as before.
- D-b means the oneshot's success no longer says the agent restarted; the
  agent's own journal does.
