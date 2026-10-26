# Design

## Context

On 2026-10-26 (`main@e54344d`), a grep for the lab's names found them in about
twelve test files, two specs, one man page, the README and three scripts'
comments. No code reads any of them: the Proxmox backend takes node and
cluster names from `/etc/pve`, VM IDs from the guest configs and topology from
the node description. They are fixture values and prose.

## Decisions

### D-a: One neutral vocabulary

| Site name | Neutral name |
|---|---|
| node `hogan` / `holger` (and `hogan-a`, `Hogan`) | `node-a` / `node-b` (`node-x`, `Node-A`) |
| cluster `bgl-cluster`, `bgl` (as cluster name), `BGL-Cluster` | `cluster-1`, `Cluster-1` |
| `bgl` as a region value | `region-1` |
| region `bgl-1`, `bgl-x`, `DUS` | `region-1`, `region-x`, `region-a` |
| zone `DUS4`, `DUS3`, `keller` | `AZ4`, `AZ3`, `room-a` |
| rack `keller01` | `rack-01` |
| VM `399` (`vtpm-pilot`, `pilot`) | `350` (`<guest>`, `guest-p`) |
| MACs `bc:24:11:75:da:c0`, `bc:24:11:06:1d:b2`, `BC:24:11:06:1D:B3` | `bc:24:11:00:01:99`, `bc:24:11:00:02:99`, `BC:24:11:00:02:9A` |
| `CN=BGL Proxmox DevID CA` | `CN=Example DevID CA` |

The table is fixed instead of renaming case by case, so a reviewer can check
the diff mechanically and later tests take their names from the same list.
Some tests depend on a name's case (`BGL-Cluster`, upper-case `DUS4`) or its
length; the neutral name keeps that property. `bc:24:11` is Proxmox's own OUI
and stays. Only the host part changes, because it identified one real NIC.

### D-b: Spec scenarios are restated, not reworded

Four requirements in `vm-inventory` and `instance-metadata` use site names in
their scenarios. Each is MODIFIED by applying D-a and nothing else, so the
delta can be reviewed as a rename. Requirement text outside the scenarios is
copied verbatim.

### D-c: What counts as the product

The product is:
- code and tests
- `openspec/specs/`
- man pages, and `debian/` files other than the changelog
- shipped configuration
- `README.md` and `ARCHITECTURE.md`
- the scripts

Archived changes, the changelog and verification notes in `tasks.md`
record this project's own history. They may name the host and VM a check ran
on, because that is what they record.

### D-d: AGENTS.md holds rules, not history or lab facts

The maintainer, 2026-10-26: history is in git, and lab facts belong in the
operator's private infrastructure repository. `AGENTS.md` keeps the rules
(OpenSpec workflow, scope, commit discipline, verifying on real artifacts,
review, coordination) and the pitfalls that still hold, in neutral words. The
queue, the archive chronicle and the lab host facts are removed; the removed
text was handed to the operator as a file and stays in git history.

## Risks

- A future test copies a lab name back in. Under the requirement that is a
  review finding. There is no automated check, because a word list in CI would
  itself carry the site's names.
