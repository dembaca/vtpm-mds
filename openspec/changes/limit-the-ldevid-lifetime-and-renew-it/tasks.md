## 1. Implementation (after coordinator ACK of this record)

- [ ] 1.1 Add `mds.devid_lifetime` to config (default `2160h`, fail load on bad/non-positive values); verify with `go test ./internal/config/...` (omit / `48h` / `90` / `0s`).
- [ ] 1.2 Thread the duration into the DevID CA and set `notAfter = notBefore.Add(lifetime)` in `IssueDevID`; verify with `go test ./internal/devid/...` (default and configured lifetime).
- [ ] 1.3 Guest client renew-or-skip (one-third / expired / missing) reusing existing TPM DevID blobs; verify decision-logic unit tests with a fixed clock.
- [ ] 1.4 Guest package oneshot + daily timer (`Before=spire-agent.service`), enable timer in `postinst`, man page; verify `dpkg-deb -c` lists the units and `systemd-analyze verify` where available.
- [ ] 1.5 Document `devid_lifetime: 2160h` in packaged host `config.yaml`; verify `go test` that loads `debian/config.yaml` still passes.

## 2. Lab verification (after code)

- [ ] 2.1 Issue with default lifetime; verify `openssl x509 -dates` shows `notAfter ≈ now+90d`.
- [ ] 2.2 Short lifetime + renew oneshot; verify same priv/pub blob checksums, new cert PEM, new SHA-1 DER fingerprint.
- [ ] 2.3 Boot order oneshot before `spire-agent` and re-attest after renew; verify journal / `spire-server agent list` shows a new `tpm_devid/<fingerprint>` while CN selectors still match.
- [ ] 2.4 Confirm SPIRE 1.14.0 `agent evict` / `agent purge` flags match design D-e; verify by re-running `-help` on the lab's `/opt/spire/bin/spire-server`.

## 3. OpenSpec

- [ ] 3.1 Before merge, verify `openspec validate --all --strict` is green on the PR branch.
