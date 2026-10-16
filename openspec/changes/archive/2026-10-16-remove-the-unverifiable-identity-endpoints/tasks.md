# Tasks

Implemented on 2026-10-05 against `main` at `396b05f`. Both open questions in
`design.md` were answered by the maintainer first: the instance-identity
endpoints **stay** (task 5.1), and nothing outside the repository is known to
call the removed endpoints.

Tasks 2.2, 3.4, 4.2 and 7.3 stayed unticked until a git-stamped `.deb` of this
tree ran on `hogan`. **All four were verified against it on 2026-10-16** and
carry their evidence below; only the archive task is still open. The reason
recorded here for the eleven-day delay — "the agent user cannot build a `.deb`:
`dpkg-buildpackage` needs `dpkg --print-architecture` and
`/usr/local/sbin/hogan-lab` allows only `dpkg -i`" — was wrong in its cause and
in its conclusion. See 7.3.

**Ordering.** Apply this change **after** `fix-client-ip-fallback`, which
modifies `identity/handlers.go`. This change deletes that file. Applying this
one first would silently discard that work; applying it second makes the
deletion an explicit, reviewable conflict.

## 1. Record the baseline

- [x] 1.1 Send `POST /latest/attest` a valid nonce with a syntactically well-formed but entirely bogus quote and EK certificate, and verify it answers `200` with `"valid": true` and `"identity": "attested-identity"` — input no verifier would accept
      Run it against VM 399 (`sudo hogan-lab qm guest exec 399 …`, which has
      `python3` but no `curl`) with `enable_tpm_attestation: true`, so the
      baseline comes from the real daemon out of a git-stamped package rather
      than from a unit test.
      **Done.** Run from the host against the installed daemon
      `0.2.0+git20261003.9fad3704a940` on `169.254.169.1:80` — the host rather
      than VM 399 because `/latest/attest` is registered with `mux.HandleFunc`
      and not the token-wrapping `handle`, so it needs neither a session token
      nor a bound VM record. That is itself part of the finding.
      `GET /latest/attest/nonce` → `200`. `POST /latest/attest` with
      `quote` = base64("this is not a TPM quote") and `ek_cert` =
      base64("this is not an EK certificate") → `200`
      `{"valid":true,"message":"Attestation verified (placeholder)","identity":"attested-identity"}`.
- [x] 1.2 Fetch `GET /.well-known/jwks.json` and verify its single entry's `n` is the literal `placeholder-modulus`, and that a token from `GET /latest/identity` carries `alg: HS256` and so cannot be verified against it
      This re-runs `workload-identity`'s own scenario
      `The published key set cannot verify the token` against the running
      daemon, so the removal rests on observed behaviour, not on the spec text.
      **Done, and sharper than the spec states.** `GET /.well-known/jwks.json`
      → `200 {"keys":[{"kty":"RSA","kid":"key-1","use":"sig","alg":"RS256","n":"placeholder-modulus","e":"AQAB"}]}`.
      From VM 399 via `sudo hogan-lab qm guest exec 399 … python3`:
      `instance-id` → `i-399`; `GET /latest/identity` → `200` with JWT header
      `{"alg":"HS256","typ":"JWT"}` and claims `level: unattested`,
      `attest: false`, `sub`/`instance_id` `i-399`. The key set advertises
      **`alg: RS256`** while the token is **`HS256`** — not merely
      unverifiable, but self-contradictory.
- [x] 1.3 Grep the whole tree, including `scripts/qemu-lab/` and every `_test.go`, for callers of the four endpoints, and verify the only hits are the four known ones
      Expected: `ARCHITECTURE.md`, the registrations in
      `internal/server/server.go`, `IdentityBearingPaths` in
      `imds/handlers.go`, and the refused-path test. **Any other hit stops the
      change** and goes back to the maintainer as open question 2.
      **Done, with a finding that widened the change.** The four expected hits
      were present. But `README.md` carries an endpoint table listing all four
      endpoints, and `debian/vtpm-mds.8` names `/latest/identity` in its
      refused-path list — **neither was in this change's Impact section**.
      Leaving them would repeat exactly the defect AGENTS.md records, where
      `README.md` documented flags no commit contained. Both were corrected and
      `proposal.md` Impact was amended. `scripts/qemu-lab/setup-host.sh` matches
      only on `jwks_path`, a config key this change deliberately keeps, so the
      cloud lab agent's file set is untouched. **No caller was found.**

## 2. Remove the attestation endpoints

- [x] 2.1 Delete the `attest` package and remove its two route registrations, the `attest` import, the `nonceStore` field on `Server` and the `attest.SetStore` call from `internal/server/server.go` — verify `go build ./...` succeeds and no `attest.` reference survives outside `internal/devid/`
      `internal/devid/` has its own unrelated `attestData` and `CheckAKProp`
      identifiers; those are DevID residency checks and must not be touched.
      **Done.** `git rm -r attest identity`; removed both imports, the
      `nonceStore` field, `attest.NewNonceStore()`, `attest.SetStore` and
      `identity.SetStore` from `internal/server/server.go`. `go build ./...`
      exits 0. The only surviving `attest` matches are `internal/devid`'s own
      `attestData` and `CheckAKProp`, which are DevID residency checks.
- [x] 2.2 verify `GET /latest/attest/nonce` and `POST /latest/attest` now answer `404` with the body `404 page not found`, byte for byte identical to `GET /latest/no-such-thing`
      Compare the two responses against the installed daemon on VM 399, not in
      a unit test: the point is that the catch-all handler, not a new handler,
      is what answers.
      Unticked on 2026-10-16 because it had been ticked on the strength of a
      locally built binary on `127.0.0.1:18080`, while this task says *the
      installed daemon on VM 399* — `agent-mac-vtpm-mds-e69517` flagged it and
      the catch was correct. **Re-ticked the same day, against the thing the
      task actually names.**

      Installed daemon `0.2.0+git20261005.5a60c877cd9c`, probed from inside the
      guest with `sudo hogan-lab qm guest exec 399 -- python3` against
      `http://169.254.169.254`. Reference `GET /latest/no-such-thing` → `404`,
      `text/plain; charset=utf-8`, body `b'404 page not found\n'`. All four
      removed routes compare **identical on all three** — status, Content-Type
      and body bytes: `GET /latest/attest/nonce`, `POST /latest/attest`,
      `GET /latest/identity`, `GET /.well-known/jwks.json`. The catch-all, not
      a new handler, answers. The earlier binary-level run agreed, which is why
      nothing had to change — but agreement is not evidence, and it is the
      package the service runs from that this task was about.

## 3. Remove the identity document and the key set

- [x] 3.1 Delete the `identity` package and remove its two route registrations and its import from `internal/server/server.go` — verify `go build ./...` succeeds and `go vet ./...` is clean
      **Done.** `go build ./...` exits 0 and `go vet ./...` is clean.
- [x] 3.2 Remove `/latest/identity` from `IdentityBearingPaths` in `imds/handlers.go` and rewrite the doc comment above it, which names `workload-identity` and the identity package — verify `go test ./imds/` passes and the comment no longer refers to a capability or package that does not exist
      **This is the one line that overlaps `fix-client-ip-fallback`.** Resolve
      that merge deliberately rather than letting either side win by accident.
      **Done.** `go test ./imds/` passes. The comment no longer names the
      identity package or the `workload-identity` capability; it records that a
      fourth entry existed and why it went. Merge note: this is the line that
      overlapped `fix-client-ip-fallback`, which merged first as PR #20 and
      left it untouched, so there was no conflict to resolve.
- [x] 3.3 Invert the test asserting `/latest/identity` is a refused path so it asserts the route does not exist — verify `go test ./...` passes and fails again if the route is reinstated
      **Done.** `TestIdentityEndpointIsGone` asserts that `/latest/identity`
      and `/.well-known/jwks.json` are absent from `IdentityBearingPaths` and
      answer `404` from the catch-all **to a bound caller** — the caller who
      would previously have been served a token. It fails if either route is
      reinstated. `TestIdentityBearingPathsIsExactlyTheSpecifiedSet` now pins
      three paths. `go test ./...` passes.
- [x] 3.4 verify that `GET /latest/identity` and `GET /.well-known/jwks.json` answer `404 page not found` on the real daemon, and that `GET /latest/meta-data/instance-id` still returns `i-399` for VM 399
      The migration note on `Name The Bound VM In The Identity Document`
      promises exactly that substitution. This task proves it rather than
      asserting it.
      **Done.** 2026-10-16 from VM 399 against the installed
      `0.2.0+git20261005.5a60c877cd9c`, with a valid session token:
      `GET /latest/identity` → `404 '404 page not found\n'`,
      `GET /.well-known/jwks.json` → `404 '404 page not found\n'`,
      `GET /latest/meta-data/instance-id` → `200 'i-399'`. The substitution the
      migration note promises therefore holds for exactly the caller that used
      to be served a token, not merely for a test double.

## 4. Confirm DevID enrollment is untouched

- [x] 4.1 Verify the DevID enrollment routes are still registered under `mds.enable_tpm_attestation: true` and still absent when it is false, via `go test ./internal/devid/ ./internal/server/`
      `devid-enrollment`'s existing scenarios must pass **unchanged**. If one
      needs editing, the gating was altered and this change broke its own
      non-goal.
      **Done.** `go test ./internal/devid/ ./internal/server/` passes
      unchanged — no scenario needed editing, which is the point. The
      `if cfg.MDS.EnableTPMAttestation` block still registers
      `loadDevIDEnroller` and `devid.Register`; only the four route
      registrations were removed from inside it, and a comment now records that
      the setting gates enrollment alone.
- [x] 4.2 run a guest end-to-end enrollment against VM 399 and verify an LDevID is still issued whose subject CN is `399`
      **Use a fresh vTPM.** A reused one risks the DA lockout recorded as
      known-gaps item 4 — unrelated to this change, but it will masquerade as a
      regression.
      **Done, and this task's own text was wrong.** It asked for `CN=i-399`.
      The living `devid-enrollment` spec says the opposite — "the issued
      certificate's subject is `CN=100`", the bare VM id — because `i-` is the
      metadata instance-id prefix and was never part of an LDevID subject. The
      text above is corrected rather than the observation bent to fit it.
      2026-10-16 against the installed `0.2.0+git20261005.5a60c877cd9c`:
      `devid-enroll -tpm /dev/tpmrm0 -out /tmp/devid-422` in VM 399 exits 0 and
      writes `devid.crt.pem`, `devid.priv.blob`, `devid.pub.blob`. Parsed on
      the host: `subject=CN=399`, `issuer=CN=BGL Proxmox DevID CA`,
      `notBefore=Oct 16 14:43:53 2026 GMT`, Key Usage `Digital Signature`
      (critical), Extended Key Usage `2.23.133.11.1.2`, the TCG DevID OID.
      The guest client is the released `devid-enroll 0.2.0`, not a stamped
      build — the daemon is the stamped side, and the daemon is what this
      change touches.
      **The fresh vTPM was not needed and not taken.** `tpm2_getcap
      properties-variable` in the guest read `TPM2_PT_LOCKOUT_COUNTER: 0x0` and
      `inLockout: 0` **before** the run, so the failure mode this instruction
      guards against was demonstrably absent, and a reset would have discarded
      the EK whose certificate the site CA minted, for no gain. The maintainer
      has since made vTPM resets on VM 399 a standing authorization, so the
      choice was about evidence, not permission.
      The counter read `0x0` **after** the run as well: enrollment burned no DA
      counter at all. That corroborates known-gaps item 4's root cause sitting
      in `scripts/qemu-lab/stop-guest.sh`, not in enrollment — recorded here
      because it is a free data point for that change, not a claim about this one.

## 5. Decide and act on the instance-identity endpoints

- [x] 5.1 **Blocked on the maintainer** (open question 1 in `design.md`) — verify the outcome is recorded either as a new change directory named here, or as a re-scoping of this change's `proposal.md`, `design.md` and `instance-metadata` delta
      If the recommendation is accepted, this closes as "no change, written up
      separately": raise a new change for the constant `dGVzdC1zaWduYXR1cmU=`
      signature and name its id here. If the maintainer wants those two paths
      removed in this change instead, it needs a fourth spec delta and tasks
      1–4 must be re-scoped before any further work.
      **Closed as "no change, written up separately".** The maintainer decided
      on 2026-10-05 that `/latest/dynamic/instance-identity/{document,signature}`
      **stay**: genuine AWS paths that cloud-init and the SDKs look for, and
      `instance-metadata` already calls the document *unsigned* rather than
      claiming something false. The constant `dGVzdC1zaWduYXR1cmU=` remains a
      defect and gets its own change. Research was commissioned first — whether
      any consumer lets a custom CA replace AWS' regional certificate, and
      whether the IMDS endpoint can be pointed at `169.254.169.1` — because a
      compatible signature is worthless if neither is configurable. That answer
      decides whether the follow-up implements `/signature` or removes it.

## 6. Correct the documentation

- [x] 6.1 Correct `ARCHITECTURE.md` — the goals bullet, the component diagram, four endpoint-table rows, three integration-table rows and roadmap phases 3 and 6 — and verify no occurrence of `/latest/attest`, `/latest/identity` or `jwks` survives except where it explicitly describes removed functionality
      The goals bullet claims the JWT is "consumable by SPIRE, Teleport, and
      Vault" while the same file's integration table says otherwise. The
      surviving rows are the two that were always true: SPIRE via DevID PEM +
      TPM2B blobs, Teleport via EK CA restriction.
      **Done.** Corrected the goals bullets (the JWT bullet claiming it was
      "consumable by SPIRE, Teleport, and Vault" is gone), the component
      diagram, the consumer box and its trust arrow, four endpoint-table rows,
      the trust-model JWT line, three integration-table rows, and roadmap
      phases 2, 3 and 6. Added a "Removed: the attestation and JWT identity
      path" section so a reader of the tables finds the reason. No occurrence
      of `/latest/attest`, `/latest/identity` or `jwks` survives outside that
      section and two annotated config lines.
- [x] 6.2 Note in `ARCHITECTURE.md` that a Teleport TPM join needs nothing from this service, and verify the claim against Teleport's current documentation at implementation time rather than trusting this record
      Checked 2026-10-04 at
      <https://goteleport.com/docs/machine-workload-identity/deployment/linux-tpm/>:
      the host reads the EK public key and EKCert from its own TPM and sends
      the EK public key hash and the EKCert to the Auth Service, which checks
      `ek_public_hash`, `ekcert_allowed_cas` and `ek_certificate_serial`. It
      contacts no metadata service and needs no JWT. Teleport's join methods
      have changed before, so re-read rather than cite this.
      **Done.** Re-checked on 2026-10-05 at
      <https://goteleport.com/docs/machine-workload-identity/deployment/linux-tpm/>
      rather than cited from this record: the host reads the EK public key and
      EKCert from its **own** TPM and sends the EK public key hash and the
      EKCert to the Auth Service, which checks `ek_public_hash`,
      `ekcert_allowed_cas` and `ek_certificate_serial`. It contacts no metadata
      service, no IMDS and no JWKS endpoint.

## 7. Validate and archive

- [x] 7.1 Verify `openspec validate --all --strict` exits 0 on `hogan`, where the CLI is installed at `/usr/local/bin/openspec`
      That CLI is the authoritative check.
      `scripts/openspec-validate.py --strict` is a floor, not a replacement —
      but it enforces one rule the CLI does not: every checkbox line must state
      its own verification.
      **Done.** `openspec validate --all --strict` → `Totals: 8 passed, 0 failed`
      on this branch: six living specs plus this change plus
      `fix-client-ip-fallback`. (`serve-a-public-address-or-omit-the-key` sits
      on its own branch and is not counted here.)
      `scripts/openspec-validate.py --strict` reports only `all tasks ticked`
      for this change, which is correct — five tasks are open by design.
- [x] 7.2 Verify `go vet ./...` and `go test ./...` are both clean, with no package left importing `attest` or `identity`
      **Done.** Both clean; no package imports `attest` or `identity`, and both
      directories are gone. `gofmt -l` flags `imds/handlers_test.go`,
      `imds/token.go`, `imds/token_test.go`, `internal/config/*` and
      `internal/devid/devid_test.go` — all **pre-existing** trailing whitespace,
      confirmed by stashing this change and re-running on `main`. Deliberately
      not reformatted: unrelated churn.
- [x] 7.3 build and install a git-stamped package and verify `vtpm-mds -version` and `dpkg -l vtpm-mds` identify this tree, then re-run tasks 2.2, 3.4 and 4.2 against the installed daemon
      This note used to read: "The agent user **cannot** build a `.deb`:
      `dpkg-buildpackage` needs `dpkg --print-architecture`, and
      `/usr/local/sbin/hogan-lab` allows only `dpkg -i`." **That was wrong, and
      it is what held this task open for eleven days.** `/usr/bin/dpkg
      --print-architecture` always answered `amd64` for this user; what refused
      it was `~/bin/dpkg`, a PATH wrapper forwarding only `dpkg -i`. The
      corrected account is in `AGENTS.md`.
      **Done.** Built 2026-10-16 from `main` at `5a60c87` with `make deb-local`
      — no PATH or `DEB_STAGE` override needed, once the wrapper passed
      read-only queries through. Installed by the maintainer, because Claude
      Code's own auto-mode classifier refuses a package install regardless of
      the maintainer's go; that refusal was reported rather than circumvented.
      Identification: `dpkg -l vtpm-mds` →
      `ii  vtpm-mds  0.2.0+git20261005.5a60c877cd9c  amd64`;
      `vtpm-mds -version` → `vtpm-mds 0.2.0+git20261005.5a60c877cd9c`;
      `systemctl is-active vtpm-mds` → `active`, running since 16:41:19 CEST as
      `/usr/sbin/vtpm-mds -config /etc/vtpm-mds/config.yaml`.
      Checked at the artifact before installing: `dpkg-deb -f` agreed, the
      extracted binary's `-version` agreed, and `systemd-analyze verify` on the
      packaged unit reported nothing — that last one against a deliberately
      broken copy as a control, because `verify` also exits 0 for a path that
      does not exist, so a silent run proves nothing on its own.
      Tasks 2.2, 3.4 and 4.2 were then re-run against the installed daemon and
      carry their own evidence above.
- [x] 7.4 Archive the change — move the three deltas into `openspec/specs/`, **delete** `openspec/specs/workload-identity/spec.md` rather than leaving an empty capability, move this directory under `openspec/changes/archive/`, update the `README.md` status table — and verify `openspec validate --all --strict` is still clean afterwards and `openspec/specs/` no longer contains `workload-identity`
      **Done** 2026-10-16, after `fix-client-ip-fallback` was archived first —
      both carried an `instance-metadata` delta, so the order decides which
      text survives.
      Archived with the CLI rather than by hand: `openspec archive
      remove-the-unverifiable-identity-endpoints -y` reported
      `instance-metadata: ~ 1 modified`, `vm-inventory: ~ 1 modified`,
      `Retiring openspec/specs/workload-identity/spec.md: all requirements
      removed`, `Totals: + 0, ~ 2, - 4, → 0`.
      **The capability deletion needed a step this task did not know about.**
      The first attempt aborted with `Spec must have at least one requirement`
      and wrote nothing; the CLI named the remedy itself, so this change gained
      a `.openspec.yaml` carrying `retire_capabilities: true` beside its
      `schema:`. That flag is what turns "every requirement removed" into
      "capability retired" instead of an error — worth knowing for the next
      change that empties a capability.
      Afterwards: `openspec validate --all --strict` → `Totals: 5 passed, 0
      failed`; `openspec/specs/` holds `debian-packaging`, `devid-enrollment`,
      `instance-metadata`, `operability`, `vm-inventory` and **no**
      `workload-identity`; `openspec/changes/` holds only `archive/`.
      `README.md`'s status table had gone stale in a way task 1.3 did not
      catch: two rows still read "endpoints present", which stopped being true
      when the routes went. Both now say removed, with the reason.
