## ADDED Requirements

### Requirement: Derive Node Aliases From Each VM's Pool And Tags

For every qemu guest of a configured cluster the reconciler SHALL want these
SPIRE node aliases, with `<td>` the configured trust domain and `<cluster>` the
cluster's configured name:

- `spiffe://<td>/pve/<cluster>/vm/<vmid>` for every guest;
- `spiffe://<td>/pve/<cluster>/pool/<pool>` when the guest is in a pool;
- `spiffe://<td>/pve/<cluster>/pool/<pool>/tag/<key>/<value>` for each instance
  tag of a guest that is in a pool.

An instance tag SHALL be a Proxmox tag `vtpm_mds.io.<key>.<value>`, the prefix
matched without regard to case, `<key>` the characters up to the next `.` and
`<value>` the rest. Every such tag SHALL yield its own alias; two tags with one
key SHALL yield two aliases. Pool, key and value SHALL be lowercased. A pool,
key or value that is then not made only of `a-z`, `0-9`, `.`, `-` and `_`, or
that is `.` or `..`, SHALL yield no alias, and the reconciler SHALL log the VM
ID and the offending tag or pool once per pass. A pool whose ID contains `/`
(a nested pool) SHALL yield no pool or tag alias and SHALL be logged the same
way. When two distinct pools of one cluster lowercase to the same segment,
neither SHALL yield a pool or tag alias, and the reconciler SHALL log both
once per pass. A guest that is in no pool SHALL get only its `vm/` alias; its
tags SHALL grant nothing.

#### Scenario: A VM with a pool and tags

- **GIVEN** guest `350` of cluster `cluster-1` in pool `web` with the tags
  `vtpm_mds.io.role.Frontend`, `vtpm_mds.io.role.cache` and `other`
- **WHEN** the reconciler computes the wanted aliases
- **THEN** it wants `…/pve/cluster-1/vm/350`, `…/pve/cluster-1/pool/web`,
  `…/pve/cluster-1/pool/web/tag/role/frontend` and
  `…/pve/cluster-1/pool/web/tag/role/cache`, and nothing for `other`

#### Scenario: A VM without a pool

- **GIVEN** guest `351` in no pool with the tag `vtpm_mds.io.role.admin`
- **WHEN** the reconciler computes the wanted aliases
- **THEN** it wants only `…/pve/cluster-1/vm/351`

#### Scenario: A tag that cannot be a path segment

- **GIVEN** guest `352` in pool `web` with the tag `vtpm_mds.io.role.a+b`
- **WHEN** the reconciler computes the wanted aliases
- **THEN** it wants no `tag/role/…` alias for it and logs VM `352` and that tag

#### Scenario: Two pools that differ only in case

- **GIVEN** pools `Web` and `web` in `cluster-1`, each with a guest
- **WHEN** the reconciler computes the wanted aliases
- **THEN** it wants no `pool/web` alias and no tag alias under it, only the
  guests' `vm/` aliases, and logs both pools

#### Scenario: A nested pool

- **GIVEN** guest `354` in pool `web/front` with the tag `vtpm_mds.io.role.db`
- **WHEN** the reconciler computes the wanted aliases
- **THEN** it wants only `…/pve/cluster-1/vm/354` and logs the pool

### Requirement: Select Aliases Only By Released tpm_devid Selectors

Every alias for guest `<vmid>` of a cluster SHALL have
`spiffe://<td>/spire/server` as its parent and exactly two selectors,
`tpm_devid:subject:cn:<vmid>` and `tpm_devid:ca:fingerprint:<sha1>`, with
`<sha1>` the lowercase hex SHA-1 of the DER of a configured intermediate of
that cluster. With several intermediates configured for a cluster, each wanted
alias SHALL exist once per intermediate. Every other field of the entry SHALL
be at its default.

#### Scenario: Two intermediates during a rotation

- **GIVEN** cluster `cluster-1` configured with intermediates A and B
- **WHEN** guest `350` is wanted under `…/pve/cluster-1/vm/350`
- **THEN** two entries exist under that ID, one selecting A's fingerprint and
  one selecting B's, both with `subject:cn:350`

### Requirement: Refuse A Configuration Whose Fingerprints Span Clusters

Loading the configuration SHALL fail, naming the file and the cluster, when an
intermediate certificate is self-signed, when one fingerprint is configured
for more than one cluster, when two clusters share a name, or when the trust
domain or a cluster name is not a valid SPIFFE path component.

#### Scenario: A fingerprint shared by two clusters

- **GIVEN** intermediate A configured for `cluster-1` and for `cluster-2`
- **WHEN** the reconciler starts
- **THEN** it exits non-zero, naming A and both clusters

#### Scenario: A root configured as an intermediate

- **GIVEN** a self-signed certificate configured as an intermediate of
  `cluster-1`
- **WHEN** the reconciler starts
- **THEN** it exits non-zero, naming that file

### Requirement: Own Only Node Aliases Under The Configured Prefixes

The reconciler SHALL create and delete only entries whose parent is
`spiffe://<td>/spire/server` and whose SPIFFE ID starts with
`spiffe://<td>/pve/<cluster>/` for a configured `<cluster>`. Among those it
SHALL keep an entry only when its SPIFFE ID and selector set equal a wanted
alias and its other fields are at their defaults; it SHALL delete every other
such entry, including entries it did not create, and create the wanted aliases
that are missing. It SHALL NOT change any other entry.

#### Scenario: A removed tag

- **GIVEN** the alias `…/pve/cluster-1/pool/web/tag/role/frontend` for guest `350`
- **WHEN** the tag is removed in Proxmox and the next good pass reads the cluster
- **THEN** that alias is deleted, and `…/vm/350` and `…/pool/web` stay

#### Scenario: An entry outside the prefix

- **GIVEN** a hand-written node alias `spiffe://<td>/nodes/build` selecting
  `subject:cn:350`
- **WHEN** a pass runs
- **THEN** that entry is unchanged

#### Scenario: A workload entry under the prefix

- **GIVEN** a workload entry `…/pve/cluster-1/pool/web/app` whose parent is
  `…/pve/cluster-1/pool/web`
- **WHEN** a pass runs
- **THEN** that entry is unchanged

#### Scenario: A hand-edited alias

- **GIVEN** the wanted alias `…/pve/cluster-1/vm/350` edited by hand to
  `admin: true`
- **WHEN** a pass runs
- **THEN** that entry is deleted and the alias is created again with defaults

### Requirement: Never Act On A Doubtful Read

A pass SHALL change a cluster's aliases, and SHALL count a cluster's read
toward a ban, only after a good read of that cluster. A read SHALL be doubtful,
and SHALL leave that cluster unchanged for the pass with one log line, when a
request fails, on a TLS or authentication error, when it lists no qemu guest,
or when the privilege check fails. Before every pass the reconciler SHALL read
the token's permissions and require `VM.Audit` on `/vms` and `Pool.Audit` on
`/pool`, each with the propagate flag set. When that check fails at start the
reconciler SHALL exit non-zero, naming the cluster and the missing privilege.

#### Scenario: The API is unreachable

- **GIVEN** aliases for guests of `cluster-1`
- **WHEN** a pass cannot reach that cluster's API
- **THEN** no alias of `cluster-1` is deleted, the pass does not count toward a
  ban, and one line is logged

#### Scenario: An empty listing

- **GIVEN** cluster `cluster-1`, with or without aliases
- **WHEN** a read succeeds but lists no qemu guest
- **THEN** the cluster is left unchanged, the pass does not count toward a ban,
  and one line is logged

#### Scenario: A token without rights at start

- **GIVEN** a token for `cluster-1` without `Pool.Audit` on `/pool`
- **WHEN** the reconciler starts
- **THEN** it exits non-zero, naming `cluster-1` and `Pool.Audit`

#### Scenario: Rights withdrawn during a run

- **GIVEN** a running reconciler whose token for `cluster-1` loses the
  propagate flag of `VM.Audit` on `/vms`
- **WHEN** the next pass runs
- **THEN** `cluster-1` is left unchanged for that pass and one line is logged

### Requirement: Ban Agents Of Destroyed VMs

A `tpm_devid` agent whose selectors include `ca:fingerprint` of a configured
intermediate of a cluster, and whose `subject:cn` names no qemu guest of that
cluster in two consecutive good reads, SHALL be banned, and one line naming
the agent ID and the VM ID SHALL be logged. An agent whose VM is listed SHALL
NOT be banned or evicted by the reconciler. The count of consecutive reads
SHALL be held in memory only, so a restart starts it again, and a single pass
run with `-once` SHALL NOT ban.

#### Scenario: A destroyed VM

- **GIVEN** an attested agent with `subject:cn:353` and cluster-1's
  intermediate, and guest `353` destroyed
- **WHEN** two consecutive good passes read cluster-1 without guest `353`
- **THEN** the agent is banned after the second, and one line is logged

#### Scenario: A VM missing from one listing only

- **GIVEN** guest `350` missing from one good read and listed in the next
- **WHEN** both passes run
- **THEN** its agent is not banned

#### Scenario: A single pass

- **GIVEN** guest `353` destroyed and its agent attested
- **WHEN** the reconciler runs with `-once`
- **THEN** no agent is banned

### Requirement: Explain Why A Tag Needs A Pool

The reconciler's manual page SHALL derive the scope rule: who can set a
Proxmox tag (`VM.Config.Options` on the guest, narrowed by the datacenter
options `user-tag-access` and `registered-tags`), that a tag alone is therefore
worth only as much as the VM's owner, why the pool is the scope, which IDs a VM
receives, and what an owner must do for a tag to take effect. It SHALL give a
worked example for a VM with a pool and tags, a VM without a pool, a tag that
cannot be a path segment, a removed tag, a destroyed VM, a nested pool and two
pools that differ only in case, using neutral names only. It SHALL state that
node aliases written by hand under the reconciler's prefix are deleted, that a
guest hidden by `NoAccess` is read as destroyed, that renaming a cluster in the
configuration leaves the old prefix's aliases to be deleted by hand, and that a
VM restored after its agent was banned must enroll again.

#### Scenario: An operator looks up why a tag grants nothing

- **GIVEN** a VM in no pool with the tag `vtpm_mds.io.role.admin`
- **WHEN** the operator reads the manual page
- **THEN** the example for a VM without a pool explains that the tag grants
  nothing and that adding the VM to a pool makes it effective
