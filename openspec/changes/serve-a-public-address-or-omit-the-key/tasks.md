# Tasks

Implementation started 2026-10-16 against `main` at `7db05db`, on branch
`implement-serve-a-public-address`.

**A contradiction in this change's own spec delta had to be repaired first.**
The `ADDED` requirement still carried a paragraph from the requirement it
replaces — "`GET /latest/meta-data/public-ipv4` SHALL respond `404` … for every
caller, because no source of a public address is implemented. The path SHALL
nonetheless remain listed in the index." That is the defect, quoted verbatim in
the `REMOVED` section's own **Reason** as the thing being removed, and it
contradicts both the clause above it and the scenario
`Public address is served when declared`. A self-contradictory contract cannot
be implemented against, so it was deleted rather than worked around.

**No longer blocked.** The open question in `design.md` — *where does the
address come from* — was answered by the maintainer on 2026-10-05: **option A**,
an operator-declared per-VM value in the inventory. Sections 2 and 3 stand as
written, and the spec delta needs no change because it was written for that
option.

**Ordering.** The prerequisite is met: `fix-client-ip-fallback` was implemented
and merged as PR #20 (`main` at `396b05f`), and it left `getPublicIP` and
`HandleMetaDataIndex` untouched. This change is free to proceed against that
tree.

## 1. Record the baseline

- [x] 1.1 Read `GET /latest/meta-data/public-ipv4` from VM 399 against the installed daemon and verify it answers `404` with an **empty** body and `Content-Type: text/plain`, not the catch-all's `404 page not found`
      **Done** 2026-10-16 from inside VM 399 against the installed
      `0.2.0+git20261005.5a60c877cd9c`:
      `public-ipv4` → `404`, `Content-Type: text/plain`, body length **0**.
      Catch-all for comparison, same run: `/latest/no-such-thing` → `404`,
      `Content-Type: text/plain; charset=utf-8`, body `b'404 page not found\n'`.
      The two differ in **both** the body and the Content-Type — the charset
      parameter is present only on the catch-all's. The delta promises the
      first shape survives this change.
      Use `sudo hogan-lab qm guest exec 399 …`; VM 399 has `python3` and no
      `curl`. The distinction matters: the delta promises this exact response
      shape survives the change.
- [x] 1.2 Read `GET /latest/meta-data/` from the same guest and verify the index is six lines and includes `public-ipv4` — the key that 1.1 just proved unreadable
      **Done**, same run as 1.1: `200`, 71 bytes, six lines —
      `instance-id`, `local-hostname`, `local-ipv4`, `public-ipv4`,
      `placement/`, `services/`. The fourth is the key 1.1 just proved
      unreadable, which is the defect in one reading.
      This pair is the defect in one screenshot; record both outputs together.

## 2. Add the source — blocked on the maintainer's choice

- [x] 2.1 Verify the implementation reads the address from the bound VM record and from nowhere else — the maintainer chose option A on 2026-10-05, an operator-declared per-VM value
      **Done.** `imds.PublicIPv4` reads `RawConfig["public-ipv4"]` from the
      record `inventory.GetVMConfigFromRequest` returns, and touches nothing
      else — no `RemoteAddr`, no header, no observed address. Proven by
      `TestPublicIPv4IsNeverDerivedFromTheConnection`, which sets the peer to
      `203.0.113.7` and `X-Forwarded-For` to `203.0.113.9` with nothing
      declared and requires `404` both times.
      Answered, so this task is a check rather than a decision. Options B
      (Proxmox `ipconfig0`) and C (guest agent) were rejected because a public
      address is not something this service can observe; see `design.md`.
- [x] 2.2 Implement the chosen source and verify a VM with a declared address serves it while a VM without one still answers `404` with an empty body
      For option A, `VMConfig.RawConfig` already exists and is documented as
      "Optional key/value metadata", so no struct change is needed.
      **Done, both halves.** The reading side:
      `TestPublicIPv4ServesTheDeclaredAddress` serves `203.0.113.7` for a VM
      that declares it, answers `404` with an empty body for one that does not,
      and treats `not-an-address`, `2001:db8::1`, `203.0.113.7/32`,
      `203.0.113.999` and a blank value as absent rather than serving them.
      **With the Proxmox backend that is already the whole feature**, with no
      spec change at all: `vm-inventory`'s
      `Derive VM Records From Proxmox Guest Configuration` says every
      `key: value` line of a guest config is "stored key/value in the record's
      raw configuration map", so a `public-ipv4: 203.0.113.7` line in
      `/etc/pve/qemu-server/<vmid>.conf` reaches `RawConfig["public-ipv4"]`
      today.
      **With the YAML backend it was forbidden until this record gained a
      `vm-inventory` delta.** `Load VM Records From The YAML Inventory` said
      entries support "`id`, `name`, `macs` and `ek_sha256`, and keys outside
      that set SHALL be ignored", and that the raw configuration map "SHALL
      contain exactly one key, `name`" — both of which a `public_ipv4:` field
      contradicts. The gap was in this record, not in the spec: the Impact
      section named `internal/inventory/inventory.go` while no delta covered
      it. **The maintainer decided on 2026-10-17** that the YAML inventory may
      declare a public IPv4 so it is on par with the Proxmox one, so the delta
      went into this change rather than a separate one.
      `TestLoadYAMLPublicIPv4` proves the declared address reaches
      `RawConfig["public-ipv4"]` — the same key the Proxmox backend produces,
      so a reader need not know which backend built the record — while an
      entry that declares nothing carries **no such key at all**, which is
      distinguishable from an empty one.
      `TestLoadYAMLAcceptsAnAbsentOrEmptyPublicIPv4` keeps the field optional,
      so every inventory written before this change still loads.
      **Both halves are verified here at handler and loader level only.** The
      daemon-level checks are 5.2 for the YAML source and 5.3 for the Proxmox
      one, and neither has run. This tick means the code does what the delta
      says when exercised directly — not that anyone has watched the service do
      it.
- [x] 2.3 Validate a declared address at inventory load time and verify a malformed value fails the load loudly rather than being served
      Follow the precedent `fail-fast-on-unusable-configuration` set: an
      unusable value stops the service instead of degrading silently. Verify a
      non-IPv4 string and an IPv6 string are both rejected.
      **Done for the YAML inventory, and deliberately not for Proxmox.**
      `LoadYAML` parses a declared `public_ipv4` and returns
      `inventory entry "100": public_ipv4 "…" is not an IPv4 address`,
      producing **no inventory at all** rather than a partial one.
      `TestLoadYAMLRejectsAMalformedPublicIPv4` covers five shapes, each
      required to fail and to name both the entry and the field:
      `not-an-address`, `2001:db8::1` (a valid address, but not IPv4),
      `203.0.113.7/32` (a prefix), `203.0.113.999` (out of range) and
      `203.0.113.7:80` (host and port). The load fails loudly and the value is
      never served, which is what this task's checkbox line asks.
      **Corrected 2026-10-17 — this note used to claim "a typo stops the
      service". It does not, and the guidance above ("an unusable value stops
      the service") is not met.** 5.2 showed it on a running daemon: the load
      fails, but `Load The Cache At Startup And Refresh It On SIGHUP` says a
      startup load failure "SHALL NOT prevent the service from starting; the
      service then runs with an empty cache, so every caller is unbound". That
      requirement predates this change and the daemon follows it. The claim was
      mine and rested on a unit test, which cannot show what a daemon does at
      startup.
      **What the composition actually does depends on the mode, and neither
      outcome is loud:** one malformed `public_ipv4` in one VM's entry empties
      the inventory, so every caller is unbound, while `/health` still answers
      `200`. Under the default `mds.require_vm_identity: true` that is
      **fail-closed** — every identity read is refused with `404`, so no guest
      is handed a wrong identity, but every guest loses its metadata and nothing
      says why. Under `require_vm_identity: false`, the migration aid, it is
      **neither closed nor loud**: every guest falls back to the invented
      `i-<peer-address>` id instead of its own VM id, served with `200`. The
      distinction was made by `agent-mac-vtpm-mds-43fc22`; this note first said
      only that every VM "loses its identity", which is right for the default
      mode and wrong for the other. And
      it contradicts this very note's own argument below — the paragraph on
      Proxmox rejects failing a whole host's inventory over one typo as "a worse
      outcome than the typo", and the YAML half then does exactly that. The
      better designs are to reject only the field, or to refuse to start on any
      inventory error; choosing between them is the maintainer's, and it is
      deferred with the rest of `public-ipv4` to
      `declare-per-vm-metadata` (PR #33), which already lists failure behaviour
      as an open question. The YAML inventory is the lab's backend; production
      on `hogan` reads Proxmox, so the blast radius is the lab.
      **The Proxmox backend validates nothing, on purpose.** It stores every
      `key: value` line of a guest config verbatim, as its own requirement
      demands, so a malformed `public-ipv4:` line there is carried into the
      record and rejected by `PublicIPv4` where it is read. Failing a whole
      host's inventory over one typo in a file this service does not own would
      deny metadata to **every** VM on that host — a worse outcome than the
      typo, and a different posture from the YAML file, which this project owns
      and an operator writes by hand. The asymmetry is written into the
      `vm-inventory` delta rather than left for a reader to discover.
- [x] 2.4 Verify a declared address is never derived from the connection — assert that a caller whose peer is `203.0.113.7` but whose VM declares nothing still gets `404`
      **Done.** `TestPublicIPv4IsNeverDerivedFromTheConnection` asserts exactly
      that peer and that status, with an empty body and
      `Content-Type: text/plain`, and repeats it with `X-Forwarded-For` set.
      This is the failure this change could have shipped with every other test
      still green.
      This is the one way the change could go wrong without any test failing:
      quietly falling back to the peer address would look correct in a lab
      where the two happen to match.

## 3. Make the index caller-dependent

- [x] 3.1 Make `HandleMetaDataIndex` list `public-ipv4` only when the caller has a declared address, and verify the two new index scenarios both pass
      **Done.** The handler builds the three always-listed keys, appends
      `public-ipv4` when `PublicIPv4(r)` is non-empty, then appends
      `placement/` and `services/` so the order the index has always had is
      kept. `TestIndexListsPublicIPv4OnlyWhenDeclared` covers both scenarios:
      five lines without a declaration, six with, and the five always-present
      keys checked in each.
      The handler currently builds a fixed slice and consults nothing. It needs
      the same bound-VM lookup the other handlers already do.
- [x] 3.2 Verify the index is rendered per request and never cached or shared between callers, by reading it as two different VMs in the same process and confirming the two listings differ
      **Done.** `TestIndexIsRenderedPerRequest` reads the index as VM 100 with
      a declared address, as VM 101 without, and as VM 100 again, in one
      process: the first two must differ and the first and third must match.
      A cache keyed on nothing would fail the first assertion; one that never
      refreshed would fail the second.
      This is the property the delta explicitly requires. Nothing caches it
      today; the task exists so that stays true.
- [x] 3.3 Verify an unbound caller still receives an index, without `public-ipv4`, and is not refused
      **Done.** `TestUnboundCallerStillGetsAnIndex`: `200` with the five
      always-listed keys and no `public-ipv4`. `RequireVMIdentity` leaves the
      index open on purpose, because the index asserts no identity, and that
      is unchanged here.
      `Refuse An Instance Identity To Callers With No VM Record` deliberately
      leaves the index open, because it asserts no identity. That must not
      change here.

## 4. Confirm nothing else moved

- [x] 4.1 Verify `local-hostname`, `local-ipv4`, `placement/availability-zone`, `services/domain`, the unknown-key subtree behaviour and the `301` on `/latest/meta-data` are byte for byte unchanged
      Every one of these is carried verbatim into the replacement requirement,
      so any difference is a regression, not a decision.
      **Done at source level; re-checked on the wire under 5.2.**
      `git diff origin/main...HEAD -- imds/handlers.go` has hunks in exactly
      two functions, `HandleMetaDataIndex` and `HandlePublicIPv4`. Zero changed
      lines mention `HandleLocalHostname`, `HandleLocalIPv4`,
      `HandleAvailabilityZone` or `HandleDomain`, and no changed line touches
      `LocalIPv4`'s body — the third hunk header names it only because
      `getPublicIP` sat directly beneath it. Route registration is untouched,
      so the subtree behaviour for unknown keys and the `301` on
      `/latest/meta-data` cannot have moved: both come from the mux, not from a
      handler.
      Their tests pass unchanged: `TestHandleLocalIPv4`,
      `TestHandleLocalIPv4_IPv6Peer`, `TestHandleLocalIPv4_IPv4MappedPeer`,
      `TestHandleAvailabilityZone`.
- [x] 4.2 Verify `getAvailabilityZone` and `getRegion` are untouched, including their `TODO` comments
      **Done.** `git diff imds/handlers.go` shows `func getAvailabilityZone`
      only as an unchanged context line; neither function nor its
      `TODO: Extract from Proxmox cluster config` comment appears on a `+` or
      `-` line. They return the fixed values the spec pins and are out of
      scope; touching them would repeat the `-version` mistake AGENTS.md
      records.
      They carry the same comment as `getPublicIP` but return the real fixed
      values the spec pins. They are explicitly out of scope; touching them
      would repeat the `-version` mistake AGENTS.md records.
- [x] 4.3 Verify `go vet ./...` and `go test ./...` are both clean and report the output
      **Done** 2026-10-16, Go 1.24.4. `go vet ./...` exits 0 with no output.
      `go test ./...`: `ok imds 0.032s`, `ok internal/config`,
      `ok internal/devid`, `ok internal/inventory`, `ok internal/server
      0.108s`; `no test files` for the root package, `cmd/devid-enroll` and
      `internal/proxmox`. No failures.
      `gofmt -l` still flags `imds/handlers_test.go`, as it did before this
      change — checked by stashing and re-running. Every hunk `gofmt -d`
      reports is at or below line 750; this change's additions start at line
      900. Pre-existing trailing whitespace, deliberately not reformatted:
      unrelated churn.

## 5. Validate and archive

- [x] 5.1 Verify `openspec validate --all --strict` exits 0 on `hogan`, where the CLI is at `/usr/local/bin/openspec`
      Also run `scripts/openspec-validate.py --strict`: it is a floor, not a
      replacement, but it enforces one rule the CLI does not — every checkbox
      line must contain its own verification.
      **Done** 2026-10-17 on this branch, rebased onto `main` at `bb27143`:
      `openspec validate --all --strict` → `Totals: 7 passed, 0 failed` — five
      living specs plus this change and
      `remove-the-constant-instance-identity-signature`.
      `scripts/openspec-validate.py --strict` reports `all tasks ticked` twice,
      once for each of those two changes, which is correct: both still have
      open tasks by design.
**5.2 was one task and is now two, 5.2 and 5.3; the old 5.3 became 5.4.**

They were briefly numbered `5.2a` and `5.2b`, which **the fallback validator
cannot see at all**: `scripts/openspec-validate.py` matches
`^- \[([ xX])\] (\d+[a-z]*\.\d+) (.+)$`, where the letter may only sit
between the two numbers. A `5.2a` line is therefore not counted as a task, is
never reported as unticked, and never checked for stating its own
verification — it is invisible rather than wrong, which is worse. Caught by
`agent-mac-vtpm-mds-52ae2f`, who noticed the validator reporting 13/14 against
a record that says 13/16. It named only VM 399, which covers the
Proxmox source and leaves the YAML source — the half the maintainer decided to
add on 2026-10-17 — verified by unit tests alone. A test is not a daemon, and
`AGENTS.md` already says where each half belongs: *"A cloud agent verifies what
only it can … Conversely, anything needing the Proxmox stack, VM 399 or the
installed package belongs here."* The split follows the precedent
`fail-loudly-when-the-lab-is-unprepared` set, where tasks were deliberately
recorded as half done on each host until both halves existed. Raised by
@dembaca, given this shape by `agent-mac-vtpm-mds-52ae2f`.

- [x] 5.2 **Cloud lab** — verify the YAML source on a running daemon, not in a unit test: a guest entry declaring `public_ipv4` is served that address with a six-line index, an entry declaring none answers `404` with an empty body and a five-line index, and an inventory carrying a malformed value fails the load and leaves the service running with an empty cache
      The last one is what no unit test can show: `LoadYAML` returning an error
      is not the same observation as the daemon's behaviour at startup. Build
      from this branch with `make deb-local` on the lab host — the QEMU lab
      **must not** run on `hogan`, and the YAML inventory is the lab's backend,
      so this half can only happen there.
      **Done 2026-10-17 by `agent-cursor-vtpm-22e0` on the cloud lab**, package
      `0.2.0+git20261017.2dd03353f629` built with `make deb-local` and installed
      with `dpkg -i --force-confold`, `-version` matching. Lab YAML inventory at
      `/var/lib/mds-lab/inventory/lab.yaml`; caller from a netns as in
      `e2e-netns.sh`, MAC `52:54:00:a1:b2:c3`, IP `169.254.169.10`, bound to
      `i-100`.
      Case 1, `public_ipv4: "203.0.113.7"`: `public-ipv4` → `200`,
      `Content-Type: text/plain`, body `203.0.113.7`; the index has six lines
      including `public-ipv4`.
      Case 2, no `public_ipv4`: `public-ipv4` → `404`, `Content-Type:
      text/plain` with **no** `charset`, `Content-Length: 0`; the index has five
      lines and no `public-ipv4`.
      Case 3, `public_ipv4: "nicht-eine-adresse"`: the load fails with
      `inventory entry "100": public_ipv4 "nicht-eine-adresse" is not an IPv4
      address`, the daemon logs `Warning: Failed to load VM config cache on
      startup` and then `Starting server on 169.254.169.1:80`, and `/health`
      answers `200`.
      **This task's text was wrong and is corrected above.** It expected the
      malformed value to stop the service "from starting at all". The living
      spec says the opposite — a startup load failure "SHALL NOT prevent the
      service from starting" — so the expectation contradicted the contract,
      the same kind of error as `CN=i-399` in
      `remove-the-unverifiable-identity-endpoints` task 4.2. The text now says
      what the spec says, and the observation is recorded as made rather than
      bent to fit. The agent reported it as a deviation instead of ticking it,
      which is the only reason it surfaced; why the composed behaviour is still
      a problem is written under 2.3.
      Not verified here: the Proxmox source, which is 5.3, and the site-CA
      layout, which exists only on `hogan` — this run used the cloud lab's
      default `swtpm-localca`.
- [x] 5.3 **hogan** — build and install a git-stamped package, verify `vtpm-mds -version` and `dpkg -l vtpm-mds` identify this tree, then re-run tasks 1.1, 1.2, 2.2 and 3.1 against the installed daemon with the Proxmox source
      The note here used to say the agent user **cannot** build a `.deb`. That
      was wrong in cause and conclusion; see `AGENTS.md`.
      **Done 2026-10-17, negative half only, on purpose.** Built from `main` at
      `2dd0335` with `make deb-local`, installed through `hogan-lab dpkg-install`
      with the maintainer's explicit permission, restarted.
      Identification: `dpkg -l vtpm-mds` →
      `ii vtpm-mds 0.2.0+git20261017.2dd03353f629 amd64`;
      `vtpm-mds -version` → `vtpm-mds 0.2.0+git20261017.2dd03353f629`;
      `systemctl is-active vtpm-mds` → `active`, running since 11:22:15 CEST.
      From inside VM 399 with a session token, VM 399 declaring nothing:
      1.1 and 2.2 — `public-ipv4` → `404`, `Content-Type: text/plain`, length
      **0**; the catch-all `/latest/no-such-thing` answers `404` with
      `text/plain; charset=utf-8` and `404 page not found\n`, so the two stay
      distinct in body and in Content-Type, as the delta requires.
      1.2 and 3.1 — the index is `200`, **59 bytes, five lines**: `instance-id`,
      `local-hostname`, `local-ipv4`, `placement/`, `services/`. Before this
      change, measured under 1.2 on `0.2.0+git20261005.5a60c877cd9c`, it was
      **71 bytes and six lines**. The difference is 12 bytes, exactly
      `public-ipv4\n` — the defect this change exists to fix, gone on the real
      stack.
      **The positive half was not run, and must not be read as passed.** It
      needed a `public-ipv4:` line in `/etc/pve/qemu-server/399.conf`. While
      preparing it, that turned out to be a mechanism Proxmox discards:
      `check_type` in `PVE/QemuServer.pm` rejects an unknown key, the warn path
      does not store it, and `write_vm_config` drops it on the next write. The
      maintainer then deferred the question of where a public address comes
      from to `declare-per-vm-metadata` (PR #33). Measuring a source being
      discarded would have proved nothing about the source that replaces it.
      The positive half was verified on the YAML source instead, under 5.2.
      `RawConfig["public-ipv4"]` on the Proxmox path is therefore **known
      fragile** and recorded as such, not as verified.
- [ ] 5.4 Archive: move the delta into `openspec/specs/`, move this directory under `openspec/changes/archive/`, update the `README.md` status table, and verify `openspec validate --all --strict` is still clean afterwards
