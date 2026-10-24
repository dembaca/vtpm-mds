# Tasks

Written on 2026-10-24 against `main@3a12af3` by
`agent-vtpm-hogan-d3e6cbc82`, at the maintainer's request (ask on the bus,
answer A). Files: `debian/rules`, `.github/workflows/deb.yml` and this
record. Not product code, not `AGENTS.md`/`README.md`.

## 1. Build

- [ ] 1.1 Add `override_dh_builddeb` with `dh_builddeb -- -Zxz` to
      `debian/rules` (D-a); verify with `make deb` on hogan that both
      packages build, `ar t` lists `control.tar.xz` and `data.tar.xz`, and
      `lintian --fail-on error,warning` exits 0 for both
- [ ] 1.2 Verify `dpkg-deb -c` of both packages lists the same paths as
      before the change (same tree, `make deb` without the override)
- [ ] 1.3 Add a CI step after "Record package metadata" that fails unless
      `ar t` of each `.deb` lists `control.tar.xz` and `data.tar.xz`
      (D-b); verify the step locally with the same commands on the hogan
      build and on the zstd `v0.3.0` asset (must fail)
- [ ] 1.4 Run `openspec validate --all --strict`
- [ ] 1.5 The PR's CI run passes the new step on `ubuntu-24.04`

## 2. Archive

- [ ] 2.1 After merge, the coordinator archives the change
