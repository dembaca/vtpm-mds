# Tasks

Verification notes are indented under each task. Unless stated otherwise, they
were run in a worktree of `main` at `82dab20`, with both prerequisite changes
(`parse-peer-address-correctly`, `fail-fast-on-unusable-configuration`)
already merged.

## 1. Reproduce the defect

- [x] 1.1 Add a test that requests `/latest/meta-data/instance-id` from an unbound caller with `X-Forwarded-For: 10.9.9.9` and verify the current response is `200` with the body `i-10-9-9-9` — record that as the baseline
      Baseline recorded in commit `b61ffb1`, before any code change:
      `TestHandleInstanceID_UnboundCallerForwardedFor` PASSED asserting `200`
      and the body `i-10-9-9-9`. After the fix it fails against that
      assertion (`got "i-192-168-1-100"`) and is inverted: the header is
      ignored, and through the registered route the caller gets `404`.
- [x] 1.2 Add a test that requests `/latest/identity` as a caller bound to VM `100` with `X-Forwarded-For: 10.9.9.9` and verify the current claims report `i-10-9-9-9` rather than `i-100`, confirming that endpoint ignores the inventory entirely
      Baseline recorded in the same commit: `TestHandleIdentity_IgnoresInventory`
      PASSED asserting `sub` and `instance_id` of `i-10-9-9-9` for a caller
      bound to VM `100`. After the fix it fails (`got "i-100"`) and is
      inverted as `TestHandleIdentity_NamesTheBoundVM`.

## 2. Add the setting

- [x] 2.1 Add `RequireVMIdentity` to `internal/config` with the yaml key `require_vm_identity` and a built-in default of true, and verify `DefaultConfig()` reports true
      `go test ./internal/config/ -run TestDefaultConfig -v` → PASS, with the
      added assertion on `cfg.MDS.RequireVMIdentity`.
- [x] 2.2 Verify a configuration file that does not mention `require_vm_identity` loads it as true — this pins the ordering dependency on `fail-fast-on-unusable-configuration` and fails loudly if the defaults merge is ever lost
      `TestLoadOmittedRequireVMIdentityDefaultsToTrue` → PASS. The guard was
      checked by temporarily replacing `cfg := DefaultConfig()` in `Load` with
      `cfg := &Config{}`: the test then FAILED with "RequireVMIdentity must
      keep its built-in default (true) when the file omits it". The merge was
      restored and the test passes again.
- [x] 2.3 Verify an explicit `require_vm_identity: false` loads as false
      `TestLoadExplicitRequireVMIdentityFalse` → PASS.
- [x] 2.4 Document the setting in `config.yaml` and in `debian/vtpm-mds.8`, describing it as a migration aid, and verify the man page still parses with `groff -man -ww -z`
      `groff -man -ww -z debian/vtpm-mds.8` → no output, exit 0. The new
      CONFIGURATION section renders as expected under `man -l`, and
      `config.yaml` still parses as YAML with `require_vm_identity: true`.

## 3. Refuse unbound callers

- [x] 3.1 Wrap exactly the four identity-bearing route registrations in `internal/server/server.go` — `instance-id`, the instance identity document, its signature and `/latest/identity` — with a check that refuses a request whose connection has no bound VM record, and verify no other route is wrapped
      The refused set is data: `imds.IdentityBearingPaths`, consulted by the
      `handle` helper that registers the EC2 and identity routes, so a route
      is wrapped if and only if its path is in that set.
      `TestIdentityBearingPathsIsExactlyTheSpecifiedSet` pins the four paths
      and that `/health` and `/latest/api/token` are not among them. Confirmed
      on the running daemon (see 3.4 and 3.5): only those four answer `404`.
- [x] 3.2 Verify the refusal runs after token validation: an unbound caller with no token gets `401` with `IMDSv2 token required`, and with a valid token gets `404`
      `TestRefusalRunsAfterTokenValidation` → PASS. On the running daemon, an
      unbound caller with no token gets `HTTP/1.1 401 Unauthorized` with
      `X-Aws-Ec2-Metadata-Token: required` and the body `IMDSv2 token
      required`; with a valid token the same request gets `404`.
- [x] 3.3 Verify the refused response body is exactly `404 page not found`, identical to a request for an unregistered route
      `TestRefusalIsIndistinguishableFromUnknownRoute` → PASS. On the running
      daemon the refusal body is `404 page not found\n` (`od -c`:
      `4 0 4 space p a g e ... \n`), `cmp` against the body of
      `GET /latest/no-such-thing` reports the files identical, and both carry
      `Content-Type: text/plain; charset=utf-8`.
- [x] 3.4 Verify all four refused paths return `404` for an unbound caller: `/latest/meta-data/instance-id`, `/latest/dynamic/instance-identity/document`, `/latest/dynamic/instance-identity/signature` and `/latest/identity`
      `TestRefusedAndServedPaths` (three imds paths) and
      `identity.TestHandleIdentity_UnboundCallerRefused` → PASS. On the
      running daemon all four answered `404 page not found` to a loopback
      caller, which the inventory cannot bind.
- [x] 3.5 Verify the paths that are deliberately not refused still return `200` to an unbound caller — `/latest/meta-data/`, `local-hostname`, `local-ipv4`, `placement/availability-zone`, `services/domain` — with a table-driven test that pins both sides of the line, so a handler added to the wrong side fails rather than ships
      `TestRefusedAndServedPaths` is table-driven over every registered
      metadata path, asserting the side of the line, the status, and the value
      served (the index still lists `instance-id`; `local-ipv4` is the
      caller's own peer address). Checked that it bites: temporarily adding
      `/latest/meta-data/local-ipv4` to `IdentityBearingPaths` FAILED the
      table ("expected status 200, got 404") and
      `TestIdentityBearingPathsIsExactlyTheSpecifiedSet` ("expected exactly 4
      identity-bearing paths, got 5"). On the running daemon those five paths
      returned `200` to an unbound caller. `public-ipv4` is in the table too:
      it is not refused, but it answers `404` with an *empty* body — as it
      does for a bound caller, since the value is always unavailable — which
      is why it is not in the `200` list above.
- [x] 3.6 Verify `PUT /latest/api/token` and `GET /health` are served to an unbound caller
      `TestTokenAndHealthAreNeverRefused` → PASS. On the running daemon an
      unbound loopback caller minted a 44-character token and got
      `200 {"status":"ok"}` from `/health`.
- [x] 3.7 Verify a bound caller is served every metadata value exactly as before, by running the existing `imds` handler tests unchanged
      The pre-existing tests in `imds/handlers_test.go` are untouched;
      `go test ./imds/` → ok. `TestRefusedAndServedPaths` additionally drives
      every path as a caller bound to VM `100` and requires `200` (and the
      pre-existing `404` for `public-ipv4`) with no refusal body.

## 4. Stop headers from naming the caller

- [x] 4.1 Remove the `X-Forwarded-For` branch from `getClientIP` in `imds/handlers.go` and from its copy in `identity/handlers.go`, and verify `grep -rn 'X-Forwarded-For' imds identity` returns nothing outside tests
      `identity.getClientIP` is deleted outright; `imds.getClientIP` keeps
      only the peer-address path. `grep -rn 'X-Forwarded-For' imds identity |
      grep -v '_test.go'` → no output.
- [x] 4.2 Replace `identity.getInstanceID` with the same inventory-backed derivation the metadata handler uses, and verify the task 1.2 test now reports `i-100`
      `identity.getInstanceID` is deleted; the handler calls `imds.InstanceID`
      and `imds.LocalIPv4`. `TestHandleIdentity_NamesTheBoundVM` → PASS with
      `sub` and `instance_id` `i-100`, the `ip` claim the peer address, and
      `10.9.9.9` absent from the whole document.
- [x] 4.3 Verify the fallback path with `require_vm_identity: false`: an unbound caller from `127.0.0.1` sending `X-Forwarded-For: 10.9.9.9` receives `i-127-0-0-1`
      `TestFallbackInstanceIDWithoutRequireVMIdentity` and
      `identity.TestHandleIdentity_FallbackWithoutRequireVMIdentity` → PASS.
      On a daemon started with `require_vm_identity: false`, a loopback caller
      sending `X-Forwarded-For: 10.9.9.9` got `200 i-127-0-0-1` from
      `instance-id`, and `sub`/`instance_id` `i-127-0-0-1` from
      `/latest/identity`.
- [x] 4.4 Verify the `ip` claim of the identity document is the peer address and is the empty string when the peer has no IPv4 address
      `TestHandleIdentity_IPClaimForPeerWithoutIPv4` → PASS: `ip` is `""` for
      a peer of `[fe80::1]:5000`, and `i-100` is still reported.
      `TestHandleIdentity_NamesTheBoundVM` covers the IPv4 case.

## 5. Make the refusal diagnosable

- [x] 5.1 Log one line per refused request naming the caller's peer address, and verify it appears once for a refused metadata read and identifies the address
      `TestRefusalIsLogged` → PASS: exactly one line, naming the peer address
      and the refused path. On the running daemon:
      `Refusing GET /latest/meta-data/instance-id: no VM record bound to
      caller 127.0.0.1:40904`, one per refused request.

## 6. Confirm end to end

- [x] 6.1 Run `go vet ./...` and `go test ./...` and verify both pass, reporting the output
      `go vet ./...` → no output, exit 0. `go test ./...` → ok for `attest`,
      `identity`, `imds`, `internal/config`, `internal/devid`,
      `internal/inventory`, `internal/server`; no failures.
- [ ] 6.2 Run `scripts/qemu-lab/e2e-netns.sh` and verify it still reports `i-100`, since its netns MAC is in the lab inventory — needs root on the lab host, so record it as maintainer-run if it cannot be executed here
      Not run: needs root on the lab host; maintainer-run.
- [ ] 6.3 Verify an unbound caller is refused on the running service by requesting the instance id from a netns whose MAC is not in the inventory and confirming `404` plus the log line from task 5.1, and that the same caller still gets `200` on `local-ipv4` — needs root, record who ran it
      Not run: needs root on the lab host; maintainer-run.
      Partial substitute without root: the daemon built from this tree was run
      on `127.0.0.1:18099`, where a loopback caller has no ARP entry and so is
      unbound. It was refused `404` on all four identity-bearing paths, with
      one log line each naming its peer address, and still got `200` on
      `local-ipv4` (its own address), the index, `local-hostname`,
      `placement/availability-zone` and `services/domain`. This used a
      `go build` binary, not a git-stamped package, so it does not replace the
      packaged netns run.
- [x] 6.4 Verify DevID enrollment still refuses an unbound caller with `401` and `VM identity required (MAC not in inventory)`, so this change did not flatten the two refusals into one
      `internal/devid` is untouched by this change, and its routes are
      registered by `devid.Register`, which the new `handle` helper does not
      wrap (their paths are not in `IdentityBearingPaths`).
      `go test ./internal/devid/ -run TestAuthenticateEnrollCaller` → PASS
      unchanged, including the "expected unauthorized without VM" case, which
      `AuthenticateEnrollCaller` returns as
      `errUnauthorized("VM identity required (MAC not in inventory)")` and
      `writeEnrollAuthError` maps to `http.StatusUnauthorized`. Verified at
      unit and source level; not measured over the wire, which belongs to the
      root-only run in 6.3.
