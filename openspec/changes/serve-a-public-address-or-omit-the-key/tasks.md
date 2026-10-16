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
- [ ] 2.2 Implement the chosen source and verify a VM with a declared address serves it while a VM without one still answers `404` with an empty body
      For option A, `VMConfig.RawConfig` already exists and is documented as
      "Optional key/value metadata", so no struct change is needed.
      **Half done, and the other half needs a decision this record does not
      carry.** The reading side is implemented and tested:
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
      **With the YAML backend it is forbidden.**
      `Load VM Records From The YAML Inventory` says entries support "`id`,
      `name`, `macs` and `ek_sha256`, and keys outside that set SHALL be
      ignored", and that the raw configuration map "SHALL contain exactly one
      key, `name`". A `public_ipv4:` field would contradict both clauses. This
      change has **no `vm-inventory` delta**, so the contract does not cover
      it — a gap in this record, found while implementing it, not a gap in the
      spec. See the note under 2.3.
- [ ] 2.3 Validate a declared address at inventory load time and verify a malformed value fails the load loudly rather than being served
      Follow the precedent `fail-fast-on-unusable-configuration` set: an
      unusable value stops the service instead of degrading silently. Verify a
      non-IPv4 string and an IPv6 string are both rejected.
      **Blocked on the maintainer, with the question stated rather than
      guessed.** Load-time validation and the YAML field both change
      `vm-inventory`, which this change has no delta for. Three routes, and the
      choice is not the implementer's:
      **(i)** add a `vm-inventory` delta to *this* change, modifying
      `Load VM Records From The YAML Inventory` — the record's Impact section
      already names `internal/inventory/inventory.go`, so the file was in scope
      even though the spec consequence was never written down;
      **(ii)** keep this change to the Proxmox source and the index, which is
      spec-complete as it stands, and raise a separate change for the YAML
      field; **(iii)** drop the YAML field, leaving the lab inventory unable to
      declare an address.
      Until then the reading side is the second line of defence on its own:
      `PublicIPv4` treats a malformed declaration as absent, so one can never
      be served as an address, and the index never advertises it. That is not a
      substitute for failing the load — a typo stays silent, which is exactly
      what `fail-fast-on-unusable-configuration` argued against — which is why
      this task stays open rather than being ticked against the fallback.
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

- [ ] 4.1 Verify `local-hostname`, `local-ipv4`, `placement/availability-zone`, `services/domain`, the unknown-key subtree behaviour and the `301` on `/latest/meta-data` are byte for byte unchanged
      Every one of these is carried verbatim into the replacement requirement,
      so any difference is a regression, not a decision.
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

- [ ] 5.1 Verify `openspec validate --all --strict` exits 0 on `hogan`, where the CLI is at `/usr/local/bin/openspec`
      Also run `scripts/openspec-validate.py --strict`: it is a floor, not a
      replacement, but it enforces one rule the CLI does not — every checkbox
      line must contain its own verification.
- [ ] 5.2 Build and install a git-stamped package and verify `vtpm-mds -version` and `dpkg -l vtpm-mds` identify this tree, then re-run tasks 1.1, 1.2, 2.2 and 3.1 against the installed daemon
      The agent user **cannot** build a `.deb` — `dpkg-buildpackage` needs
      `dpkg --print-architecture` and `/usr/local/sbin/hogan-lab` allows only
      `dpkg -i`. That half belongs to the maintainer; say so explicitly rather
      than implying it ran.
- [ ] 5.3 Archive: move the delta into `openspec/specs/`, move this directory under `openspec/changes/archive/`, update the `README.md` status table, and verify `openspec validate --all --strict` is still clean afterwards
