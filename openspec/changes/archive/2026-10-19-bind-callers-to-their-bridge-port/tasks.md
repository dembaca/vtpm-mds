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

- [x] 1.1 **Maintainer, optional** — accept or overrule D-a to D-i in `design.md` (D-b own binary, D-f bridge as a unit argument, and fail open are the ones most likely to move), and verify the outcome is recorded there
- [x] 1.2 **Coordinator** — ACK the record and the delta on the bus before any code is written, and verify the ACK names this change
      **Done, 2026-10-19 16:11:** `agent-mac-vtpm-mds-42d61f`, "ACK … on the
      record `bind-callers-to-their-bridge-port` @ `be6437a`", draft PR #68,
      code after three additions: the live-migration window and the
      hand-written block beside the binder (both in Risks), and a rebase onto
      `main@c94bb2d` (#62) before the code.
- [x] 1.3 After #66 is merged, add the `debian-packaging` MODIFIED block for "Installing The Host Package Leaves systemd In Step" (both units) against the archived text in `openspec/specs/`, and verify with `openspec validate --all --strict`
      Written on 2026-10-19 before #66 merged, on the coordinator's request:
      the text is the one #66 archives, verified identical between
      `openspec/changes/restart-the-service-on-upgrade/specs/debian-packaging/spec.md`
      and `openspec/specs/debian-packaging/spec.md` on
      `origin/archive-restart-the-service-on-upgrade`. Added: both units
      restart alike, and configuring starts `vtpm-mds-bind` when `vtpm-mds`
      is active (also on `dpkg-reconfigure`), with two scenarios.
      `openspec validate --all --strict` passes. #66 was merged before
      this change was rebased onto `main@da0083a`; the block matches the text
      it archived in `openspec/specs/debian-packaging/spec.md` plus this
      change's additions (Fable re-check of `1f73bdb`).

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
      **After the Fable pre-review** (`6f82f86`): `Depends: …, nftables`;
      `postinst` starts `vtpm-mds-bind` when `vtpm-mds` is active; each
      bound port gets a third rule dropping 802.1Q/802.1ad frames; a failed
      apply is retried every tick and logged when its error changes, and a
      failed first apply exits non-zero; VM ids ≤ 9 digits and `net0`–`net99`
      so tap names fit IFNAMSIZ; `nft` calls time out after 10 s. New tests
      `TestAFailedApplyIsRetriedEveryTickAndLoggedOnce`,
      `TestRunFailsWhenTheFirstApplyFails`,
      `TestGuestNICsSkipsIDsThatCannotNameATap`, render with the tag rule;
      six more mutants (no tag rule, no retry, log every tick, `Run` ignoring
      the failure, unbounded id, unbounded index) all red. In the netns a
      tagged ARP with own frame MAC and forged sender is dropped by the tag
      rule (counter 1); **control without that rule: it reached the observer
      port**, so the finding was real. Package
      `0.2.0+git20261019.6f82f86cabb3`: `lintian` exit 0.
- [x] 4.2 Install on `hogan` (rebased onto current `main`, needs the maintainer's go); verify that the binder started by `postinst` (`vtpm-mds` is active) applied its table **under the unit's sandbox** — `nft` as `vtpm-mds` with ambient `CAP_NET_ADMIN`, `SystemCallFilter=~@privileged @resources`, `MemoryDenyWriteExecute`, `ProtectKernelModules`, which the netns runs did not have — by its `Applied table` journal line and `systemctl is-active vtpm-mds-bind`; verify the generated table equals the hand-written one for VM 399 plus a pair for VM 300's `tap300i1`, and nothing for VM 106
      Done 2026-10-19 19:38 on `hogan` with the maintainer's go: built `4b54732`
      (rebased onto `main@da0083a`, clean tree) → `0.2.0+git20261019.4b547320d1c7`,
      installed through `hogan-lab dpkg-install --force-confold` (dpkg calls it a
      "downgrade" only because the hash sorts lower). `postinst` enabled
      `vtpm-mds-bind` and started it because `vtpm-mds` was active:
      `vtpm-mds` try-restart PID 3270635 → 3361048, `Server stopped gracefully`
      19:38:25.002 → `Starting server` 19:38:25.268 (gap 0.27 s), `DevID CA key
      from systemd credential`, 3 VMs/5 MACs; binder PID 3361065 logged
      `Applied table bridge vtpm_mds_bind: 2 guest ports on vmbr_imds` at
      19:38:25.463, `is-active` → `active` for both. **Sandbox:** `/proc/<pid>/status`
      of the binder: Uid/Gid 987 (`vtpm-mds`), Groups 33 987, CapInh/Prm/Eff/Bnd/Amb
      all `0x1000` = `cap_net_admin` only, `NoNewPrivs: 1`, `Seccomp: 2`; the
      `nft` it ran applied under that. The table (`hogan-lab nft list ruleset`,
      and `/run/vtpm-mds-bind/vtpm_mds_bind.nft`, `vtpm-mds:vtpm-mds`) is the
      hand-written VM 399 pair plus the tag rule, and the same three rules for
      `tap300i1` with VM 300's MAC; nothing for VM 106. The rest of the ruleset
      is byte-identical to before the install. VM 399's EK pin was not touched
      by the package (`postinst` only sets the mode of `/var/lib/vtpm-mds`);
      the agent cannot read `ek-pins/` (0750), so its content was not re-read.
- [x] 4.3 Repeat the MAC test of 2026-10-19 in VM 399 (foreign MAC → timeout and `ether saddr` counter rises; own MAC → `200`), VM 399 restored afterwards
      Done 19:38 (`scratchpad/mac399b.py` via `qm guest exec 399`): before
      `instance-id 200 i-399`; `eth1` → `02:00:00:00:03:99`: timeout after 2 s,
      `ether saddr` counter 0 → `packets 1`; restored to `bc:24:11:06:1d:b2`:
      first `/health` after 6.0 s, then `instance-id 200 i-399`.
- [x] 4.4 Test the ARP-sender rule on its own: from VM 399, with `eth1` keeping its own MAC, send an ARP packet whose sender hardware address is foreign (raw socket in `python3`, no `scapy` needed); verify the `arp saddr ether` counter rises and the host's neighbour entry for `169.254.169.10` keeps VM 399's MAC (reading counters needs the maintainer, or `hogan-lab` widened to list `bridge vtpm_mds_bind`)
      Done 19:39: from VM 399, `eth1` keeping its own MAC, three ARP replies
      sent on an `AF_PACKET` socket with frame source = own MAC and ARP sender
      = `02:00:00:00:03:99` for `169.254.169.10`. `arp saddr ether` counter
      0 → `packets 3 bytes 84`, `ether saddr` unchanged at 1; the host's entry
      stayed `169.254.169.10 lladdr bc:24:11:06:1d:b2`; `/health` 200 before and
      after. No control without the rule on `hogan` (the netns run in 2.3 had
      one).
- [x] 4.5 Restart `nftables` (maintainer), verify the table is restored within one interval with one log line
      Done 19:39 by the agent (`hogan-lab systemctl restart nftables`, allowed
      since 2026-10-16): `nftables` finished 19:39:34.797, having reloaded only
      the hand-written VM 399 pair; the binder logged `Table bridge
      vtpm_mds_bind was changed outside the binder, restoring it` at
      19:39:35.474 followed by its `Applied table …: 2 guest ports` line, so one
      restore, logged once (the cause line plus the apply line), 0.68 s after the
      reload; 6 rules again, both units `active`, same binder PID. Between the
      reload and the restore only VM 399 was protected, as Risks says.
- [x] 4.6 Say what was not verified (e.g. a migration in and its fail-open window on the target, a hot-plugged NIC, `firewall=1` beyond reading the rules for VM 300, LXC)
      Not verified on `hogan`: a migration in and its fail-open window on the
      target (VM 399 cannot migrate); a hot-plugged NIC; a new VM appearing
      (netns only, 3.1); `firewall=1` traffic from VM 300 (only its rules were
      read; VM 300 is not ours to test); LXC on the bridge (none exists); a
      first check that cannot read the configurations exiting non-zero (Fable
      re-check of `1f73bdb`; `TestRunFailsWhenTheFirstReadFails`, red on the
      old `Run`, no reinstall on `hogan` for it); a
      failing apply under the real unit and the `StartLimitBurst` path (unit
      tests only); `postrm remove` deleting the table (would remove the
      package); the upgrade from a version without the binder on a node whose
      daemon is stopped (`No running daemon, no binder`).

## 5. Hand over

- [x] 5.1 Give the coordinator the `README.md` line for the new unit
      **Done, 2026-10-19 19:40** (hogan-d3e6, in the #68 thread); the coordinator
      put it into the README feature table when archiving.
- [ ] 5.2 **Maintainer** — remove the hand-written VM 399 block from `/etc/nftables.d/vtpm-mds.nft` in the Ansible role (keep the `inet vtpm_mds` idempotency lines), and make the role install the package on every node with vTPM guests; verify on `hogan` that the file no longer names `tap399i1` and the generated table is in place

## 6. Archive

- [x] 6.1 After merge, the coordinator archives with `openspec archive bind-callers-to-their-bridge-port` and verifies `openspec validate --all --strict` stays clean
      **Done, 2026-10-19:** archived by the coordinator after PR #68 merged
      (`01fd5f1`); 1.1 ticked because the maintainer overruled none of D-a to
      D-i and merged as recorded (fail open stays). **5.2 is still open** and
      is the maintainer's: until the Ansible role drops the VM 399 block, two
      writers own `bridge vtpm_mds_bind` (design, Risks).
