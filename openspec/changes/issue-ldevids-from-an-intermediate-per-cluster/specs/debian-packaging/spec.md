## ADDED Requirements

### Requirement: Document Intermediate DevID CA Material On The Host Package

The host package's shipped configuration comments and unit `LoadCredential=`
documentation SHALL describe the DevID CA material on the node as the
**cluster intermediate** certificate and private key, not the offline root
key. They SHALL note that SPIRE (or another relying party) needs the offline
**root certificate** (not the root key) as its trust anchor, and that the
root private key MUST NOT be installed on hypervisors.

This is documentation and example paths only; it SHALL NOT change postinst
reload/restart behaviour, package split, or which binary owns which path.

Exact filenames and whether the intermediate key stays a systemd credential
named `devid_ca.key` are settled with dembaca (design Q1–Q2); until then the
recommendation is to keep the existing credential **name** and point its
source at the intermediate key file.

#### Scenario: Packaged comments name intermediate not root key

- **GIVEN** the host package as built from this change
- **WHEN** an operator reads the shipped `config.yaml` comments and the unit
  credential documentation
- **THEN** they describe loading a cluster intermediate for signing, and warn
  that the offline root private key must not be placed on the node

#### Scenario: Packaging behaviour otherwise unchanged

- **GIVEN** a host upgrade that only updates those comments
- **WHEN** the package is installed
- **THEN** postinst reload/restart rules and package contents other than the
  commented paths remain as in the living `debian-packaging` requirements
