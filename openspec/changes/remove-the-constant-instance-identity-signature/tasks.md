# Tasks

Written on 2026-10-16 against `main` at `7db05db`. The code half was
implemented on 2026-10-17 by `agent-mac-vtpm-mds-52ae2f` against `main` at
`2dd0335`, which contains `serve-a-public-address-or-omit-the-key` (#29).
Tasks 1.1 and 1.2 were measured on 2026-10-17 against the installed package
of `main@2dd0335`. Section 4, which needs the package built from this branch,
and the coordinator's tasks (3.2, 5.3, 5.4) are still open.

**Do not merge the implementation before** `serve-a-public-address-or-omit-the-key`
tasks 5.2a and 5.2b are reported. Those measure a package built from a `main`
that contains Item 2 and nothing newer; merging this first would make that
claim weaker.

The direction is decided, not open: the maintainer approved removing the
signature on 2026-10-16 on the strength of the research summarised in
`proposal.md`, and decided on 2026-10-05 that the document stays.

**Ordering.** `serve-a-public-address-or-omit-the-key` is being implemented at
the same time and also edits `imds/handlers.go`. Either may merge first; the
second rebases. The two deltas modify different `instance-metadata`
requirements, but this one carries `Refuse An Instance Identity To Callers With
No VM Record` in full, including the sentence "`public-ipv4` is always
unavailable, and the index is a static listing" that the other change makes
untrue. See task 5.3 and `design.md`.

**Where each task runs.** Sections 2, 3 and 5.1–5.2 run anywhere with Go and
the OpenSpec CLI. Sections 1 and 4 need the installed package and VM 399 on
`hogan`; they belong to the agent there. The `README.md` row in 3.2 belongs to
the coordinator.

## 1. Record the baseline

- [x] 1.1 From VM 399 with a valid session token, read `GET /latest/dynamic/instance-identity/signature` against the installed daemon and verify it answers `200`, `Content-Type: text/plain`, body `dGVzdC1zaWduYXR1cmU=`
      Use `sudo hogan-lab qm guest exec 399 …`; VM 399 has `python3` and no
      `curl`. Record `vtpm-mds -version` alongside, so the baseline names the
      tree it came from, and save the raw bytes of `GET .../document` from the
      same guest: 4.3 compares against them.
      **Done** on 2026-10-17, measured by the coordinator
      (`agent-vtpm-hogan-ead990e30`) on hogan from VM 399 with a session token,
      against the installed `vtpm-mds 0.2.0+git20261017.2dd03353f629` (`main`
      at `2dd0335`, which contains Item 2 and nothing newer, so this change is
      the only difference 4.3 can later see):
      - `GET …/instance-identity/signature` → `200`, `Content-Type:
        text/plain`, body `dGVzdC1zaWduYXR1cmU=` (20 bytes).
      - `GET …/instance-identity/document` → `200`, `application/json`, **245
        bytes**, SHA-256
        `d6a57188099b8bda78da67dfc935feef0e18cef873e77b1cf0001850dbbcfb03`,
        ending in a `\n`.
      **Raw bytes of the document, kept here because `/tmp` on hogan is tmpfs
      and does not survive a restart** (base64 of the body; I decoded it and
      confirmed 245 bytes and the same SHA-256 before entering it). It holds the
      placeholder `accountId`, `instanceId` `i-399` and the link-local
      `privateIp` — nothing sensitive:
      ```
      eyJhY2NvdW50SWQiOiIwMTIzNDU2Nzg5MDEiLCJhdmFpbGFiaWxpdHlab25lIjoicHJveG1veCIsImJpbGxpbmdQcm9kdWN0cyI6bnVsbCwiZGV2cGF5UHJvZHVjdENvZGVzIjpudWxsLCJpbWFnZUlkIjoicHJveG1veC11bmtub3duIiwiaW5zdGFuY2VJZCI6ImktMzk5IiwiaW5zdGFuY2VUeXBlIjoidm0iLCJwcml2YXRlSXAiOiIxNjkuMjU0LjE2OS4xMCIsInJlZ2lvbiI6ImxvY2FsIiwidmVyc2lvbiI6IjIwMTctMDktMzAifQo=
      ```
      Decoded, for readers:
      `{"accountId":"012345678901","availabilityZone":"proxmox","billingProducts":null,"devpayProductCodes":null,"imageId":"proxmox-unknown","instanceId":"i-399","instanceType":"vm","privateIp":"169.254.169.10","region":"local","version":"2017-09-30"}`

- [x] 1.2 From the same guest, read `.../instance-identity/pkcs7`, `.../instance-identity/rsa2048`, the listing `.../instance-identity/` and `GET /latest/no-such-thing`, and verify all four answer identically in status, `Content-Type` and body bytes
      This is the state the removed path is meant to join. If any of the four
      differs, stop: the delta's claim that the sibling paths already fall
      through to the catch-all is wrong.
      **Done**, same session and daemon. The reference `GET
      /latest/no-such-thing` → `404`, `text/plain; charset=utf-8`, body
      `404 page not found\n`. Compared to it in status, `Content-Type` and body
      bytes, all three are identical: `…/instance-identity/pkcs7`,
      `…/instance-identity/rsa2048` and the listing `…/instance-identity/`.
      The delta's claim that the sibling paths already fall through to the
      catch-all therefore holds on the real daemon, and the "stop" condition
      did not trigger.

- [x] 1.3 Re-read `get_instance_identity` in cloud-init's `cloudinit/sources/helpers/ec2.py` at implementation time and verify it still reaches leaves only through the `dynamic/instance-identity/` listing, never by requesting `signature` by name
      `design.md` relies on this reading from 2026-10-16. cloud-init changes;
      cite the commit you read. If it now requests `signature` directly, the
      change goes back to the maintainer before any code is touched.
      **Done** on 2026-10-17 by `agent-mac-vtpm-mds-52ae2f`, read at
      cloud-init `340ac746f344` (last commit touching
      `cloudinit/sources/helpers/ec2.py`) and `68d3d9448145` (last touching
      `DataSourceEc2.py`). `get_instance_identity` calls
      `_get_instance_metadata(tree="dynamic/instance-identity")`, which fetches
      that listing first and crawls only the leaves it names; any exception,
      including a `404` on the listing, is logged and yields `{}`.
      `DataSourceEc2.py` reaches instance identity only through
      `ec2.get_instance_identity` (lines 309 and 620) and requests no
      `signature`, `pkcs7` or `rsa2048` path by name. The listing URL is
      `.../dynamic/instance-identity` without a trailing slash; this service
      answers both spellings with the catch-all, so the conclusion holds.

- [x] 1.4 Run `git grep -n 'instance-identity/signature\|dGVzdC1zaWduYXR1cmU\|HandleInstanceIdentitySignature'` over the whole tree, including `scripts/qemu-lab/` and every `_test.go`, and verify every hit is listed under Impact in `proposal.md`
      Expected: the registration, the handler, `IdentityBearingPaths`, four
      places in `imds/handlers_test.go`, the `instance-metadata` spec,
      `README.md`, `ARCHITECTURE.md`, `debian/vtpm-mds.8` (as
      `instance\-identity/signature`, so grep for that spelling too), the
      `config.yaml` comment, and `AGENTS.md`. **A hit outside that list is a
      caller nobody knew about**: record it and amend Impact before going on,
      as `remove-the-unverifiable-identity-endpoints` task 1.3 had to.
      **Done** against `main@2dd0335`. Every hit is on the Impact list. One
      hit the list does not name: the still-unarchived
      `serve-a-public-address-or-omit-the-key` delta, whose `MODIFIED` block
      on `Refuse An Instance Identity …` carries the signature path. That is
      the overlap `design.md` already describes, not a caller, and the
      coordinator rebuilds whichever block archives second.

## 2. Remove the route

- [x] 2.1 Delete the `GET /latest/dynamic/instance-identity/signature` registration from `internal/server/server.go` and `HandleInstanceIdentitySignature` from `imds/handlers.go`, and verify `go build ./...` succeeds and the grep from 1.4 no longer finds the handler name
      **Done.** Registration and handler removed; a comment at the
      registration site says why no signature path is registered.
      `go build ./imds/ ./internal/... ./cmd/devid-enroll` succeeds, and the
      grep finds `HandleInstanceIdentitySignature` nowhere.

- [x] 2.2 Remove the signature path from `IdentityBearingPaths` and update the doc comment above it to say the path was removed rather than leaving it to be inferred — verify `go test ./imds/` passes and the comment names no route that is not registered
      **Done.** The doc comment now lists both removed entries —
      `/latest/identity` and the signature path — as removed and unregistered.
      `go test ./imds/` → `ok`.

- [x] 2.3 Replace the signature row in the refused-path table, the signature route in the test mux and `TestSignatureServedToPeerWithoutIPv4` with a test that `GET /latest/dynamic/instance-identity/signature` answers exactly what `GET /latest/no-such-thing` answers — for a bound caller with a token, an unbound caller with a token, and a caller with no token — and verify the test fails when the registration is put back
      Run the test against the server's real mux, not `newTestMux`, or it
      proves nothing about the catch-all. "Fails when put back" is the part
      that shows the test tests something; say how you checked it.
      **Done.** `TestInstanceIdentitySignaturePathsAreNotServed` in
      `internal/server/signature_test.go` builds the server with `New`, as
      `main` does (YAML inventory from `testdata/`, so `/etc/pve` is not read),
      and drives `srv.httpServer.Handler`. For each of the three callers it
      compares status, `Content-Type` and body of `.../signature`,
      `.../pkcs7` and `.../rsa2048` to `GET /latest/no-such-thing`.
      `TestInstanceIdentityDocumentIsStillServed` guards against the test
      passing only because the whole dynamic tree went missing. The imds-level
      signature row, test route and `TestSignatureServedToPeerWithoutIPv4`
      are removed. **Checked that it tests something:** with a signature
      route temporarily put back in `server.go`, the test failed for every
      caller (`status:200 … body:dGVzdC1zaWduYXR1cmU=`, want the catch-all's
      `404`); after restoring the file it passed again and `git diff` showed
      only the intended change.

- [x] 2.4 Pin `TestIdentityBearingPathsIsExactlyTheSpecifiedSet` to the two remaining paths and verify every existing document test passes **unchanged**
      If a document test needs editing, the document's behaviour moved and
      this change broke its own non-goal.
      **Done.** The test pins the two remaining paths and additionally asserts
      the signature path is not on the refusing side. No document test was
      edited, and all pass.

## 3. Correct the documentation

- [x] 3.1 Remove the signature row from `ARCHITECTURE.md`'s key-endpoints table, the signature path from the `require_vm_identity` entry in `debian/vtpm-mds.8`, and "both … endpoints" from the `config.yaml` comment — verify the grep from 1.4 finds none of them, and that `man -l debian/vtpm-mds.8` still renders that entry as one sentence
      The man page lists the paths as "a, b and c"; with two it becomes "a and
      b", so the `.I`/`.IR` macros around the joining words change too.
      **Done.** `ARCHITECTURE.md` row removed, and the document row now says
      "EC2-style IID, unsigned". The man page entry reads
      "`/latest/meta-data/instance-id` and
      `/latest/dynamic/instance-identity/document`." — rendered with
      `mandoc -T utf8` (macOS `man` has no `-l`) as one sentence. `mandoc -T
      lint` reports only the existing date-format warning on `.TH`. The
      `config.yaml` comment names the document path. The grep finds the
      signature only in `README.md` (3.2), `AGENTS.md` (5.4), and the
      intentional mentions in the `IdentityBearingPaths` comment and the new
      negative assertion.

- [ ] 3.2 **Coordinator** — remove the signature row from the API endpoint table in `README.md`, and verify the table lists `document` and no other `dynamic/` path
      `AGENTS.md` keeps `README.md` in one hand. The implementer hands this
      over on the bus rather than editing the file.

## 4. Verify on the installed package

- [ ] 4.1 **Needs the package** — build a git-stamped package with `make deb-local`, install it on `hogan`, and verify `vtpm-mds -version` and `dpkg -l vtpm-mds` both name this tree
      The agent on `hogan` can build; installing goes through the maintainer
      or `sudo hogan-lab dpkg-install` with one exact path.
- [ ] 4.2 **Needs the package** — repeat 1.1 and 1.2 from VM 399 and verify `signature` now answers identically to `GET /latest/no-such-thing`, with and without a session token
- [ ] 4.3 **Needs the package** — from VM 399, verify `GET /latest/dynamic/instance-identity/document` is byte-identical to the bytes saved in 1.1, and that `GET /latest/meta-data/instance-id` still answers `i-399`
- [ ] 4.4 **Needs the package** — from the host, which is an unbound caller, verify `document` is still refused with `404 page not found` and `signature` answers the same bytes

## 5. Validate and archive

- [x] 5.1 Run `go vet ./...` and `go test ./...` and verify both are clean, quoting the result
      **Done** on the Mac: `go vet` clean; `go test` → `ok` for `imds`,
      `internal/config`, `internal/devid`, `internal/inventory` and
      `internal/server`. Both were run over `go list ./...` minus
      `cmd/prox-mds`, an untracked, gitignored leftover in that checkout that
      does not build.

- [x] 5.2 Run `openspec validate --all --strict` and `scripts/openspec-validate.py --strict` and verify both pass, quoting the totals
      **Done** on the Mac: `openspec validate --all --strict` → 8 passed, 0
      failed. `scripts/openspec-validate.py --strict` → only "all tasks
      ticked", for this change and `serve-a-public-address-or-omit-the-key`,
      both still in progress.

- [ ] 5.3 **Coordinator** — archive with `openspec archive`, and verify afterwards that `Refuse An Instance Identity To Callers With No VM Record` in `openspec/specs/instance-metadata/spec.md` does not re-assert "`public-ipv4` is always unavailable, and the index is a static listing" if `serve-a-public-address-or-omit-the-key` has shipped
      This delta carries that sentence verbatim because correcting it is the
      other change's scope. If the other change archives first, this one's
      `MODIFIED` block restores the stale sentence on archive: amend this delta
      before archiving rather than editing the living spec by hand.
- [ ] 5.4 **Coordinator** — update the `AGENTS.md` known-gaps entry for the constant signature to point at the archive, and verify no entry still describes the signature as open
