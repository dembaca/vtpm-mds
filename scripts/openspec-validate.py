#!/usr/bin/env python3
"""Structural check for openspec/ when the `openspec` CLI is unavailable.

The upstream CLI (`openspec validate --all --strict`) is an npm package. It is
installed on `hogan` and is what to run there; this script is the fallback for
a host without node. It covers the structure the CLI enforces: required
documents, task checkboxes with a verification clause, and spec deltas whose
requirements carry what their delta operation requires.

Each delta operation is checked by its own rule. ADDED and MODIFIED
requirements need at least one scenario; REMOVED requirements carry **Reason**
and **Migration** instead and must NOT be asked for one; RENAMED entries need
FROM:/TO:. A MODIFIED requirement must also keep every scenario the living spec
still has, because archive replaces the whole block and a partial MODIFIED
silently loses behaviour.

This is a floor, not a replacement: run the real CLI where it exists.

Usage: scripts/openspec-validate.py [--strict]
  --strict  also fail when a change has unticked tasks
"""

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
PROPOSAL_HEADINGS = ("## Why", "## What Changes", "## Capabilities", "## Impact")
DELTA_SECTION = re.compile(r"^## (ADDED|MODIFIED|REMOVED|RENAMED) Requirements", re.M)
SPEC_SECTION = re.compile(r"^## Requirements", re.M)

failures: list[str] = []


def check(cond: bool, msg: str) -> None:
    print(f"{'PASS' if cond else 'FAIL'}  {msg}")
    if not cond:
        failures.append(msg)


SECTION_SPLIT = re.compile(r"^## (ADDED|MODIFIED|REMOVED|RENAMED) Requirements\s*$", re.M)


def _requirement_blocks(text: str) -> list[tuple[str, str]]:
    """Return (name, body) for each `### Requirement:` block in text."""
    blocks = []
    for block in re.split(r"^### Requirement: ", text, flags=re.M)[1:]:
        blocks.append((block.splitlines()[0].strip(), block))
    return blocks


def _scenario_names(body: str) -> list[str]:
    return [s.splitlines()[0].strip() for s in re.split(r"^#### Scenario: ", body, flags=re.M)[1:]]


def _check_scenarios(capability: str, name: str, body: str) -> None:
    scenarios = re.split(r"^#### Scenario: ", body, flags=re.M)[1:]
    check(bool(scenarios), f"{capability}/{name}: has at least one scenario")
    for scenario in scenarios:
        check(
            bool(re.search(r"\*\*(GIVEN|WHEN|THEN)\*\*", scenario)),
            f"{capability}/{name}: scenario uses GIVEN/WHEN/THEN",
        )


def _living_spec_scenarios(capability_path: Path) -> dict[str, list[str]]:
    """Scenario names per requirement in the living spec, or {} if there is none.

    Used to enforce the rule the CLI states as "archive refuses to drop them":
    a MODIFIED requirement replaces the whole block, so a scenario it omits is
    behaviour lost at archive time.
    """
    spec = ROOT / "openspec" / "specs" / capability_path / "spec.md"
    if not spec.is_file():
        return {}
    return {name: _scenario_names(body) for name, body in _requirement_blocks(spec.read_text())}


def check_delta(path: Path, capability_path: Path) -> None:
    """Check a change delta, applying each section's own rule."""
    capability = str(capability_path)
    text = path.read_text()
    check(bool(DELTA_SECTION.search(text)), f"{capability}: has an ADDED/MODIFIED/REMOVED section")
    check("SHALL" in text, f"{capability}: uses normative SHALL")

    parts = SECTION_SPLIT.split(text)
    sections = list(zip(parts[1::2], parts[2::2]))
    check(bool(sections), f"{capability}: has at least one delta section")

    living = _living_spec_scenarios(capability_path)
    seen_any = False
    for op, body in sections:
        blocks = _requirement_blocks(body)
        if op == "RENAMED":
            # RENAMED entries are FROM:/TO: pairs, not requirement blocks.
            check(
                bool(re.search(r"^\s*-?\s*FROM:", body, re.M))
                and bool(re.search(r"^\s*-?\s*TO:", body, re.M)),
                f"{capability}: RENAMED section uses FROM:/TO:",
            )
            seen_any = True
            continue
        for name, block in blocks:
            seen_any = True
            if op == "REMOVED":
                # A REMOVED requirement documents why it goes and what to do
                # instead; asking it for a scenario is what this script got
                # wrong before, and it rejected every correct removal.
                check(
                    "**Reason**" in block,
                    f"{capability}/{name}: REMOVED requirement states **Reason**",
                )
                check(
                    "**Migration**" in block,
                    f"{capability}/{name}: REMOVED requirement states **Migration**",
                )
                continue
            _check_scenarios(capability, name, block)
            if op == "MODIFIED" and name in living:
                dropped = [s for s in living[name] if s not in _scenario_names(block)]
                check(
                    not dropped,
                    f"{capability}/{name}: MODIFIED keeps the living spec's scenarios "
                    f"(dropped: {dropped})",
                )
    check(seen_any, f"{capability}: has '### Requirement:' headers")


def check_spec(path: Path) -> None:
    """Check a living spec under openspec/specs/, which has no delta sections."""
    capability = path.parent.name
    text = path.read_text()
    check(bool(SPEC_SECTION.search(text)), f"{capability}: has a '## Requirements' section")
    check(
        not DELTA_SECTION.search(text),
        f"{capability}: is a living spec, not a delta (no ADDED/MODIFIED/REMOVED heading)",
    )
    check("SHALL" in text, f"{capability}: uses normative SHALL")
    blocks = _requirement_blocks(text)
    check(bool(blocks), f"{capability}: has '### Requirement:' headers")
    for name, block in blocks:
        _check_scenarios(capability, name, block)


def check_change(change: Path, strict: bool) -> None:
    print(f"--- change: {change.name}")
    for required in ("proposal.md", "design.md", "tasks.md"):
        check((change / required).exists(), f"has {required}")
    if (change / "proposal.md").exists():
        proposal = (change / "proposal.md").read_text()
        for heading in PROPOSAL_HEADINGS:
            check(heading in proposal, f"proposal has {heading!r}")
    if (change / "tasks.md").exists():
        tasks = (change / "tasks.md").read_text()
        # Task ids are dotted, and a group inserted later may be numbered 3a,
        # so the group part is digits plus optional letters: 1.1, 3a.4, 12.10.
        boxes = re.findall(r"^- \[([ xX])\] (\d+[a-z]*\.\d+) (.+)$", tasks, re.M)
        check(bool(boxes), "tasks.md has numbered checkboxes")
        unverifiable = [num for _, num, text in boxes if not re.search(r"\bverif", text, re.I)]
        check(not unverifiable, f"every task states a verification (missing: {unverifiable})")
        ticked = sum(1 for state, _, _ in boxes if state.lower() == "x")
        print(f"      {ticked}/{len(boxes)} tasks ticked")
        if strict:
            check(ticked == len(boxes), "all tasks ticked (--strict)")
    skip_specs = "skip_specs: true" in (change / ".openspec.yaml").read_text() \
        if (change / ".openspec.yaml").exists() else False
    deltas = sorted((change / "specs").rglob("spec.md")) if (change / "specs").exists() else []
    if skip_specs:
        check(not deltas, "skip_specs is set, so the change carries no spec delta")
    else:
        check(bool(deltas), "has at least one spec delta under specs/<capability>/spec.md")
    for delta in deltas:
        check_delta(delta, delta.parent.relative_to(change / "specs"))


def main() -> int:
    strict = "--strict" in sys.argv[1:]
    changes_dir = ROOT / "openspec" / "changes"
    if not changes_dir.is_dir():
        print("FAIL  openspec/changes does not exist")
        return 1

    for change in sorted(changes_dir.iterdir()):
        if change.is_dir() and change.name != "archive":
            check_change(change, strict)

    specs_dir = ROOT / "openspec" / "specs"
    if specs_dir.is_dir():
        for spec in sorted(specs_dir.rglob("spec.md")):
            print(f"--- spec: {spec.parent.name}")
            check_spec(spec)
    else:
        print("FAIL  openspec/specs/ does not exist; README.md links to specs that are missing")
        failures.append("openspec/specs/ does not exist")

    print()
    if failures:
        print(f"RESULT: {len(failures)} problem(s)")
        return 1
    print("RESULT: all checks passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
