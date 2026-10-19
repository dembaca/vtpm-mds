## ADDED Requirements

### Requirement: Persist An EK Pin Per VM After First Successful Enrollment

On a successful `enroll/finish` that issues an LDevID, when the caller's VMID
has **no** inventory `ek_sha256` and **no** pin file yet, the service SHALL
persist the lowercase hex SHA-256 digest of that enrollment's EK certificate
DER to `/var/lib/<package>/ek-pins/<vmid>` (one line, no other fields). The
`ek-pins` directory SHALL be created if missing. The write SHALL use an
exclusive create (or equivalent) so two concurrent first finishes leave a
single pin.

The pin write SHALL be attempted **before** the LDevID is signed when no pin
exists yet, so an unusable state directory fails the finish with `5xx` rather
than issuing a certificate that is not pinned. When a pin already exists, finish
SHALL NOT rewrite it.

The service SHALL NOT delete or replace a pin automatically on TPM reset,
failed enroll, inventory reload, or absence of the VM from inventory.

#### Scenario: First successful finish creates a pin file

- **GIVEN** VM `100` with empty inventory `ek_sha256` and no file
  `/var/lib/<package>/ek-pins/100`
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

### Requirement: Enforce The Stored EK Pin When Inventory Has None

On both `enroll/start` and `enroll/finish`, after resolving the caller VMID,
the effective EK pin SHALL be:

1. the inventory `ek_sha256` if non-empty after trim; otherwise
2. the contents of `/var/lib/<package>/ek-pins/<vmid>` if that file exists and
   contains a non-empty fingerprint; otherwise
3. no pin.

When the effective pin is non-empty, both legs SHALL require the header EK
certificate fingerprint to equal it (case-insensitive). A mismatch SHALL be
rejected with `401` and body `EK certificate does not match inventory pin`
when the pin came from inventory, or `EK certificate does not match stored pin`
when the pin came from the store file.

This extends, and does not replace, the existing inventory-pin scenarios in
"Bind The EK Certificate To Request, Pin And Session".

#### Scenario: Stored pin rejects a different EK

- **GIVEN** `/var/lib/<package>/ek-pins/100` holding fingerprint `abc…` and
  inventory `ek_sha256` empty
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

#### Scenario: No pin allows first enroll

- **GIVEN** empty inventory `ek_sha256` and no store file for the VMID
- **WHEN** either enroll leg is called with a trusted EK bound to the
  endorsement key
- **THEN** the pin check does not reject the request
