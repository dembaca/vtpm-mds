# Proposal

## Why

`scripts/openspec-validate.py` rejects a valid spec delta. It collects
requirement blocks with a single `re.split` on `### Requirement:` across the
whole document, without regard to the `## ADDED` / `## MODIFIED` / `## REMOVED`
section each block sits under, and then insists every block carry at least one
`#### Scenario:`. A `REMOVED` requirement correctly carries **Reason** and
**Migration** instead, so every change that removes a requirement is reported
as broken.

Six changes proposed on 2026-09-20 remove a requirement, because each replaces
a requirement whose scenarios state a defect as the contract. All six pass
`openspec validate --all --strict` and all six fail the fallback script, on
requirements the upstream tool is satisfied with. A checker that fails on
correct input is worse than no checker: the next person reads the failures,
finds them spurious, and stops reading its output.

Separately, `AGENTS.md` states that the lab hosts have no node/npm and that the
OpenSpec CLI therefore cannot be installed there. On `hogan` the CLI is
installed at `/usr/local/bin/openspec` (1.13.1) with node v20.19.2, and
`openspec validate --all --strict` runs. The instruction that sends an agent to
the fallback script first is wrong about its own premise.

## What Changes

- `scripts/openspec-validate.py` parses `## ADDED` / `## MODIFIED` /
  `## REMOVED` / `## RENAMED` sections and applies the scenario rule only where
  it belongs: `ADDED` and `MODIFIED` requirements need a scenario, `REMOVED`
  requirements need **Reason** and **Migration**, and `RENAMED` entries need
  FROM/TO.
- The script gains a check it should always have had: a `MODIFIED` requirement
  must not drop a scenario the living spec still has, which is the rule the
  upstream CLI enforces and the one most likely to lose behaviour at archive
  time.
- `AGENTS.md` and `README.md` record that the CLI is installed on `hogan` and
  that `openspec validate --all --strict` is the check to run there, with the
  script as the fallback for a host without node — which is what it was written
  for.
- No change to the daemon, the client, the packaging, or any spec.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This change alters a development-time checker and a contributor
instruction; no behaviour of the service changes, so `.openspec.yaml` sets
`skip_specs: true`.

## Impact

- `scripts/openspec-validate.py`.
- `AGENTS.md`, the "Always start here" step 4 and the "Lab host facts" section.
- `README.md`, the comment above the `scripts/openspec-validate.py` invocation,
  which repeats the same wrong claim that lab hosts cannot run the CLI.
- Anyone running the fallback validator, and the six changes proposed on
  2026-09-20 that it currently rejects.
