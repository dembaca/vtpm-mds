# Tasks

Written on 2026-10-19 by `agent-cursor-vtpm-22e0` against `main@da0083a`
(record) / rebase onto `main` after #70 archives before code. Files in scope
after code-go: `internal/devid/`, `internal/config/`, `cmd/devid-enroll/`,
guest `debian/` units + man page, packaged host `config.yaml` comment.
**Not** root `README.md`, **not** `AGENTS.md`, **not** host daemon logic beyond
the config key.

## 1. Implementation (after coordinator code-go)

- [x] 1.1 Add `mds.devid_lifetime` (default `2160h`, reject bad/non-positive/`>8760h`); verify with `go test ./internal/config/...`.
- [x] 1.2 Thread lifetime into the CA; set `notAfter = notBefore.Add(lifetime)`; include `notBefore`/`notAfter` in `uniqueSerial`; verify `go test ./internal/devid/...` including same-key renew → new serial.
- [x] 1.3 Guest client renew-or-skip (one-third / expired / missing), reuse TPM blobs, clear SPIRE agent SVID cache on PEM change + `try-restart`; verify decision-logic unit tests with a fixed clock.
- [x] 1.4 Guest package oneshot + daily timer (`Before=spire-agent`, no `Requires=`), enable timer in `postinst`, man page including E2.4 `evict`/`purge`; verify `dpkg-deb -c` and `systemd-analyze verify` where available.
- [x] 1.5 Document `devid_lifetime: 2160h` (and the one-year cap) in packaged host `config.yaml`; verify tests that load `debian/config.yaml` still pass.

## 2. Lab / hogan verification (after code)

- [x] 2.1 Cloud lab: default lifetime → `notAfter ≈ now+90d`; verify with `openssl x509 -dates`.
- [x] 2.2 Cloud lab: short lifetime + renew; verify same priv/pub blob checksums, new cert PEM, new SHA-1 fingerprint, new serial; SVID cache cleared so next agent start attests the new ID.
- [ ] 2.3 Hogan VM 399: renew into a **scratch** output directory (do not overwrite live `/var/lib/spire/agent/` materials); verify new cert shape / serial; confirm SPIRE's live materials unchanged. Verify with `openssl` + `sha1sum` / journal as appropriate.
- [x] 2.4 Confirm SPIRE 1.14.0 `agent evict` / `agent purge` flags still match design D-e; verify via `-help` on lab or hogan.

## 3. OpenSpec / apply order

- [ ] 3.1 After #70 merges and archives, rebase this branch onto that `main`; verify the REMOVED title is `Issue A Non-Expiring LDevID Whose Subject Is Only The VM ID`.
- [x] 3.2 Before merge, verify `openspec validate --all --strict` is green.

## 4. Not verified in this record (do not tick as done)

- [ ] 4.1 Clock-skewed “LDevID expired while agent keeps running” was **inferred** from the cached-SVID path (lab A: swapped LDevID ignored) rather than by forging an expired PEM under a live agent; verify explicitly in 2.2 if reviewers require it.
- [ ] 4.2 Hogan end-to-end renew against the **live** SPIRE agent paths (399) is deferred until code-go; verify only the scratch-directory check in 2.3 was planned in this record.
