## ADDED Requirements

### Requirement: Read Cluster EK Pins From pmxcfs

The service SHALL read cluster EK pins from the directory
`mds.cluster_pin_dir` (default `/etc/pve/vtpm-ek`; empty disables the
cluster source). A cluster pin is the file `<cluster_pin_dir>/<vmid>` whose
name is a VM ID (digits only) and whose content is exactly 64 characters
`[0-9a-f]`, optionally followed by one `\n`: the lowercase hex SHA-256 of the
EK certificate DER, the value logged as `ek=` in `devid issued`. Any other
file in that directory, an empty one included, SHALL NOT count as a pin; when
it is named after the caller's own VM ID the leg SHALL log one line
`devid enroll: invalid cluster pin vm=<id>`.

The service SHALL NOT create, write, rename or delete anything under
`cluster_pin_dir`. A missing directory SHALL count as an empty store. A
directory that exists but cannot be listed, or a valid-looking pin file that
cannot be read, SHALL make the leg answer `500` with body
`pin store unavailable` and issue nothing.

`mds.require_cluster_pin` (boolean, default `false`) SHALL, when `true`, make
both enroll legs answer `401` with body `no cluster pin for this VM` for a
caller whose VM has neither a YAML inventory `ek_sha256` nor a valid cluster
pin, before any challenge, and SHALL prevent the creation of a local pin.
`require_cluster_pin: true` with an empty `cluster_pin_dir` SHALL fail the
configuration load with an error naming both keys.

When the caller has no YAML inventory `ek_sha256` and no valid cluster pin
and `require_cluster_pin` is `false`, each leg SHALL log one line
`devid enroll: no cluster pin vm=<id>, using local pin`. No log line of this
requirement SHALL contain a fingerprint.

#### Scenario: A valid cluster pin is read

- **GIVEN** `/etc/pve/vtpm-ek/100` containing 64 lowercase hex characters and
  a newline
- **WHEN** VM `100` calls either enroll leg
- **THEN** that value is VM `100`'s cluster pin

#### Scenario: A malformed cluster pin is no pin

- **GIVEN** `/etc/pve/vtpm-ek/100` containing 64 upper-case hex characters,
  or a second line, or nothing
- **WHEN** VM `100` calls either enroll leg with `require_cluster_pin: false`
- **THEN** the leg logs `devid enroll: invalid cluster pin vm=100` and
  proceeds as for a VM without a cluster pin

#### Scenario: Required cluster pin refuses a VM without one

- **GIVEN** `require_cluster_pin: true`, no YAML inventory `ek_sha256` for VM
  `100` and no valid `/etc/pve/vtpm-ek/100`
- **WHEN** VM `100` calls either enroll leg
- **THEN** the response is `401` with body `no cluster pin for this VM`, no
  challenge or LDevID is issued and no local pin file for `100` exists

#### Scenario: Missing directory is an empty store

- **GIVEN** no directory `/etc/pve/vtpm-ek` and `require_cluster_pin: false`
- **WHEN** VM `100` enrolls for the first time
- **THEN** it enrolls with a local pin as without this requirement and the
  leg logs `devid enroll: no cluster pin vm=100, using local pin`

#### Scenario: Unlistable cluster directory fails closed

- **GIVEN** `/etc/pve/vtpm-ek` exists and the service cannot list it
- **WHEN** either enroll leg is called
- **THEN** the response is `500` with body `pin store unavailable` and no
  LDevID is issued

#### Scenario: Requiring a pin without a directory is a configuration error

- **GIVEN** a configuration with `require_cluster_pin: true` and
  `cluster_pin_dir: ""`
- **WHEN** the configuration is loaded
- **THEN** it exits non-zero with an error naming `require_cluster_pin` and
  `cluster_pin_dir`

## MODIFIED Requirements

### Requirement: Persist An EK Pin Per VM After First Successful Enrollment

On `enroll/finish`, when the caller's VMID has **no** inventory `ek_sha256`,
**no** valid cluster pin (see "Read Cluster EK Pins From pmxcfs") and **no**
pin file yet, and `mds.require_cluster_pin` is `false`, the service SHALL
create `/var/lib/<package>/ek-pins/<vmid>` (one line: lowercase hex SHA-256 of
that enrollment's EK certificate DER) with an exclusive create (`O_EXCL` or
equivalent) **before** signing the LDevID. The `ek-pins` directory SHALL be
created if missing. Two concurrent first finishes SHALL leave a single pin.
When the caller has a valid cluster pin, finish SHALL NOT create a local pin.

If that create fails for any reason other than the file already existing, the
finish SHALL answer `500` and SHALL NOT return an LDevID PEM. When a pin
already exists (including the concurrent loser), finish SHALL NOT rewrite it
and SHALL enforce it before signing. The store applies for both YAML and
Proxmox inventory whenever `ek_sha256` is empty and no cluster pin applies.

The service SHALL NOT delete or replace a pin automatically on TPM reset,
failed enroll, inventory reload, absence of the VM from inventory, or a
differing cluster pin.

#### Scenario: First successful finish creates a pin file

- **GIVEN** VM `100` with empty inventory `ek_sha256`, no cluster pin and no
  file `/var/lib/<package>/ek-pins/100`
- **WHEN** `enroll/finish` succeeds for that VM with EK fingerprint `abc…`
- **THEN** the file `/var/lib/<package>/ek-pins/100` contains `abc…` and a
  later enroll with a different trusted EK for VM `100` is rejected

#### Scenario: Second finish does not overwrite an existing pin

- **GIVEN** `/var/lib/<package>/ek-pins/100` already holding fingerprint `abc…`
- **WHEN** `enroll/finish` succeeds again with the same EK
- **THEN** the file content remains `abc…`

#### Scenario: Unusable pin directory fails closed before issuance

- **GIVEN** VM `100` with no pin yet and `/var/lib/<package>/ek-pins` not
  writable by the service user
- **WHEN** `enroll/finish` would otherwise issue an LDevID
- **THEN** the response is `5xx`, no LDevID PEM is returned, and no pin file
  is created

#### Scenario: A cluster pin replaces the local first-use pin

- **GIVEN** VM `100` with a valid cluster pin `abc…` and no file
  `/var/lib/<package>/ek-pins/100`
- **WHEN** `enroll/finish` succeeds for VM `100` with EK fingerprint `abc…`
- **THEN** no file `/var/lib/<package>/ek-pins/100` exists afterwards

### Requirement: Enforce The Stored EK Pin When Inventory Has None

On both `enroll/start` and `enroll/finish`, after resolving the caller VMID,
the effective EK pin SHALL be:

1. the inventory `ek_sha256` if non-empty after trim; otherwise
2. the caller's valid cluster pin (see "Read Cluster EK Pins From pmxcfs");
   otherwise
3. the contents of `/var/lib/<package>/ek-pins/<vmid>` if that file exists and
   contains a non-empty fingerprint; otherwise
4. no pin.

When the effective pin is non-empty, both legs SHALL require the header EK
certificate fingerprint to equal it (case-insensitive). A mismatch SHALL be
rejected with `401` and body `EK certificate does not match inventory pin`
when the pin came from inventory, `EK certificate does not match cluster pin`
when it came from the cluster pin, or `EK certificate does not match stored
pin` when it came from the store file.

When the cluster pin is the effective pin and a store file for the same VMID
holds a different fingerprint, the leg SHALL log one line
`devid enroll: local pin differs from cluster pin vm=<id>`, without either
fingerprint, and SHALL leave the store file unchanged.

This extends, and does not replace, the existing inventory-pin scenarios in
"Bind The EK Certificate To Request, Pin And Session".

#### Scenario: Stored pin rejects a different EK

- **GIVEN** `/var/lib/<package>/ek-pins/100` holding fingerprint `abc…`,
  inventory `ek_sha256` empty and no cluster pin for VM `100`
- **WHEN** either enroll leg is called for VM `100` with a trusted EK whose
  fingerprint is not `abc…`
- **THEN** the response is `401` with body
  `EK certificate does not match stored pin`

#### Scenario: Inventory pin wins over a conflicting store file

- **GIVEN** inventory `ek_sha256` set to fingerprint `aaa…` and store file
  for the same VMID holding `bbb…`
- **WHEN** either enroll leg is called with EK fingerprint `aaa…`
- **THEN** the request is not rejected for pin mismatch (other auth checks
  still apply)

#### Scenario: Cluster pin rejects a different EK

- **GIVEN** a valid cluster pin `abc…` for VM `100` and inventory
  `ek_sha256` empty
- **WHEN** either enroll leg is called for VM `100` with a trusted EK whose
  fingerprint is not `abc…`
- **THEN** the response is `401` with body
  `EK certificate does not match cluster pin`

#### Scenario: Cluster pin wins over a differing store file

- **GIVEN** a valid cluster pin `aaa…` for VM `100` and
  `/var/lib/<package>/ek-pins/100` holding `bbb…`
- **WHEN** either enroll leg is called for VM `100` with EK fingerprint `aaa…`
- **THEN** the request is not rejected for pin mismatch, the leg logs
  `devid enroll: local pin differs from cluster pin vm=100`, and the store
  file still holds `bbb…`

#### Scenario: A migrated VM is checked on its new node

- **GIVEN** VM `100` with a valid cluster pin `abc…`, first enrolled on
  another node, and no store file for `100` on this node
- **WHEN** either enroll leg is called for VM `100` on this node with an EK
  whose fingerprint is not `abc…`
- **THEN** the response is `401` with body
  `EK certificate does not match cluster pin`

#### Scenario: No pin allows first enroll

- **GIVEN** empty inventory `ek_sha256`, no cluster pin, no store file for
  the VMID and `require_cluster_pin: false`
- **WHEN** either enroll leg is called with a trusted EK bound to the
  endorsement key
- **THEN** the pin check does not reject the request

### Requirement: Refuse An EK Pinned To Another VM

On both `enroll/start` and `enroll/finish`, after the EK certificate is
trusted and bound to the endorsement key and after the caller's own effective
pin is enforced, the service SHALL refuse the leg when the caller's EK
fingerprint is pinned to a **different** VM ID: when a file
`/var/lib/<package>/ek-pins/<other>` whose name is a valid VM ID other than
the caller's holds that fingerprint (case-insensitive), when a valid cluster
pin `<cluster_pin_dir>/<other>` for another VM ID holds it, or when another
VM in the current inventory has that fingerprint as its non-empty
`ek_sha256`.

On a host whose inventory backend is Proxmox, a store file or cluster pin
whose VM ID is not listed in the cluster's VM list (`/etc/pve/.vmlist`) SHALL
NOT refuse anyone. When that list cannot be read or parsed, no pin SHALL be
ignored for this reason and the leg SHALL log one line saying so.

A refused leg SHALL answer `401` with body
`EK certificate is pinned to another VM`, SHALL NOT name the other VM in the
response, SHALL NOT issue a challenge or an LDevID and SHALL NOT create a pin
for the caller. Its log line SHALL name the caller's VM ID and the VM ID
that holds the pin and SHALL NOT contain the fingerprint. `enroll/finish`
SHALL, when it creates the caller's pin, repeat the check atomically with
that create, so that two VMs presenting one EK concurrently never both
receive a pin.

A pin file SHALL count even when the same VM's inventory `ek_sha256` or
cluster pin overrides it for that VM. Empty pin files and files whose name is
not a VM ID (digits only, so a set-aside `399.old` does not count) SHALL be
ignored. When the local pin directory exists but cannot be listed, the leg
SHALL answer `500` with body `pin store unavailable`; a missing directory
SHALL count as an empty store. The service SHALL NOT delete or rewrite any
pin because of a refusal.

#### Scenario: A clone with its source's EK is refused

- **GIVEN** `/var/lib/<package>/ek-pins/399` holding fingerprint `abc…`
- **WHEN** either enroll leg is called by VM `390` with a trusted EK whose
  fingerprint is `abc…`
- **THEN** the response is `401` with body
  `EK certificate is pinned to another VM`, no pin file for `390` exists,
  and the log line names `vm=390` and `pinned=399`

#### Scenario: A clone is refused on a node that never saw its source

- **GIVEN** a valid cluster pin `/etc/pve/vtpm-ek/399` holding `abc…`, VM
  `399` listed in `/etc/pve/.vmlist`, and no store file for `399` on this
  node
- **WHEN** either enroll leg is called on this node by VM `390` with EK
  fingerprint `abc…`
- **THEN** the response is `401` with body
  `EK certificate is pinned to another VM` and the log line names `vm=390`
  and `pinned=399`

#### Scenario: A destroyed VM's pin refuses nobody

- **GIVEN** a Proxmox host, a valid cluster pin and a store file for VM
  `399` holding `abc…`, and VM `399` not listed in `/etc/pve/.vmlist`
- **WHEN** either enroll leg is called by VM `450` with EK fingerprint `abc…`
- **THEN** the leg is not refused by this requirement

#### Scenario: Two first enrollments with one EK leave one pin

- **GIVEN** no pin files and VMs `390` and `399` presenting the same EK
- **WHEN** both call `enroll/finish` concurrently
- **THEN** at most one of them receives an LDevID and exactly one pin file
  holds that fingerprint

#### Scenario: The VM that holds the pin still enrolls

- **GIVEN** `/var/lib/<package>/ek-pins/399` holding fingerprint `abc…`
- **WHEN** either enroll leg is called by VM `399` with that EK
- **THEN** the leg is not refused by this requirement

#### Scenario: A set-aside pin file refuses nobody

- **GIVEN** `/var/lib/<package>/ek-pins/399.old` holding fingerprint `abc…`
  and no other pin file holding it
- **WHEN** either enroll leg is called by VM `399` with that EK
- **THEN** the leg is not refused by this requirement

#### Scenario: Another VM's inventory pin refuses

- **GIVEN** VM `100` with inventory `ek_sha256` `aaa…`
- **WHEN** either enroll leg is called by VM `101` with EK fingerprint `aaa…`
- **THEN** the response is `401` with body
  `EK certificate is pinned to another VM`

#### Scenario: Unlistable pin store fails closed

- **GIVEN** a pin directory the service cannot list
- **WHEN** either enroll leg is called
- **THEN** the response is `500` with body `pin store unavailable` and no
  LDevID is issued
