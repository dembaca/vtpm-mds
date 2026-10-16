# Design

## Context

See proposal.md — Why. Evidence collected 2026-10-05 on the cloud lab agent:

- `maxAuthFail=3`, `lockoutInterval=1000`, `lockoutRecovery=1000` (swtpm default).
- Host-test command-socket enroll (full `CreateSigningRequest`…`ActivateCredential`,
  twice) left `lockoutCounter` at 0.
- Guest live read via SSH after successful `devid-enroll`: 0.
- Graceful `poweroff` + SIGTERM to swtpm: still 0.
- `stop-guest.sh` (kill QEMU, leave/then kill swtpm) after enroll: 0 → 1 in NVRAM.

`stop-guest.sh` today:

```bash
kill "$pid"   # QEMU
# wait up to ~10s
kill -9 "$pid"
# tap cleanup
# does NOT stop the guest swtpm — setup-guest-tpm.sh owns that
```

QEMU dying while the emulator still has sessions is what records a DA-protected
authorization failure into the vTPM state.

## Goals / Non-Goals

**Goals:**

- Stopping a lab guest does not increment `lockoutCounter`.
- Reusing `vms/guest100/tpm` across successful e2e runs does not hit DA lockout
  solely because of how the previous run was stopped.
- Operators measuring the counter after a stop see the same value the guest saw
  while alive (absent other TPM use).

**Non-Goals:**

- Changing DevID enrollment, key templates, or `ActivateCredential`.
- Raising swtpm `maxAuthFail` or calling `TPM2_DictionaryAttackLockReset`.
- Fixing nested-KVM / TCG choice.
- Proxmox / VM 399 (out of scope for this host).

## Decisions

### Graceful powerdown before kill

`stop-guest.sh` SHALL try, in order:

1. QMP `system_powerdown` on the existing QMP unix socket (no SSH dependency,
   works even when cloud-init left SSH half-broken).
2. Wait for the QEMU pid to exit (tens of seconds under TCG).
3. Only then SIGTERM, then SIGKILL, as today.

SSH `poweroff` is an optional fast path when keys and sshd are known good; QMP
is the reliable default because the lab already opens a QMP socket in
`start-guest.sh`.

### Stop swtpm after QEMU

Today unrelated scripts kill the guest swtpm ad hoc. After this change,
`stop-guest.sh` (or a tiny `stop-guest-tpm.sh` it calls) SIGTERMs the guest
swtpm pidfile **after** QEMU has exited, so NVRAM is flushed without an
in-flight QEMU client.

### Do not reset the DA counter

A lab `DictionaryAttackLockReset` would let reuse work while leaving the hard
kill in place. The symptom would vanish; the next person measuring after
`stop-guest.sh` would still see a bump and re-open the "enrollment is broken"
investigation. Fix the stop path instead.

### README

Replace the paragraph that blames enrollment with: hard qemu kill while swtpm
is attached increments `lockoutCounter`; graceful stop does not; enrollment
itself does not.

## Risks / Trade-offs

- **[Risk] QMP powerdown hangs under a wedged guest** → Mitigation: keep the
  existing timed SIGTERM/SIGKILL fallback after a deadline (e.g. 60s). Document
  that a fallback kill may still bump the counter once.
- **[Trade-off] stop takes longer** (ACPI + TCG) → Acceptable; e2e already waits
  minutes for boot.
- **[Risk] Someone still `kill -9`s qemu by hand** → Out of scope; README warns.

## Migration Plan

None. Lab scripts only.

## Open Questions

None for the maintainer. Implementation detail (exact QMP wait timeout) can be
chosen in tasks without further product decision.
