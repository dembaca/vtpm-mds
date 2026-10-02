# Design

## Context

See proposal.md — Why.

`debian/rules` has:

```
override_dh_installsystemd:
	dh_installsystemd --no-start
```

with a comment explaining the intent: enable for the next boot, but do not
start during `dpkg -i`, because binding `169.254.169.1:80` fails on hosts that
lack the IMDS bridge. That intent is correct and stays.

What was not intended is the side effect. `dh_installsystemd` splits its
`postinst` work across autosnippets, and `systemctl daemon-reload` lives in the
start/restart one. Suppressing starts therefore also suppresses the reload.
The generated `postinst` in the built package contains only
`deb-systemd-helper unmask`, `was-enabled`, `enable` and `update-state` — no
`systemctl` call at all, which is verifiable with `dpkg-deb -e`.

The README globs are a separate, older habit from when `dist/` only ever held
one build. `make deb-local` writes a git-stamped version next to whatever
`make deb` left, so the directory routinely holds two.

## Goals / Non-Goals

**Goals:**

- A `dpkg -i` that replaces the unit leaves systemd holding the new one.
- The documented install command installs exactly one, named package.
- `--no-start` keeps meaning what it says.

**Non-Goals:**

- Starting or restarting the daemon from the package. That is the existing
  policy and the reason `--no-start` is there; changing it would break
  installs on build hosts.
- Changing the unit's directives, the package contents, or the split between
  host and guest packages.
- Cleaning `dist/` automatically, or having `make deb-local` delete older
  builds. Keeping both is useful; the fix is to name what you install.
- Teaching the Makefile to install. It already prints the exact command with
  the full filename, which is the behaviour the README should match.

## Decisions

### Reload in `debian/rules`, next to the `--no-start` it compensates for

The override becomes `dh_installsystemd --no-start` plus an explicit reload
snippet, so the two sit together and a reader sees why the reload is spelled
out by hand. Putting it in `debian/postinst` directly would separate the
compensation from the flag that makes it necessary, and `debian/postinst`
already carries project-specific logic that is easier to review when it is not
mixed with debhelper plumbing.

The reload is guarded on `[ -d /run/systemd/system ]`, the same test
debhelper's own snippets use, and ends in `|| true`: a host without a running
manager, or a manager that refuses the reload, must not fail the installation.

Dropping `--no-start` and letting debhelper emit its full snippet was rejected
outright. It would start the daemon on every install, which is the thing the
flag exists to prevent, and it would fail the build-host installs that
`debian-packaging` already covers.

### README names one package and shows how to pick it

The install lines become an explicit filename. For the local git-stamped build
the README shows the `ls -t … | head -1` form restricted to `*+git*`, which
picks the newest stamped build and cannot match a release package.

Telling readers to clean `dist/` first was rejected: it makes the correct
command depend on a prior step that is easy to skip, and the failure mode when
it is skipped is silent.

## Risks / Trade-offs

- **[Risk] The reload makes systemd pick up a unit whose daemon is running
  from the previous binary, so the running process and the loaded unit
  disagree until the next restart** → Accepted, and it is the normal Debian
  behaviour the `--no-start` policy implies. It is also strictly better than
  today, where the *unit* disagrees too. The operator restarts when ready.
- **[Risk] `dh_installsystemd`'s snippet layout changes in a future debhelper
  and the manual reload becomes a duplicate** → Mitigation: a duplicate
  `daemon-reload` is harmless, and the verification task inspects the built
  `postinst` rather than assuming, so a change in layout is visible.
- **[Trade-off] The README install line is longer and less pretty than a
  glob** → Accepted. The glob's brevity is what cost a maintainer a wrong
  version reading on 2026-10-02.

## Migration Plan

None beyond installing the new package. The first `dpkg -i` of a package
carrying the fix still leaves systemd holding the *old* unit, because the old
package's `postinst` is what runs for the removal half — from the next install
onwards the reload happens. Operators upgrading across this change should run
`systemctl daemon-reload` once by hand, and the task list says so.

## Open Questions

None.
