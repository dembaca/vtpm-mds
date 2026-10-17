# Design

## Context

`proposal.md` explains why the Proxmox half of
`serve-a-public-address-or-omit-the-key` needs correcting. This document
records the evidence, so nobody has to derive it again, and sets out the four
options with their costs.

### Evidence, with sources

Read on 2026-10-17. Line numbers drift between versions, so each source names
the copy it came from.

- **`check_type` rejects unknown keys.** `PVE/QemuServer.pm`, `sub check_type`:
  `die "unknown setting '$key'\n" if !$schema->{$key};`. That is line 1804 in
  `proxmox/qemu-server` at `4127436e5a7e` (`src/PVE/QemuServer.pm`, upstream),
  and line 1769 in `/usr/share/perl5/PVE/QemuServer.pm` installed on hogan.
- **Parse errors warn outside strict mode, and the key is dropped.**
  `sub parse_vm_config`: `$handle_error` is `if ($strict) { die $msg } else
  { warn $msg }` (upstream 1951–1959, hogan 1912). For a `key: value` line,
  `$conf->{$key} = $value` runs only in the `else` after
  `eval { check_type(…) }`. On error the message is
  `vm $vmid - unable to parse value of '$key' - $@` (upstream 2053).
- **Writes serialise only parsed keys.** `sub write_vm_config`,
  `$generate_raw_config` (upstream 2153–2172): it writes the description, then
  `"$key: $conf->{$key}\n"` for each key of the parsed hash. An unparsed line
  does not survive.
- **The description is stored as URL-encoded `#` lines.** In the same closure:
  `$raw .= '#' . PVE::Tools::encode_text($cl) . "\n"` for each line of the
  description (upstream 2157–2161; hogan ~2118). It is read back by
  `if ($line =~ m/^\#(.*)$/) { $descr .= PVE::Tools::decode_text($1) . "\n" }`
  (upstream 2021–2024). The form `description: …` is accepted on read
  (upstream 2027) but never written.
- **What `encode_text` escapes.** `PVE::ParseUtils::encode_text` in
  `proxmox/pve-common` at `38395cd28456`: "all control and hi-bit characters,
  ':' and '%'", via `uri_escape`. A Notes line `public-ipv4: 203.0.113.7` is
  therefore stored on disk as `#public-ipv4%3A 203.0.113.7`.
- **Tag syntax.** `PVE::JSONSchema`, `$PVE_TAG_RE =
  qr/[a-z0-9_][a-z0-9_\-\+\.]*/i`. A tag cannot contain `:` or `=`.
- **Our parser.** `internal/proxmox/vmconfig.go`, the line loop: it skips
  blank lines and lines starting with `#`, and stores every other `key: value`
  or `key=value` line in `RawConfig`. That matches `vm-inventory`'s
  `Derive VM Records From Proxmox Guest Configuration` word for word. It is why
  a hand-written `public-ipv4:` line reaches `RawConfig["public-ipv4"]` while
  Proxmox discards it.

### A separate finding, not in this change's scope

The same parser has **no notion of sections**. A Proxmox guest config can carry
`[PENDING]`, `[special:…]` and one `[<snapshot>]` section per snapshot, each a
full set of `key: value` lines. `internal/proxmox` reads all of them into one
record:

- A later section's value **overwrites** the current one in `RawConfig`. An
  old `name` from a snapshot, for example, would replace the current name.
- Every `netN: virtio=…` line in every section **adds** its MAC to the record.

The second matters most: a NIC that was removed from the VM but survives in a
snapshot still binds callers to that VM. This is recorded here because options
1 and 2 read data that also appears per snapshot. Under `AGENTS.md` scope
discipline it gets its own change, and it has been handed to the coordinator.

## Goals

- An operator on a Proxmox host can declare a VM's public IPv4 in a place that
  survives every Proxmox write, or the record says plainly that this is not
  supported.
- Nothing is written into a Proxmox guest config that Proxmox would not write
  itself. This is the maintainer's constraint of 2026-10-17.
- A hand-written `public-ipv4:` line in a guest config stops being honoured.
- `GET /latest/meta-data/public-ipv4` keeps its contract: the value comes from
  the bound VM record and is never derived from the connection.

## Non-Goals

- Changing the YAML half of #29.
- Fixing the section-blind parser described above. That is a separate change.
- Deriving a public address from anything the service observes. That was
  rejected in `serve-a-public-address-or-omit-the-key` (options B and C) and
  stays rejected.

## Options

Each option is assessed against the same five questions:
- **Native:** does Proxmox write and keep the data?
- **Visible:** can an operator see and edit it in the Proxmox UI?
- **Validation:** where is a bad value caught?
- **Spec cost:** what has to change in the specs?
- **Code cost:** what has to change in the code?

### 1. Proxmox Notes (`description`)

The operator puts a line such as `public-ipv4: 203.0.113.7` in the VM's Notes.
Proxmox stores it as `#public-ipv4%3A 203.0.113.7`.

- **Native:** yes, and it survives every Proxmox write.
- **Visible:** yes, the Notes field in the web UI and `qm config` as
  `description`.
- **Validation:** at load time. Notes are free text, so failing the whole
  load over them would deny metadata to every VM on the host. A malformed
  declaration would instead be logged and treated as absent.
- **Spec cost:** `Derive VM Records From Proxmox Guest Configuration` currently
  says `#` lines "SHALL be ignored". That changes to: decode the leading `#`
  block of the **main section only** with Proxmox's encoding, and take exactly
  one marker line from it. Rules are needed for duplicates and for a marker
  inside a Markdown code block.
- **Code cost:** moderate. URL-decoding as `uri_unescape` does, marker parsing,
  section awareness for at least the leading block, plus tests with a real,
  encoded fixture file.
- **Risk:** free text is shared with humans. An operator could write the marker
  in prose by accident, or change it while editing Notes for another reason.

### 2. Proxmox tags (`tags:`)

The operator adds a tag such as `public-ipv4.203.0.113.7`.

- **Native:** yes.
- **Visible:** yes, as tags in the UI.
- **Validation:** at load time. Shape and parsing are entirely our convention.
- **Spec cost:** a new clause reading `tags:` and its separator (`;`).
- **Code cost:** small.
- **Risk:** it abuses a label as a key/value store. Tags are cluster-wide
  vocabulary and may be restricted by datacenter tag settings — **not checked**;
  verify before choosing this. Changing an address means renaming a tag. Of the
  four, this is the one the maintainer and the coordinator both called ugly.

### 3. A file this service owns

For example `/etc/vtpm-mds/public-addresses.yaml`, mapping VM ID to address,
read alongside the Proxmox backend. Or, more generally, `inventory_path`
allowed as an overlay to the Proxmox backend.

- **Native:** no, by design. It is our file, not Proxmox's, so the
  maintainer's constraint is met by not touching `/etc/pve` at all.
- **Visible:** no, not in the Proxmox UI.
- **Validation:** loudly at load time. We own the file, so the same rule as the
  YAML inventory applies: a bad value stops the service from starting.
- **Spec cost:** a new requirement (path, format, merge with Proxmox records,
  what an unknown VM ID means) and a configuration key.
- **Code cost:** moderate. A loader, a merge, a config key, a packaged example
  file and the man page.
- **Risk:** drift. The address lives away from the VM, so a reused or deleted
  VM ID keeps an old declaration until someone edits the file.

### 4. No public address from the Proxmox backend

Only the YAML inventory declares one. On a Proxmox host `public-ipv4` is
simply never served, and the index never lists it — honestly, as on an EC2
instance without a public address.

- **Native / visible:** not applicable.
- **Validation:** none needed.
- **Spec cost:** small. The `Load VM Records From The YAML Inventory` clause
  that names the Proxmox backend as a source goes, and `Derive VM Records …`
  gains "SHALL NOT populate `public-ipv4`", the same way it already says it
  does not populate the EK pin.
- **Code cost:** small. The Proxmox parser must not let a `public-ipv4:` line
  reach that key.
- **Risk:** the feature is unavailable on the hosts this project runs on
  today, hogan included, until one of 1–3 follows.

## Recommendation

**The decision is the maintainer's.** For what it is worth, both agents who
looked at it lean the same way.

- **Option 1 (Notes)** is the one that meets "on par with the Proxmox
  inventory", which the maintainer asked for on 2026-10-17, *and* "only what
  Proxmox would put there". The operator declares the address in Proxmox's own
  UI, and Proxmox keeps it. Its price is the parser work above and a rule for
  free text. The coordinator first recommended it as cheaper than it is,
  because it read `description:` from `qm config` output rather than from the
  file format, and has said so.
- **Option 4** costs least and is the right first step under any choice,
  because it stops honouring the fragile line. It could ship on its own while
  1 or 3 is designed.

So the suggested shape is **4 now, then 1**, unless the maintainer prefers
not to expose this in Proxmox at all, in which case **4 alone**, or **4 then
3**.

## Open questions for the maintainer

1. Which option, or which combination (for example 4 now, 1 later)?
2. For option 1: what marker syntax — `public-ipv4: <address>` as one line of
   Notes, or something less likely to be written in prose?
3. For option 1: is a malformed declaration in Notes logged and treated as
   absent (the recommendation, since Notes are free text), or should it stop
   the service like the YAML inventory?
