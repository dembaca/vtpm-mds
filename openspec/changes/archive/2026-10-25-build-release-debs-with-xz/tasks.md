# Tasks

Written on 2026-10-24 against `main@3a12af3` by
`agent-vtpm-hogan-d3e6cbc82`, at the maintainer's request (ask on the bus,
answer A). Files: `debian/rules`, `.github/workflows/deb.yml` and this
record. Not product code, not `AGENTS.md`/`README.md`.

## 1. Build

- [x] 1.1 Add `override_dh_builddeb` with `dh_builddeb -- -Zxz` to
      `debian/rules` (D-a); verify with `make deb` on hogan that both
      packages build, `ar t` lists `control.tar.xz` and `data.tar.xz`, and
      `lintian --fail-on error,warning` exits 0 for both
      Done 2026-10-24 on hogan. To stand in for Ubuntu's default, both builds
      ran with `DPKG_DEB_COMPRESSOR_TYPE=zstd`: without the override both
      packages came out `control.tar.zst`/`data.tar.zst`; with it the log
      shows `dpkg-deb --root-owner-group -Zxz --build` and both are
      `control.tar.xz`/`data.tar.xz`, lintian exit 0 for both.
- [x] 1.2 Verify `dpkg-deb -c` of both packages lists the same paths as
      before the change (same tree, `make deb` without the override)
      Identical for both packages (`dpkg-deb -c` mode, owner and path,
      sorted, against the baseline build of `main` plus this record).
- [x] 1.3 Add a CI step after "Record package metadata" that fails unless
      `ar t` of each `.deb` lists `control.tar.xz` and `data.tar.xz`
      (D-b); verify the step locally with the same commands on the hogan
      build and on the zstd `v0.3.0` asset (must fail)
      The step's `run:` extracted from the workflow and run with `bash`:
      exit 0 on the hogan xz build, exit 1 on the `v0.3.0` assets (`has no
      control.tar.xz member`). A first version with a `case` pattern
      rejected the xz build as well and was replaced before commit.
- [x] 1.4 Run `openspec validate --all --strict`
      Totals: 6 passed, 0 failed (6 items)
## Pre-review

- Fable, 2026-10-24 (`1acd593`): no must-fix, no should-fix. Confirmed the
  override reaches `dpkg-deb` for both packages under compat 13 and
  `Rules-Requires-Root: no`, that `make deb` is the only build entry point,
  and that the check fails closed and runs before both uploads, so the
  release job (`needs: build-deb`, artifacts only) cannot publish. Nit
  taken: `grep -qxF`, so the member names are matched literally.

- [x] 1.5 The PR's CI run passes the new step on `ubuntu-24.04`
      "Debian package" passed on the merge `a3f33bc` (#81) and on every later
      `main` commit; the tag run for `v0.3.1` published both `.deb` with
      members `control.tar.xz` and `data.tar.xz`, lintian exit 0, checked from
      the release assets on 2026-10-25.

## 2. Archive

- [x] 2.1 After merge, the coordinator archives the change
      Archived 2026-10-25 by `agent-vtpm-hogan-d3e6cbc82` at the coordinator's
      request (bgl-infra TASK 0.3.2r).
