# Tasks

Nothing below is ticked: this change record is planning only, and no code has
been touched. The two open questions in `design.md` must be answered before
task 2 begins — question 1 decides whether task 5 exists at all.

**Ordering.** Apply this change **after** `fix-client-ip-fallback`, which
modifies `identity/handlers.go`. This change deletes that file. Applying this
one first would silently discard that work; applying it second makes the
deletion an explicit, reviewable conflict.

## 1. Record the baseline

- [ ] 1.1 Send `POST /latest/attest` a valid nonce with a syntactically well-formed but entirely bogus quote and EK certificate, and verify it answers `200` with `"valid": true` and `"identity": "attested-identity"` — input no verifier would accept
      Run it against VM 399 (`sudo hogan-lab qm guest exec 399 …`, which has
      `python3` but no `curl`) with `enable_tpm_attestation: true`, so the
      baseline comes from the real daemon out of a git-stamped package rather
      than from a unit test.
- [ ] 1.2 Fetch `GET /.well-known/jwks.json` and verify its single entry's `n` is the literal `placeholder-modulus`, and that a token from `GET /latest/identity` carries `alg: HS256` and so cannot be verified against it
      This re-runs `workload-identity`'s own scenario
      `The published key set cannot verify the token` against the running
      daemon, so the removal rests on observed behaviour, not on the spec text.
- [ ] 1.3 Grep the whole tree, including `scripts/qemu-lab/` and every `_test.go`, for callers of the four endpoints, and verify the only hits are the four known ones
      Expected: `ARCHITECTURE.md`, the registrations in
      `internal/server/server.go`, `IdentityBearingPaths` in
      `imds/handlers.go`, and the refused-path test. **Any other hit stops the
      change** and goes back to the maintainer as open question 2.

## 2. Remove the attestation endpoints

- [ ] 2.1 Delete the `attest` package and remove its two route registrations, the `attest` import, the `nonceStore` field on `Server` and the `attest.SetStore` call from `internal/server/server.go` — verify `go build ./...` succeeds and no `attest.` reference survives outside `internal/devid/`
      `internal/devid/` has its own unrelated `attestData` and `CheckAKProp`
      identifiers; those are DevID residency checks and must not be touched.
- [ ] 2.2 Verify `GET /latest/attest/nonce` and `POST /latest/attest` now answer `404` with the body `404 page not found`, byte for byte identical to `GET /latest/no-such-thing`
      Compare the two responses against the installed daemon on VM 399, not in
      a unit test: the point is that the catch-all handler, not a new handler,
      is what answers.

## 3. Remove the identity document and the key set

- [ ] 3.1 Delete the `identity` package and remove its two route registrations and its import from `internal/server/server.go` — verify `go build ./...` succeeds and `go vet ./...` is clean
- [ ] 3.2 Remove `/latest/identity` from `IdentityBearingPaths` in `imds/handlers.go` and rewrite the doc comment above it, which names `workload-identity` and the identity package — verify `go test ./imds/` passes and the comment no longer refers to a capability or package that does not exist
      **This is the one line that overlaps `fix-client-ip-fallback`.** Resolve
      that merge deliberately rather than letting either side win by accident.
- [ ] 3.3 Invert the test asserting `/latest/identity` is a refused path so it asserts the route does not exist — verify `go test ./...` passes and fails again if the route is reinstated
- [ ] 3.4 Verify `GET /latest/identity` and `GET /.well-known/jwks.json` answer `404 page not found` on the real daemon, and that `GET /latest/meta-data/instance-id` still returns `i-399` for VM 399
      The migration note on `Name The Bound VM In The Identity Document`
      promises exactly that substitution. This task proves it rather than
      asserting it.

## 4. Confirm DevID enrollment is untouched

- [ ] 4.1 Verify the DevID enrollment routes are still registered under `mds.enable_tpm_attestation: true` and still absent when it is false, via `go test ./internal/devid/ ./internal/server/`
      `devid-enrollment`'s existing scenarios must pass **unchanged**. If one
      needs editing, the gating was altered and this change broke its own
      non-goal.
- [ ] 4.2 Run a guest end-to-end enrollment against VM 399 and verify an LDevID is still issued whose subject CN is `i-399`
      **Use a fresh vTPM.** A reused one risks the DA lockout recorded as
      known-gaps item 4 — unrelated to this change, but it will masquerade as a
      regression.

## 5. Decide and act on the instance-identity endpoints

- [ ] 5.1 **Blocked on the maintainer** (open question 1 in `design.md`) — verify the outcome is recorded either as a new change directory named here, or as a re-scoping of this change's `proposal.md`, `design.md` and `instance-metadata` delta
      If the recommendation is accepted, this closes as "no change, written up
      separately": raise a new change for the constant `dGVzdC1zaWduYXR1cmU=`
      signature and name its id here. If the maintainer wants those two paths
      removed in this change instead, it needs a fourth spec delta and tasks
      1–4 must be re-scoped before any further work.

## 6. Correct the documentation

- [ ] 6.1 Correct `ARCHITECTURE.md` — the goals bullet, the component diagram, four endpoint-table rows, three integration-table rows and roadmap phases 3 and 6 — and verify no occurrence of `/latest/attest`, `/latest/identity` or `jwks` survives except where it explicitly describes removed functionality
      The goals bullet claims the JWT is "consumable by SPIRE, Teleport, and
      Vault" while the same file's integration table says otherwise. The
      surviving rows are the two that were always true: SPIRE via DevID PEM +
      TPM2B blobs, Teleport via EK CA restriction.
- [ ] 6.2 Note in `ARCHITECTURE.md` that a Teleport TPM join needs nothing from this service, and verify the claim against Teleport's current documentation at implementation time rather than trusting this record
      Checked 2026-10-04 at
      <https://goteleport.com/docs/machine-workload-identity/deployment/linux-tpm/>:
      the host reads the EK public key and EKCert from its own TPM and sends
      the EK public key hash and the EKCert to the Auth Service, which checks
      `ek_public_hash`, `ekcert_allowed_cas` and `ek_certificate_serial`. It
      contacts no metadata service and needs no JWT. Teleport's join methods
      have changed before, so re-read rather than cite this.

## 7. Validate and archive

- [ ] 7.1 Verify `openspec validate --all --strict` exits 0 on `hogan`, where the CLI is installed at `/usr/local/bin/openspec`
      That CLI is the authoritative check.
      `scripts/openspec-validate.py --strict` is a floor, not a replacement —
      but it enforces one rule the CLI does not: every checkbox line must state
      its own verification.
- [ ] 7.2 Verify `go vet ./...` and `go test ./...` are both clean, with no package left importing `attest` or `identity`
- [ ] 7.3 Build and install a git-stamped package and verify `vtpm-mds -version` and `dpkg -l vtpm-mds` identify this tree, then re-run tasks 2.2, 3.4 and 4.2 against the installed daemon
      The agent user **cannot** build a `.deb`: `dpkg-buildpackage` needs
      `dpkg --print-architecture`, and `/usr/local/sbin/hogan-lab` allows only
      `dpkg -i`. The build half belongs to the maintainer — say so explicitly
      rather than implying it ran.
- [ ] 7.4 Archive the change — move the three deltas into `openspec/specs/`, **delete** `openspec/specs/workload-identity/spec.md` rather than leaving an empty capability, move this directory under `openspec/changes/archive/`, update the `README.md` status table — and verify `openspec validate --all --strict` is still clean afterwards and `openspec/specs/` no longer contains `workload-identity`
