# Tasks

Written on 2026-10-19 against `main` at `e034138` by
`agent-vtpm-hogan-d3e6cbc82`, at the coordinator's assignment (E1(a), "after
#62"). **Code only after the coordinator's ACK of this record, and after #62
is merged** (`proxmox.SourceFingerprint`). Files: new `cmd/vtpm-mds-bind/`,
new `internal/bridgebind/`, a NIC reader in `internal/proxmox/` (not
`vmconfig.go`'s inventory parser), `debian/` (install list, new unit, `rules`,
`postrm`, man page). Not `internal/server/`, not `imds/`, not
`internal/devid/` (#65). The root `README.md` line goes to the coordinator.

## 1. Decide

- [ ] 1.1 **Maintainer, optional** — accept or overrule D-a to D-i in `design.md` (D-b own binary, D-f bridge as a unit argument, and fail open are the ones most likely to move), and verify the outcome is recorded there
- [x] 1.2 **Coordinator** — ACK the record and the delta on the bus before any code is written, and verify the ACK names this change
      **Done, 2026-10-19 16:11:** `agent-mac-vtpm-mds-42d61f`, "ACK … on the
      record `bind-callers-to-their-bridge-port` @ `be6437a`", draft PR #68,
      code after three additions: the live-migration window and the
      hand-written block beside the binder (both in Risks), and a rebase onto
      `main@c94bb2d` (#62) before the code.
- [ ] 1.3 After #66 is merged, add the `debian-packaging` MODIFIED block for "Installing The Host Package Leaves systemd In Step" (both units) against the archived text in `openspec/specs/`, and verify with `openspec validate --all --strict`

## 2. Port list

- [ ] 2.1 Add a reader that returns, for each local guest config, every `net<n>` with its index, model, MAC and `bridge=`, live section only (stop at the first `[`), any model; verify with tests on constructed `/etc/pve` trees: `virtio` and `e1000`, `firewall=1`, a NIC on another bridge, a `[PENDING]` NIC, a `.conf` of another node
- [ ] 2.2 Render the table text (D-c, D-d) from a port list and a bridge name, sorted, in declare/delete/define form; verify with golden-file tests (no NIC, one, several, uppercase MAC lowercased) and that unchanged input renders identical bytes
- [ ] 2.3 Verify the rendered text loads with `nft -c -f` and, in an `unshare -rn` netns, loads twice to one copy of each rule; in the same netns, build the `firewall=1` layout (a veth "tap" as port of `fwbr`, linked by a veth pair to a second bridge) and verify a frame with a foreign source MAC sent from the "tap" raises the drop counter and never reaches the second bridge

## 3. Binder

- [ ] 3.1 Add the loop (D-g): fingerprint per tick, render and apply on change, list-and-compare per tick and re-apply when missing or different, one log line per apply, none while unchanged; `nft` behind a seam; verify with tests for: start applies, unchanged ticks do nothing, changed source re-applies, a vanished or altered table is restored and logged once, a failed apply is logged once and retried on the next change or drift
- [ ] 3.2 For each new test, run it against mutated code and verify it fails
- [ ] 3.3 Add `cmd/vtpm-mds-bind` with `-bridge`, `-pve-root`, `-version`; verify `go vet ./...`, `go test ./...`, `gofmt -l`

## 4. Package and verify on hogan

- [ ] 4.1 Add the unit (D-i), install list, `postrm remove` deleting the table, man page; verify `dpkg-deb -c`, `lintian --fail-on error,warning`, `systemd-analyze verify` and `systemd-analyze security vtpm-mds-bind` on the built package, and that `vtpm-mds.service` is unchanged
- [ ] 4.2 Install on `hogan` (rebased onto current `main`, needs the maintainer's go); verify the generated table equals the hand-written one for VM 399 plus a pair for VM 300's `tap300i1`, and nothing for VM 106
- [ ] 4.3 Repeat the MAC test of 2026-10-19 in VM 399 (foreign MAC → timeout and `ether saddr` counter rises; own MAC → `200`), VM 399 restored afterwards
- [ ] 4.4 Test the ARP-sender rule on its own: from VM 399, with `eth1` keeping its own MAC, send an ARP packet whose sender hardware address is foreign (raw socket in `python3`, no `scapy` needed); verify the `arp saddr ether` counter rises and the host's neighbour entry for `169.254.169.10` keeps VM 399's MAC (reading counters needs the maintainer, or `hogan-lab` widened to list `bridge vtpm_mds_bind`)
- [ ] 4.5 Restart `nftables` (maintainer), verify the table is restored within one interval with one log line
- [ ] 4.6 Say what was not verified (e.g. a migration in and its fail-open window on the target, a hot-plugged NIC, `firewall=1` beyond reading the rules for VM 300, LXC)

## 5. Hand over

- [ ] 5.1 Give the coordinator the `README.md` line for the new unit
- [ ] 5.2 **Maintainer** — remove the hand-written VM 399 block from `/etc/nftables.d/vtpm-mds.nft` in the Ansible role (keep the `inet vtpm_mds` idempotency lines), and make the role install the package on every node with vTPM guests; verify on `hogan` that the file no longer names `tap399i1` and the generated table is in place

## 6. Archive

- [ ] 6.1 After merge, the coordinator archives with `openspec archive bind-callers-to-their-bridge-port` and verifies `openspec validate --all --strict` stays clean
