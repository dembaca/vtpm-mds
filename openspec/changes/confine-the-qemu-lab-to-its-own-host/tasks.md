# Tasks

## 1. Reproduce the conflict

- [ ] 1.1 On a host that already carries the lab's addresses on another interface, run `setup-host.sh` and verify it creates `br-imds` and adds the addresses anyway, then verify `ip route show 169.254.0.0/16` lists two connected routes — record both outputs as the baseline
- [ ] 1.2 Verify the consequence rather than assuming it: with both routes present, verify `ip route get <lab guest ip>` resolves through the foreign interface, which is why a reply never reaches the lab guest
- [ ] 1.3 Verify `start-guest.sh` chooses TCG on a host with a usable `/dev/kvm`, by checking it prints `Using TCG emulation` where `/dev/kvm` is present and writable

## 2. Refuse a conflicting host

- [ ] 2.1 Add a conflict check to `setup-host.sh` that runs before any interface, address, sysctl or iptables change, and verify with a hidden side-effect marker that nothing was modified when it refuses
- [ ] 2.2 Make the check cover both cases — the lab's host or IMDS address present on an interface other than `$MDS_LAB_BRIDGE`, and an existing route for the lab prefix through another interface — and verify each triggers on its own
- [ ] 2.3 Verify the refusal exits non-zero and names the conflicting interface, so the operator can tell which setup is in the way
- [ ] 2.4 Verify a clean host is unaffected: on a host with neither condition, `setup-host.sh` completes exactly as before
- [ ] 2.5 Verify the scripts do not delete a conflicting interface, by confirming no `ip link del` was added

## 3. Keep the bootstrap off a hypervisor

- [ ] 3.1 Make `cloud-install.sh` refuse to run its `apt-get install` where `pve-qemu-kvm` provides `/usr/bin/qemu-system-x86_64`, and verify the check with `dpkg -S` on such a host
- [ ] 3.2 Verify the refusal states that the script is a cloud lab bootstrap and exits non-zero, and that it happens before `apt-get` is reached
- [ ] 3.3 Verify the script still runs normally on a host without `pve-qemu-kvm`

## 4. Default to KVM where it works

- [ ] 4.1 Change the accel default so `/dev/kvm`, present and readable and writable, selects KVM, and verify `start-guest.sh` prints `Using KVM acceleration` on such a host
- [ ] 4.2 Verify the fallback: with `/dev/kvm` absent or not writable, verify it prints `Using TCG emulation`
- [ ] 4.3 Verify `MDS_LAB_ACCEL=tcg` still forces TCG on a KVM-capable host, and `MDS_LAB_ACCEL=kvm` still forces the attempt, so the cloud agent host can pin its workaround
- [ ] 4.4 Verify the comment explaining the cloud agent nested-KVM bug now sits where the fallback is chosen, by reading the changed block

## 5. Write down which host the lab is for

- [ ] 5.1 State in `scripts/qemu-lab/README.md` that the lab is for the cloud lab host and must not be run on a hypervisor serving production guests, and verify the file names the address conflict as the reason
- [ ] 5.2 Verify the guidance agrees with the warning already in `AGENTS.md`, so the two do not drift

## 6. Confirm nothing else moved

- [ ] 6.1 Run `go vet ./...` and `go test ./...` and verify both pass, confirming no product code was touched
- [ ] 6.2 Verify a full lab run still works on the intended host, from `setup-host.sh` through a booted guest — needs the cloud lab host, so record it as maintainer-run if it cannot be executed here
