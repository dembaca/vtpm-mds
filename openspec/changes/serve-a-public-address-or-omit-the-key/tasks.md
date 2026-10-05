# Tasks

Nothing is ticked: this is a planning record, no code has been touched.

**Blocked before task 2.** The open question in `design.md` — *where does the
address come from* — decides whether sections 2 and 3 exist at all. If the
maintainer picks option D (leave it unimplemented), this change shrinks to
section 4 plus the index half of section 5, and the spec delta must be
rewritten to say the key never appears rather than appearing conditionally.

**Ordering.** Apply **after** `fix-client-ip-fallback`, which edits
`imds/handlers.go`. Different functions — `getPublicIP` and
`HandleMetaDataIndex` here, `getClientIP` and `InstanceID` there — so the
conflict is mechanical, but the two must not be implemented in parallel.

## 1. Record the baseline

- [ ] 1.1 Read `GET /latest/meta-data/public-ipv4` from VM 399 against the installed daemon and verify it answers `404` with an **empty** body and `Content-Type: text/plain`, not the catch-all's `404 page not found`
      Use `sudo hogan-lab qm guest exec 399 …`; VM 399 has `python3` and no
      `curl`. The distinction matters: the delta promises this exact response
      shape survives the change.
- [ ] 1.2 Read `GET /latest/meta-data/` from the same guest and verify the index is six lines and includes `public-ipv4` — the key that 1.1 just proved unreadable
      This pair is the defect in one screenshot; record both outputs together.

## 2. Add the source — blocked on the maintainer's choice

- [ ] 2.1 **Blocked**: record the maintainer's answer to the source question here before writing code, and verify the chosen option matches what the spec delta already says
      The delta is written for option A (an operator-declared per-VM value).
      Options B and C change the delta's wording about what the service may
      derive; option D removes the conditional listing entirely.
- [ ] 2.2 Implement the chosen source and verify a VM with a declared address serves it while a VM without one still answers `404` with an empty body
      For option A, `VMConfig.RawConfig` already exists and is documented as
      "Optional key/value metadata", so no struct change is needed.
- [ ] 2.3 Validate a declared address at inventory load time and verify a malformed value fails the load loudly rather than being served
      Follow the precedent `fail-fast-on-unusable-configuration` set: an
      unusable value stops the service instead of degrading silently. Verify a
      non-IPv4 string and an IPv6 string are both rejected.
- [ ] 2.4 Verify a declared address is never derived from the connection — assert that a caller whose peer is `203.0.113.7` but whose VM declares nothing still gets `404`
      This is the one way the change could go wrong without any test failing:
      quietly falling back to the peer address would look correct in a lab
      where the two happen to match.

## 3. Make the index caller-dependent

- [ ] 3.1 Make `HandleMetaDataIndex` list `public-ipv4` only when the caller has a declared address, and verify the two new index scenarios both pass
      The handler currently builds a fixed slice and consults nothing. It needs
      the same bound-VM lookup the other handlers already do.
- [ ] 3.2 Verify the index is rendered per request and never cached or shared between callers, by reading it as two different VMs in the same process and confirming the two listings differ
      This is the property the delta explicitly requires. Nothing caches it
      today; the task exists so that stays true.
- [ ] 3.3 Verify an unbound caller still receives an index, without `public-ipv4`, and is not refused
      `Refuse An Instance Identity To Callers With No VM Record` deliberately
      leaves the index open, because it asserts no identity. That must not
      change here.

## 4. Confirm nothing else moved

- [ ] 4.1 Verify `local-hostname`, `local-ipv4`, `placement/availability-zone`, `services/domain`, the unknown-key subtree behaviour and the `301` on `/latest/meta-data` are byte for byte unchanged
      Every one of these is carried verbatim into the replacement requirement,
      so any difference is a regression, not a decision.
- [ ] 4.2 Verify `getAvailabilityZone` and `getRegion` are untouched, including their `TODO` comments
      They carry the same comment as `getPublicIP` but return the real fixed
      values the spec pins. They are explicitly out of scope; touching them
      would repeat the `-version` mistake AGENTS.md records.
- [ ] 4.3 Verify `go vet ./...` and `go test ./...` are both clean and report the output

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
