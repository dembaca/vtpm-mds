## ADDED Requirements

### Requirement: Derive Node Aliases From Each VM's Pool And Tags

For every qemu guest of a configured cluster the reconciler SHALL want these
SPIRE node aliases, with `<td>` the configured trust domain and `<cluster>` the
cluster's configured name:

- `spiffe://<td>/pve/<cluster>/vm/<vmid>` for every guest;
- `spiffe://<td>/pve/<cluster>/pool/<pool>` when the guest is in a pool;
- `spiffe://<td>/pve/<cluster>/pool/<pool>/tag/<key>/<value>` for each instance
  tag of a guest that is in a pool.

Instance tags SHALL be the guest's Proxmox tags of the form
`vtpm_mds.io.<key>.<value>`, parsed as for instance metadata. Pool, key and
value SHALL be lowercased. A pool, key or value that is then not made only of
`a-z`, `0-9`, `.`, `-` and `_` SHALL yield no alias, and the reconciler SHALL
log the VM ID and the tag once per pass. A guest that is in no pool SHALL get
only its `vm/` alias; its tags SHALL grant nothing.

#### Scenario: A VM with a pool and tags

- **GIVEN** guest `350` of cluster `cluster-1` in pool `web` with the tags
  `vtpm_mds.io.role.Frontend` and `other`
- **WHEN** the reconciler computes the wanted aliases
- **THEN** it wants `…/pve/cluster-1/vm/350`, `…/pve/cluster-1/pool/web` and
  `…/pve/cluster-1/pool/web/tag/role/frontend`, and nothing for `other`

#### Scenario: A VM without a pool

- **GIVEN** guest `351` in no pool with the tag `vtpm_mds.io.role.admin`
- **WHEN** the reconciler computes the wanted aliases
- **THEN** it wants only `…/pve/cluster-1/vm/351`

#### Scenario: A tag that cannot be a path segment

- **GIVEN** guest `352` in pool `web` with the tag `vtpm_mds.io.role.a+b`
- **WHEN** the reconciler computes the wanted aliases
- **THEN** it wants no `tag/role/…` alias for it and logs VM `352` and that tag

### Requirement: Select Aliases Only By Released tpm_devid Selectors

Every alias for guest `<vmid>` of a cluster SHALL have
`spiffe://<td>/spire/server` as its parent and exactly two selectors,
`tpm_devid:subject:cn:<vmid>` and `tpm_devid:ca:fingerprint:<sha1>`, with
`<sha1>` the lowercase hex SHA-1 of the DER of a configured intermediate of
that cluster. With several intermediates configured for a cluster, each wanted
alias SHALL exist once per intermediate.

#### Scenario: Two intermediates during a rotation

- **GIVEN** cluster `cluster-1` configured with intermediates A and B
- **WHEN** guest `350` is wanted under `…/pve/cluster-1/vm/350`
- **THEN** two entries exist under that ID, one selecting A's fingerprint and
  one selecting B's, both with `subject:cn:350`

### Requirement: Own Only The Configured Prefixes

The reconciler SHALL create and delete node aliases only under
`spiffe://<td>/pve/<cluster>/` for a configured `<cluster>`. Under that prefix
it SHALL delete every node alias that is not wanted, including entries it did
not create. It SHALL NOT change any other entry.

#### Scenario: A removed tag

- **GIVEN** the alias `…/pve/cluster-1/pool/web/tag/role/frontend` for guest `350`
- **WHEN** the tag is removed in Proxmox and the next pass reads the cluster
- **THEN** that alias is deleted, and `…/vm/350` and `…/pool/web` stay

#### Scenario: An entry outside the prefix

- **GIVEN** a hand-written node alias `spiffe://<td>/nodes/build` selecting
  `subject:cn:350`
- **WHEN** a pass runs
- **THEN** that entry is unchanged

### Requirement: Never Apply A Doubtful Read

A pass SHALL change a cluster's aliases or ban its agents only after a complete,
successful read of that cluster. A failed request, a TLS or authentication
error, or a read listing no qemu guest while aliases exist under that cluster's
prefix SHALL leave that cluster unchanged for the pass and SHALL be logged once.
At start the reconciler SHALL check that each cluster's token holds `VM.Audit`
on `/vms` and `Pool.Audit` on `/pool`, and SHALL refuse to start, naming the
cluster and the missing privilege, when it does not.

#### Scenario: The API is unreachable

- **GIVEN** aliases for guests of `cluster-1`
- **WHEN** a pass cannot reach that cluster's API
- **THEN** no alias of `cluster-1` is deleted and no agent is banned, and one
  line is logged

#### Scenario: An empty listing

- **GIVEN** aliases for guests of `cluster-1`
- **WHEN** a read succeeds but lists no qemu guest
- **THEN** the cluster is left unchanged for that pass and one line is logged

#### Scenario: A token without rights

- **GIVEN** a token for `cluster-1` without `Pool.Audit` on `/pool`
- **WHEN** the reconciler starts
- **THEN** it exits non-zero, naming `cluster-1` and `Pool.Audit`

### Requirement: Ban Agents Of Destroyed VMs

A `tpm_devid` agent whose selectors include `ca:fingerprint` of a configured
intermediate of a cluster, and whose `subject:cn` names no qemu guest of that
cluster in two consecutive successful reads, SHALL be banned, and one line
naming the agent ID and the VM ID SHALL be logged. An agent whose VM is listed
SHALL NOT be banned or evicted by the reconciler.

#### Scenario: A destroyed VM

- **GIVEN** an attested agent with `subject:cn:353` and cluster-1's
  intermediate, and guest `353` destroyed
- **WHEN** two consecutive passes read cluster-1 without guest `353`
- **THEN** the agent is banned after the second, and one line is logged

#### Scenario: A VM missing from one listing only

- **GIVEN** guest `350` missing from one read and listed in the next
- **WHEN** both passes run
- **THEN** its agent is not banned

### Requirement: Explain Why A Tag Needs A Pool

The reconciler's manual page SHALL derive the scope rule: who can set a
Proxmox tag, that a tag alone is therefore worth only as much as the VM's
owner, why the pool is the scope, which IDs a VM receives, and what an owner
must do for a tag to take effect. It SHALL give a worked example for a VM with
a pool and tags, a VM without a pool, a tag that cannot be a path segment, a
removed tag and a destroyed VM, using neutral names only. It SHALL state that
entries written by hand under the reconciler's prefix are deleted, and that a
VM restored after its agent was banned must enroll again.

#### Scenario: An operator looks up why a tag grants nothing

- **GIVEN** a VM in no pool with the tag `vtpm_mds.io.role.admin`
- **WHEN** the operator reads the manual page
- **THEN** the example for a VM without a pool explains that the tag grants
  nothing and that adding the VM to a pool makes it effective
