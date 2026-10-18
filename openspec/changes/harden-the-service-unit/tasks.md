# Tasks

Written on 2026-10-18 against `main` at `407bfca` by the coordinator on hogan,
from the production-readiness list. **Unit changes only after the record is
pushed and the maintainer has not overruled D-a to D-g; D-f needs his answer.**

## 1. Decide

- [ ] 1.1 **Maintainer, optional** — overrule or accept D-a to D-g in `design.md`, and answer D-f (dedicated user now or later), and verify the outcome is recorded there

## 2. Implement

- [ ] 2.1 Add the directives of D-a to D-e to `debian/vtpm-mds.service`, and verify `systemd-analyze verify` on the file reports nothing new
- [ ] 2.2 Add the operator note to `debian/README.Debian`, and verify the package still passes `lintian --fail-on error,warning`
- [ ] 2.3 Run `openspec validate --all --strict`, and verify it passes

## 3. Verify on hogan

- [ ] 3.1 Install the git-stamped package (with the maintainer's go in the session), start the unit, and verify `systemd-analyze security vtpm-mds` is at most 4.0; record the value and the three highest remaining exposures
- [ ] 3.2 Verify from VM 399 that the metadata tree, tags and placement still answer, and from the host that an unbound caller still gets its refusals
- [ ] 3.3 Reload with `systemctl reload vtpm-mds`, and verify the journal shows the cache reloaded with the same VM count
- [ ] 3.4 Re-enroll VM 399 with `devid-enroll` (with the maintainer's go), and verify a certificate is issued, which proves the CA key is still readable under the new bounding set
- [ ] 3.5 Say what was not verified

## 4. Archive

- [ ] 4.1 Archive with `openspec archive`, update `AGENTS.md`, and verify `openspec validate --all --strict` is still clean
