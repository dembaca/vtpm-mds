# Tasks

Written on 2026-10-26 against `main@e54344d` by the coordinator, at the
maintainer's request (thread `yz8nmijh7i8ifmjqhraqzk13zy`).

## 1. Rename

- [ ] 1.1 Replace the site names in tests under `imds/` and `internal/` with
      D-a's names; verify `go vet ./...` and `go test ./...`
- [ ] 1.2 Replace them in comments in `internal/devid/pin.go` and
      `internal/proxmox/vmconfig.go`, in `debian/devid-enroll.1`, and in the
      prose of `scripts/deb-local.sh`, `scripts/openspec-validate.py` and
      `scripts/qemu-lab/README.md`
- [ ] 1.3 Rewrite the README's E2E cheat sheet with placeholders (`<vmid>`,
      `<guest>`, and `<DevID CA>` for the CA's subject)
- [ ] 1.4 Write the spec deltas per D-b; verify
      `openspec validate --all --strict`
- [ ] 1.5 Verify the grep is empty outside D-c's exceptions:
      `git grep -nIiwE 'hogan|holger|bgl|bgl-cluster|DUS[0-9]?|keller(01)?|vtpm-pilot|399' -- ':!AGENTS.md' ':!openspec/changes' ':!debian/changelog'`
      and
      `git grep -niE 'bc:24:11:(75:da:c0|06:1d:b2)' -- ':!AGENTS.md' ':!openspec/changes'`

## Pre-review

- [ ] Review by a stronger model before the PR is called ready

## 2. Archive

- [ ] 2.1 After merge, the coordinator archives the change
