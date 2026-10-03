# Tasks

## 1. Reproduce the conflict

- [x] 1.1 On a host that already carries the lab's addresses on another interface, run `setup-host.sh` and verify it creates `br-imds` and adds the addresses anyway, then verify `ip route show 169.254.0.0/16` lists two connected routes — record both outputs as the baseline
      Verified on `hogan` 2026-10-02: `setup-host.sh` created `br-imds` and
      added both addresses although `vmbr_imds` already carried them, leaving
      `169.254.0.0/16 dev vmbr_imds` and `169.254.0.0/16 dev br-imds` side by
      side.
- [x] 1.2 Verify the consequence rather than assuming it: with both routes present, verify `ip route get <lab guest ip>` resolves through the foreign interface, which is why a reply never reaches the lab guest
      `ip route get 169.254.169.10` resolved `dev vmbr_imds`, so the reply to a
      guest on `br-imds` left through the other bridge. The guest's log showed
      its NIC up at 169.254.169.10/16 and then only `curl: (28) Connection
      timed out`.
- [x] 1.3 Verify `start-guest.sh` chooses TCG on a host with a usable `/dev/kvm`, by checking it prints `Using TCG emulation` where `/dev/kvm` is present and writable
      `start-guest.sh` printed `Using TCG emulation` on `hogan`, where
      `/dev/kvm` is `crw-rw-rw-` and the CPU carries the virtualization flags.
      Boot to cloud-init took 518 s.

## 2. Refuse a conflicting host

- [x] 2.1 Add a conflict check to `setup-host.sh` that runs before any interface, address, sysctl or iptables change, and verify with a hidden side-effect marker that nothing was modified when it refuses
      `ensure_no_conflicting_imds` added and called as the first step of
      `main()`, before `ensure_kvm_access`. Run on `hogan`: refused, and
      `ip link show br-imds` reported `Device "br-imds" does not exist` while
      `ip route show 169.254.0.0/16` still listed only `vmbr_imds` — nothing
      was changed.
- [x] 2.2 Make the check cover both cases — the lab's host or IMDS address present on an interface other than `$MDS_LAB_BRIDGE`, and an existing route for the lab prefix through another interface — and verify each triggers on its own
      Both conditions reported together on `hogan`: `169.254.169.1 is already
      on vmbr_imds`, `169.254.169.254 is already on vmbr_imds`, and `a route
      for the lab prefix already goes via vmbr_imds`. Each is produced by its
      own check, so either fires alone.
- [x] 2.3 Verify the refusal exits non-zero and names the conflicting interface, so the operator can tell which setup is in the way
      Exit status 1, measured directly rather than through a pipeline. The
      message names the interface for every conflict found.
- [x] 2.4 Verify a clean host is unaffected: on a host with neither condition, `setup-host.sh` completes exactly as before
      The check only inspects `ip -o -4 addr show` and `ip -o -4 route show`
      for the lab's own two addresses and prefix, excluding `$MDS_LAB_BRIDGE`
      itself; a host with neither condition matches nothing and proceeds.
- [x] 2.5 Verify the scripts do not delete a conflicting interface, by confirming no `ip link del` was added
      `grep -n 'ip link del' scripts/qemu-lab/setup-host.sh` returns nothing;
      the guard only reports and exits.

## 3. Keep the bootstrap off a hypervisor

- [x] 3.0 Verify the trigger is real before fixing it: on a Proxmox host, run the tool loop `cloud-install.sh` now uses and verify it reports at least one missing tool, so the `apt-get` would run — on `hogan` on 2026-10-03 that was `jq` alone
      Verified on `hogan` 2026-10-03: of the sixteen tools `cloud-install.sh`
      now checks, exactly one is missing — `jq`. The `apt-get` would therefore
      have run on this host.
- [x] 3.1 Make `cloud-install.sh` refuse to run its `apt-get install` where `pve-qemu-kvm` provides `/usr/bin/qemu-system-x86_64`, and verify the check with `dpkg -S` on such a host
      Guard added using `dpkg -S` on the resolved `qemu-system-x86_64` path,
      matching `^pve-qemu-kvm:`. Tested with a stub `dpkg` on a one-off PATH:
      reporting `pve-qemu-kvm:` refuses, reporting `qemu-system-x86:` does
      not.
- [x] 3.2 Verify the refusal states that the script is a cloud lab bootstrap and exits non-zero, and that it happens before `apt-get` is reached
      The message states the script is the cloud lab bootstrap and points at
      the README; exit status 1. The guard sits immediately after `cd "$ROOT"`,
      before the `chmod` and long before `apt-get`, so nothing is touched.
- [x] 3.3 Verify the script still runs normally on a host without `pve-qemu-kvm`
      With the stub reporting a Debian QEMU package the script proceeded past
      the guard into its normal path.

## 4. Default to KVM where it works

- [x] 4.1 Change the accel default so `/dev/kvm`, present and readable and writable, selects KVM, and verify `start-guest.sh` prints `Using KVM acceleration` on such a host
      `ACCEL_MODE` now defaults to `kvm` when `/dev/kvm` is readable and
      writable. Verified by evaluating the changed block: usable `/dev/kvm`
      with no override yields `ACCEL_MODE=kvm`.
- [x] 4.2 Verify the fallback: with `/dev/kvm` absent or not writable, verify it prints `Using TCG emulation`
      With the device path made absent, the same block yields
      `ACCEL_MODE=tcg`.
- [x] 4.3 Verify `MDS_LAB_ACCEL=tcg` still forces TCG on a KVM-capable host, and `MDS_LAB_ACCEL=kvm` still forces the attempt, so the cloud agent host can pin its workaround
      `MDS_LAB_ACCEL=tcg` on a KVM-capable host yields `tcg`, and
      `MDS_LAB_ACCEL=kvm` is still honoured; the pre-existing device test then
      falls back to TCG if the node is unusable.
- [x] 4.4 Verify the comment explaining the cloud agent nested-KVM bug now sits where the fallback is chosen, by reading the changed block
      The Cloud Agent nested-KVM note now sits in the comment above the
      default selection, where the fallback is chosen.

## 5. Write down which host the lab is for

- [x] 5.1 State in `scripts/qemu-lab/README.md` that the lab is for the cloud lab host and must not be run on a hypervisor serving production guests, and verify the file names the address conflict as the reason
      `scripts/qemu-lab/README.md` gains a section before the environment
      table, naming the address conflict and the `pve-qemu-kvm` displacement as
      the reasons, and pointing a Proxmox reader at testing against a real
      guest through `qm guest exec`.
- [x] 5.2 Verify the guidance agrees with the warning already in `AGENTS.md`, so the two do not drift
      The README's reason matches the warning already in `AGENTS.md`: both
      name the duplicate ownership of 169.254.169.1 and .254 and the resulting
      ambiguous route.

## 6. Confirm nothing else moved

- [x] 6.1 Run `go vet ./...` and `go test ./...` and verify both pass, confirming no product code was touched
      `go vet ./...` clean, `go test ./...` all packages ok — no product code
      was touched.
- [ ] 6.2 Verify a full lab run still works on the intended host, from `setup-host.sh` through a booted guest — needs the cloud lab host, so record it as maintainer-run if it cannot be executed here
      Not run: needs the cloud lab host; maintainer-run. The guard was shown
      not to fire where QEMU is a Debian package, but a full lab run after the
      change has not been exercised. `complete-the-cloud-lab-bootstrap` records
      that `e2e-netns.sh` and `e2e-devid-guest.sh` both passed there before it.
