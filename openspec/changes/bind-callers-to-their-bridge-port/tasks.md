# Tasks

Written on 2026-10-19 against `main` at `e034138` (rebased onto `c94bb2d`,
which contains `reload-when-guest-configs-change`, before the code) by
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

- [x] 2.1 Add a reader that returns, for each local guest config, every `net<n>` with its index, model, MAC and `bridge=`, live section only (stop at the first `[`), any model; verify with tests on constructed `/etc/pve` trees: `virtio` and `e1000`, `firewall=1`, a NIC on another bridge, a `[PENDING]` NIC, a `.conf` of another node
      `internal/proxmox/nics.go`: `GuestNICs` (any `<model>=<MAC>` and the
      older `model=…,macaddr=…`; a line without a valid MAC yields `MAC: ""`),
      `NIC.Tap()`. Tests: `TestGuestNICsReadsEveryModelWithIndexAndBridge`
      (`virtio`, `e1000`, `rtl8139` via `macaddr`, an invalid MAC,
      `firewall=1`, `vmbr0`, another node's `.conf`, numeric sort 7 < 42 < 300
      < 399), `TestGuestNICsReadsTheLiveSectionOnly` (`[PENDING]`, snapshot),
      `TestGuestNICsFailsWithoutALocalLink`.
- [x] 2.2 Render the table text (D-c, D-d) from a port list and a bridge name, sorted, in declare/delete/define form; verify with golden-file tests (no NIC, one, several, uppercase MAC lowercased) and that unchanged input renders identical bytes
      `internal/bridgebind/render.go`. The expected text is inline in
      `render_test.go` rather than in files: `TestRenderWithoutGuestPortsIsAnEmptyChain`,
      `TestRenderBindsEachPortOnTheBridgeToItsMAC` (a port without a valid MAC
      gets `iifname … counter drop`, D-e; the `vmbr0` NIC gets nothing; a
      second render is identical). Lowercasing is `GuestNICs`' job and tested
      there.
- [x] 2.3 Verify the rendered text loads with `nft -c -f` and, in an `unshare -rn` netns, loads twice to one copy of each rule; in the same netns, build the `firewall=1` layout (a veth "tap" as port of `fwbr`, linked by a veth pair to a second bridge) and verify a frame with a foreign source MAC sent from the "tap" raises the drop counter and never reaches the second bridge
      Done 2026-10-19 in `unshare -rn` (nft 1.1.3) with the script
      `vtpm-mds-bind -print` rendered from a constructed tree (VM 399 plain,
      VM 300 `firewall=1`): `DRYRUN-OK`, two loads → 4 rules. Layout: `tap399i1`
      on `vmbr_imds`; `tap300i1` on `fwbr300i1`, linked by `fwln300i1`/`fwpr300p1`
      to `vmbr_imds`; an observer port on `vmbr_imds`. Raw frames from the
      guest ends: own MAC behind `fwbr` → seen; foreign behind `fwbr` → not
      seen; VM 399 own → seen, foreign → not seen; ARP with own frame MAC and
      own sender → seen; ARP with own frame MAC and **forged sender** → not
      seen, `arp saddr ether` counter `packets 1`. So `firewall=1` NICs are
      bound on their tap (Context, now verified in a netns) and the ARP rule
      works on its own there; on `hogan` it is still task 4.4.

## 3. Binder

- [x] 3.1 Add the loop (D-g): fingerprint per tick, render and apply on change, list-and-compare per tick and re-apply when missing or different, one log line per apply, none while unchanged; `nft` behind a seam; verify with tests for: start applies, unchanged ticks do nothing, changed source re-applies, a vanished or altered table is restored and logged once, a failed apply is logged once and retried on the next change or drift
      `internal/bridgebind/binder.go` (`Binder.Tick`, `Run`, `NFT` seam, `Exec`
      with `nft -s list` so counters never count as drift). Tests in
      `binder_test.go`: start applies; unchanged ticks neither apply nor log;
      an added guest and a changed MAC are applied; a change outside the
      bridge is not; a missing and an altered table are restored, each logged
      once; a failing apply is logged once over four ticks, never reported as
      "missing", and retried after a config change; unreadable configs keep
      the last table, log once, and the table is still restored if it
      vanishes; the run-dir script; `Run` checks at start and returns on
      cancel. Against real `nft` in a netns: applied at start, `nft flush
      ruleset` → "is missing, restoring it" 2.6 s later, a new VM 400 → 6
      rules within one interval, `SIGTERM` → exit 0 with the table left.
- [x] 3.2 For each new test, run it against mutated code and verify it fails
      Thirteen mutants: no ARP rule, no bridge filter, a MAC-less port left
      open, no drift restore, a failure not remembered, apply on every
      fingerprint change, a read error dropping the table, no run-dir script,
      `Run` not checking at start, reading past the live section, `virtio`
      only, `macaddr` ignored, no numeric sort. All red after the `Run` test
      was sharpened (an interval of one hour, so only the check at start can
      apply). Not covered by a unit test: `Exec`'s `-s`; the netns run would
      show it as a re-apply on every tick.
- [x] 3.3 Add `cmd/vtpm-mds-bind` with `-bridge`, `-pve-root`, `-version`; verify `go vet ./...`, `go test ./...`, `gofmt -l`
      Plus `-nft`, `-run-dir` (default `$RUNTIME_DIRECTORY`) and `-print`.
      `go vet ./...` clean, `go test -race -count=1 ./...` all `ok`, `gofmt -l`
      clean on every new file.

## 4. Package and verify on hogan

- [x] 4.1 Add the unit (D-i), install list, `postrm remove` deleting the table, man page; verify `dpkg-deb -c`, `lintian --fail-on error,warning`, `systemd-analyze verify` and `systemd-analyze security vtpm-mds-bind` on the built package, and that `vtpm-mds.service` is unchanged
      `0.2.0+git20261019.84c2fcff8f7d` (clean tree): `dpkg-deb -c` adds
      `/usr/sbin/vtpm-mds-bind`, `/usr/lib/systemd/system/vtpm-mds-bind.service`,
      `vtpm-mds-bind.8.gz`; the generated `postinst` enables both units and
      `try-restart`s both on upgrade, `prerm` stops both on remove, `postrm
      remove` deletes the table. `lintian --fail-on error,warning` exit 0 for
      both packages (after `nft` is called by name in `postrm`).
      `systemd-analyze verify --man=no` clean (unit copied with `ExecStart`
      pointing at the extracted binary); `systemd-analyze security --offline`
      **1.3 OK**. `git diff c94bb2d -- debian/vtpm-mds.service main.go imds
      internal/server` is empty. Extracted binary: `vtpm-mds-bind
      0.2.0+git20261019.47eb89f59df6` on the first build.
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
