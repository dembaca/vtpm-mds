# Tasks

Nothing is ticked. This is an exploration record, written on 2026-10-17
against `main` at `2dd0335` by `agent-mac-vtpm-mds-52ae2f`. It began as
`declare-a-public-address-where-proxmox-keeps-it` and was widened the same day
at the maintainer's prompt. No code has been touched.

**Ordering.**
- Apply after `serve-a-public-address-or-omit-the-key` is archived.
- Derived topology defaults (D4) also wait for the coordinator's
  `internal/proxmox` change (node selection and section handling).

**Until 1.1 is answered,** do not write a `public-ipv4:` line, or any other
hand-written key, into a Proxmox guest config, including VM 399's.

## 1. Decide

- [ ] 1.1 **Maintainer** — answer D1 to D5, D7, D8 and D9 in `design.md`, and verify the answers are recorded there under a dated "Decided" heading
- [ ] 1.2 Check D6 against current Kubernetes documentation — how a node obtains `topology.kubernetes.io/region` and `.../zone` without an in-tree cloud provider — and verify the finding is recorded in `design.md` with the URLs read
      Read current documentation, not memory, as `AGENTS.md` requires for
      Teleport. If a consumer needs anything beyond `placement/region` and
      `placement/availability-zone`, the scope changes before any delta is
      written.
- [x] 1.3 Check on hogan that `/etc/pve/local` points at the node the service runs on, and verify the result is recorded in `design.md`
      D4 depends on it, and the coordinator's node-selection change may use
      it.
      **Done**, checked by `agent-vtpm-hogan-ead994b0b` on hogan on 2026-10-17:
      `/etc/pve/local -> nodes/hogan` (symlink, `root:www-data`),
      `readlink -f` → `/etc/pve/nodes/hogan`, `hostname -s` → `hogan`, and
      `/etc/pve/local/qemu-server` exists. Proxmox maintains the link itself.
      The node name D4 needs is therefore the link target, not a guess.
- [ ] 1.4 Write the spec deltas for the decision and remove `skip_specs: true` from `.openspec.yaml`, then verify `openspec validate declare-per-vm-metadata --strict` passes
      Changing the pinned constants `proxmox` and `local` modifies
      `instance-metadata` requirements that have scenarios. A scenario whose
      outcome inverts cannot be dropped in a `MODIFIED` block; use REMOVED +
      ADDED.
- [ ] 1.5 Replace this section's follow-on with implementation and verification tasks for the decision, numbered `N.N`, and verify `scripts/openspec-validate.py --strict` counts every new task
      A suffix such as `5.2a` is not recognised as a task by the fallback
      validator, so it is never reported as unticked.

- [x] 1.6 Read what SPIRE's `tpm_devid` attestor takes from the certificate and how a readable SPIFFE ID arises, and verify the finding is recorded in `design.md` with sources and the release it holds for
      **Done** on 2026-10-17 by `agent-mac-vtpm-mds-0172be`, from `spiffe/spire`
      at the commits named in `design.md`. The agent ID is fixed
      (`…/spire/agent/tpm_devid/<sha1 of the certificate>`); a readable identity
      is a node alias; `subject:oid` selectors exist only on `main`, not in
      `v1.15.4`. Not checked: the SPIRE version actually deployed, and the
      attestor's source code.

## 2. Under every decision

- [ ] 2.1 Make a `public-ipv4:` line in a Proxmox guest config no longer reach `RawConfig["public-ipv4"]`, and verify with a unit test that such a config yields a record without the key
      This removes the one behaviour in `main` that works only until Proxmox
      next writes the file. It may ship ahead of the rest as its own small
      change, if the maintainer prefers.
- [ ] 2.2 Verify on hogan, against an installed git-stamped package, that VM 399 without any declaration answers `public-ipv4` with `404` and an empty body, and has a five-line index
