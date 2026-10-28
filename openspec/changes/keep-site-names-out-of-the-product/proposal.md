# Proposal

## Why

vtpm-mds is meant to be used by other operators, but the repository carries
the names of the one site it was built on:
- Tests build `/etc/pve` trees for nodes `hogan` and `holger` in cluster
  `bgl-cluster`, use VM ID `399` and that VM's real MAC addresses, and place
  hosts in `DUS4` or `keller01`.
- The specs' scenarios repeat those names.
- `devid-enroll(1)` gives an example "on hogan VM 399".
- The README's E2E cheat sheet is written for one host, one guest and one CA
  name.

No code path decides anything on these names. But the repository reads as if
the software were built for one setup, and a third party cannot tell an
example from an assumption. On 2026-10-26 the maintainer asked that nothing
specific to this setup be built into the product, and that this be written
into a spec.

## What Changes

- `operability` gains a requirement: no host, cluster, location, VM ID, MAC
  address or CA of a particular operator appears in code, tests, spec
  scenarios, man pages, shipped configuration or the README. Examples use
  neutral names.
- Every current occurrence is replaced with a neutral name (design D-a). The
  scenarios of six spec requirements use site names; those requirements are
  restated with the neutral names, and nothing else in them changes.
- The README's E2E cheat sheet uses placeholders (`<vmid>`, `<guest>`) instead
  of the lab's host, guest and CA.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `operability`: add a requirement that the product names no particular
  operator's site, and restate one requirement with a neutral VM ID.
- `vm-inventory`, `instance-metadata`: restate two requirements each with
  neutral names in their scenarios; behaviour unchanged.
- `devid-enrollment`: restate one requirement with a neutral VM ID.

## Non-goals

- No behaviour change: no flag, default, endpoint, status code or wire format.
  The default bridge name `vmbr_imds` stays. It is the product's documented
  convention, not a site's name, and `-bridge` overrides it.
- `openspec/changes/archive/` and `debian/changelog` stay as they are. They
  are the project's history, not the product.
- The maintainer's name and address in `debian/control`, `debian/copyright`,
  `README.md` and `ARCHITECTURE.md` stay.
- Verification records in an open change's `tasks.md` keep naming the host and
  VM they were run on.

## Impact

- **Code and docs:**
  - tests under `imds/` and `internal/`
  - comments in `internal/proxmox/vmconfig.go` and the TPM simulator tests
  - `debian/devid-enroll.1`, `README.md`, and `AGENTS.md`, which keeps only rules
    and still-valid pitfalls (design D-d)
  - comments and prose only in `scripts/deb-local.sh`,
    `scripts/openspec-validate.py` and `scripts/qemu-lab/README.md`
- **Specs:** `operability` gets an ADDED requirement. `vm-inventory`,
  `instance-metadata`, `operability` and `devid-enrollment` are MODIFIED, in
  scenario names only.
- **Ordering:** `read-cluster-ek-pins-from-pmxcfs` (PR #90, merged, not yet
  archived) neutralised its own tests. Its spec delta already MODIFIES
  `Refuse An EK Pinned To Another VM`, so the VM IDs of that requirement are
  renamed inside that delta (design D-b), not in a second MODIFIED block.
