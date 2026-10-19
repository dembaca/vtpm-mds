## ADDED Requirements

### Requirement: Reload The Cache When The Inventory Source Changes

The service SHALL check its inventory source at a fixed interval of a few
seconds while it runs, and SHALL reload the cache through the same path as
`SIGHUP` when the content of any file the selected backend reads differs from
the content at the last load attempt, whether that attempt came from startup,
`SIGHUP` or an earlier check. For the Proxmox backend those files
are this node's guest configurations, this node's description and the cluster
configuration; for the YAML backend, the inventory file.

A change SHALL be detected regardless of which cluster node wrote it. An
unchanged source SHALL cause no reload and no log line. A failed reload SHALL
keep the previously cached records in effect and SHALL be logged once; the
source is loaded again when it next changes. The check SHALL stop when the
service shuts down.

#### Scenario: A guest created on this node is bound without a reload

- **GIVEN** a running service on a Proxmox node
- **WHEN** a guest with a NIC on the IMDS bridge is created on this node and
  started, and no `SIGHUP` is sent
- **THEN** within one interval the cache holds its record, and the guest's
  next connection is bound to its VM id

#### Scenario: A config written by another node is picked up

- **GIVEN** a running service on a Proxmox node
- **WHEN** a guest configuration appears under this node's `qemu-server`
  directory because another node wrote it (a migration to this node)
- **THEN** within one interval the cache holds its record

#### Scenario: A changed MAC rebinds on the next connection

- **GIVEN** a running service with a cached guest
- **WHEN** the guest's MAC on the IMDS bridge is changed and no `SIGHUP` is
  sent
- **THEN** within one interval the cache holds the new MAC, and a connection
  from the new MAC is bound to the guest

#### Scenario: Nothing changes, nothing is logged

- **GIVEN** a running service whose inventory source does not change
- **WHEN** several intervals pass
- **THEN** no reload happens and the journal shows no reload line

#### Scenario: A broken source keeps the previous inventory

- **GIVEN** a running service using the YAML backend with a populated cache
- **WHEN** the inventory file is made unparseable
- **THEN** the error is logged once, the previously cached records remain in
  effect, and fixing the file reloads it within one interval
