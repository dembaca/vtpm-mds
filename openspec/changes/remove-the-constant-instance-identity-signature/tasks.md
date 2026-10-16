# Tasks

Nothing is ticked: this is a planning record, written on 2026-10-16 against
`main` at `7db05db`. No code has been touched.

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

- [ ] 1.1 From VM 399 with a valid session token, read `GET /latest/dynamic/instance-identity/signature` against the installed daemon and verify it answers `200`, `Content-Type: text/plain`, body `dGVzdC1zaWduYXR1cmU=`
      Use `sudo hogan-lab qm guest exec 399 …`; VM 399 has `python3` and no
      `curl`. Record `vtpm-mds -version` alongside, so the baseline names the
      tree it came from, and save the raw bytes of `GET .../document` from the
      same guest: 4.3 compares against them.
- [ ] 1.2 From the same guest, read `.../instance-identity/pkcs7`, `.../instance-identity/rsa2048`, the listing `.../instance-identity/` and `GET /latest/no-such-thing`, and verify all four answer identically in status, `Content-Type` and body bytes
      This is the state the removed path is meant to join. If any of the four
      differs, stop: the delta's claim that the sibling paths already fall
      through to the catch-all is wrong.
- [ ] 1.3 Re-read `get_instance_identity` in cloud-init's `cloudinit/sources/helpers/ec2.py` at implementation time and verify it still reaches leaves only through the `dynamic/instance-identity/` listing, never by requesting `signature` by name
      `design.md` relies on this reading from 2026-10-16. cloud-init changes;
      cite the commit you read. If it now requests `signature` directly, the
      change goes back to the maintainer before any code is touched.
- [ ] 1.4 Run `git grep -n 'instance-identity/signature\|dGVzdC1zaWduYXR1cmU\|HandleInstanceIdentitySignature'` over the whole tree, including `scripts/qemu-lab/` and every `_test.go`, and verify every hit is listed under Impact in `proposal.md`
      Expected: the registration, the handler, `IdentityBearingPaths`, four
      places in `imds/handlers_test.go`, the `instance-metadata` spec,
      `README.md`, `ARCHITECTURE.md`, `debian/vtpm-mds.8` (as
      `instance\-identity/signature`, so grep for that spelling too), the
      `config.yaml` comment, and `AGENTS.md`. **A hit outside that list is a
      caller nobody knew about**: record it and amend Impact before going on,
      as `remove-the-unverifiable-identity-endpoints` task 1.3 had to.

## 2. Remove the route

- [ ] 2.1 Delete the `GET /latest/dynamic/instance-identity/signature` registration from `internal/server/server.go` and `HandleInstanceIdentitySignature` from `imds/handlers.go`, and verify `go build ./...` succeeds and the grep from 1.4 no longer finds the handler name
- [ ] 2.2 Remove the signature path from `IdentityBearingPaths` and update the doc comment above it to say the path was removed rather than leaving it to be inferred — verify `go test ./imds/` passes and the comment names no route that is not registered
- [ ] 2.3 Replace the signature row in the refused-path table, the signature route in the test mux and `TestSignatureServedToPeerWithoutIPv4` with a test that `GET /latest/dynamic/instance-identity/signature` answers exactly what `GET /latest/no-such-thing` answers — for a bound caller with a token, an unbound caller with a token, and a caller with no token — and verify the test fails when the registration is put back
      Run the test against the server's real mux, not `newTestMux`, or it
      proves nothing about the catch-all. "Fails when put back" is the part
      that shows the test tests something; say how you checked it.
- [ ] 2.4 Pin `TestIdentityBearingPathsIsExactlyTheSpecifiedSet` to the two remaining paths and verify every existing document test passes **unchanged**
      If a document test needs editing, the document's behaviour moved and
      this change broke its own non-goal.

## 3. Correct the documentation

- [ ] 3.1 Remove the signature row from `ARCHITECTURE.md`'s key-endpoints table, the signature path from the `require_vm_identity` entry in `debian/vtpm-mds.8`, and "both … endpoints" from the `config.yaml` comment — verify the grep from 1.4 finds none of them, and that `man -l debian/vtpm-mds.8` still renders that entry as one sentence
      The man page lists the paths as "a, b and c"; with two it becomes "a and
      b", so the `.I`/`.IR` macros around the joining words change too.
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

- [ ] 5.1 Run `go vet ./...` and `go test ./...` and verify both are clean, quoting the result
- [ ] 5.2 Run `openspec validate --all --strict` and `scripts/openspec-validate.py --strict` and verify both pass, quoting the totals
- [ ] 5.3 **Coordinator** — archive with `openspec archive`, and verify afterwards that `Refuse An Instance Identity To Callers With No VM Record` in `openspec/specs/instance-metadata/spec.md` does not re-assert "`public-ipv4` is always unavailable, and the index is a static listing" if `serve-a-public-address-or-omit-the-key` has shipped
      This delta carries that sentence verbatim because correcting it is the
      other change's scope. If the other change archives first, this one's
      `MODIFIED` block restores the stale sentence on archive: amend this delta
      before archiving rather than editing the living spec by hand.
- [ ] 5.4 **Coordinator** — update the `AGENTS.md` known-gaps entry for the constant signature to point at the archive, and verify no entry still describes the signature as open
