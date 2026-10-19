## ADDED Requirements

### Requirement: Admit Only The Configured MAC On Each Guest Port Of The IMDS Bridge

On a Proxmox node, the host package SHALL keep an nftables `bridge` table
`vtpm_mds_bind` in which, for every NIC `net<n>` of every guest configuration
under this node's `qemu-server` directory whose `bridge=` is the IMDS bridge,
frames entering from the port `tap<vmid>i<n>` are dropped when their source
MAC differs from the MAC configured for that NIC, ARP packets entering from
that port are dropped when their sender hardware address differs from it, and
802.1Q and 802.1ad tagged frames entering from that port are dropped.
This SHALL hold for every NIC model and whether or not the NIC has
`firewall=1`. NICs on other bridges SHALL NOT be affected.

The table SHALL be derived from the same files the inventory reads, SHALL be
rewritten within one inventory check interval of a change to them, and SHALL be
restored within one interval when it is missing or differs from what was last
applied. Each rewrite SHALL replace the whole table in one transaction. An
unchanged source SHALL cause no rewrite and no log line.

The metadata daemon SHALL NOT write the table and SHALL NOT hold
`CAP_NET_ADMIN`.

#### Scenario: A guest claiming another MAC is not answered

- **GIVEN** a running guest whose NIC on the IMDS bridge is configured with
  MAC A
- **WHEN** the guest sets that interface to MAC B and requests metadata
- **THEN** its frames are dropped at its port and the request gets no answer

#### Scenario: A forged ARP sender is dropped

- **GIVEN** a running guest whose NIC on the IMDS bridge is configured with
  MAC A
- **WHEN** it sends an ARP packet from MAC A whose sender hardware address is
  MAC B
- **THEN** the packet is dropped at its port

#### Scenario: A tagged frame is dropped

- **GIVEN** a running guest whose NIC on the IMDS bridge is configured with
  MAC A
- **WHEN** it sends an 802.1Q tagged frame from MAC A
- **THEN** the frame is dropped at its port

#### Scenario: The configured MAC is answered

- **GIVEN** a running guest using the MAC configured for its NIC on the IMDS
  bridge
- **WHEN** it requests metadata
- **THEN** it is answered as before

#### Scenario: A new guest is covered without an operator

- **GIVEN** a running node
- **WHEN** a guest with a NIC on the IMDS bridge is created on this node or
  its configuration arrives here by migration
- **THEN** within one interval the table holds the rules for its port

#### Scenario: A flushed ruleset is restored

- **GIVEN** a node whose table is in place
- **WHEN** the nftables ruleset is flushed and reloaded from its files
- **THEN** within one interval the table again matches the guest
  configurations

#### Scenario: Other bridges are untouched

- **GIVEN** a guest with a NIC on a bridge other than the IMDS bridge
- **WHEN** it sends frames with any source MAC on that NIC
- **THEN** no rule of the table matches them
