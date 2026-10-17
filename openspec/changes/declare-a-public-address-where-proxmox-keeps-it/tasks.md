# Tasks

Nothing is ticked: this is a decision record, written on 2026-10-17 against
`main` at `2dd0335` by `agent-mac-vtpm-mds-52ae2f`. No code has been touched.

**Ordering.** Apply after `serve-a-public-address-or-omit-the-key` is archived.
That change's `vm-inventory` delta introduces the text this change modifies.

**Until section 1 is answered,** do not write a `public-ipv4:` line into any
Proxmox guest config, including VM 399's. It would be a measurement of the
mechanism this change replaces.

## 1. Decide

- [ ] 1.1 **Maintainer** — choose an option or a combination from `design.md`, and answer its three open questions; verify the answer is recorded in `design.md` under a "Decided" heading with the date
- [ ] 1.2 Write the spec delta for the chosen option and remove `skip_specs: true` from `.openspec.yaml`, then verify `openspec validate declare-a-public-address-where-proxmox-keeps-it --strict` passes
      The delta modifies `Derive VM Records From Proxmox Guest Configuration`,
      and the clause in `Load VM Records From The YAML Inventory` that names
      the Proxmox backend as a source, in the form archiving
      `serve-a-public-address-or-omit-the-key` leaves them. A `MODIFIED` block
      may not drop a scenario the living spec still has; where one must go, use
      REMOVED + ADDED.
- [ ] 1.3 Replace this section's follow-on with implementation and verification tasks for the chosen option, each naming its own verification, and verify `scripts/openspec-validate.py --strict` recognises every new task line
      Number the tasks `N.N`. A suffix such as `5.2a` is not recognised as a
      task by the fallback validator, so it is never reported as unticked.

## 2. Whatever is chosen

- [ ] 2.1 Make a `public-ipv4:` line in a Proxmox guest config no longer reach `RawConfig["public-ipv4"]`, and verify with a unit test that a guest config carrying `public-ipv4: 203.0.113.7` yields a record without that key
      This is option 4's code. It is needed under every option, because the
      line Proxmox discards must not keep working until it silently stops.
- [ ] 2.2 Verify on hogan, against an installed git-stamped package, that VM 399 without any declaration still answers `public-ipv4` with `404` and an empty body, and lists five index lines
      The negative half needs no write to `/etc/pve`, and stays true under all
      four options.
