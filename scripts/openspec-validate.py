#!/usr/bin/env python3
"""Structural check for openspec/ when the `openspec` CLI is unavailable.

The upstream CLI (`openspec validate --all --strict`) is an npm package; lab
hosts here have no node/npm, so this covers the structure the CLI enforces:
required documents, task checkboxes with a verification clause, and spec
deltas whose requirements each own at least one scenario.

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

failures: list[str] = []


def check(cond: bool, msg: str) -> None:
    print(f"{'PASS' if cond else 'FAIL'}  {msg}")
    if not cond:
        failures.append(msg)


def check_delta(path: Path) -> None:
    capability = path.parent.name
    text = path.read_text()
    check(bool(DELTA_SECTION.search(text)), f"{capability}: has an ADDED/MODIFIED/REMOVED section")
    check("SHALL" in text, f"{capability}: uses normative SHALL")
    blocks = re.split(r"^### Requirement: ", text, flags=re.M)[1:]
    check(bool(blocks), f"{capability}: has '### Requirement:' headers")
    for block in blocks:
        name = block.splitlines()[0]
        scenarios = re.split(r"^#### Scenario: ", block, flags=re.M)[1:]
        check(bool(scenarios), f"{capability}/{name}: has at least one scenario")
        for scenario in scenarios:
            check(
                bool(re.search(r"\*\*(GIVEN|WHEN|THEN)\*\*", scenario)),
                f"{capability}/{name}: scenario uses GIVEN/WHEN/THEN",
            )


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
        boxes = re.findall(r"^- \[([ x])\] (\d+\.\d+) (.+)$", tasks, re.M)
        check(bool(boxes), "tasks.md has numbered checkboxes")
        unverifiable = [num for _, num, text in boxes if not re.search(r"\bverif", text, re.I)]
        check(not unverifiable, f"every task states a verification (missing: {unverifiable})")
        ticked = sum(1 for state, _, _ in boxes if state == "x")
        print(f"      {ticked}/{len(boxes)} tasks ticked")
        if strict:
            check(ticked == len(boxes), "all tasks ticked (--strict)")
    deltas = sorted((change / "specs").rglob("spec.md")) if (change / "specs").exists() else []
    check(bool(deltas), "has at least one spec delta under specs/<capability>/spec.md")
    for delta in deltas:
        check_delta(delta)


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
            check_delta(spec)
    else:
        print("--- spec: openspec/specs/ is missing")
        print("NOTE  no capability specs yet; README.md links to specs that do not exist")

    print()
    if failures:
        print(f"RESULT: {len(failures)} problem(s)")
        return 1
    print("RESULT: all checks passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
