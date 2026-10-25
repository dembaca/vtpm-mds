## ADDED Requirements

### Requirement: Refuse An EK Pinned To Another VM

On both `enroll/start` and `enroll/finish`, after the EK certificate is
trusted and bound to the endorsement key and after the caller's own effective
pin is enforced, the service SHALL refuse the leg when the caller's EK
fingerprint is pinned to a **different** VM ID: when a file
`/var/lib/<package>/ek-pins/<other>` whose name is a valid VM ID other than
the caller's holds that fingerprint (case-insensitive), or when another VM in
the current inventory has that fingerprint as its non-empty `ek_sha256`.

A refused leg SHALL answer `401` with body
`EK certificate is pinned to another VM`, SHALL NOT name the other VM in the
response, SHALL NOT issue a challenge or an LDevID and SHALL NOT create a pin
for the caller. It SHALL log one line
`devid refused ek pinned to another vm vm=<caller> pinned=<other>` and SHALL
NOT log the fingerprint in it.

Empty pin files and files whose name is not a valid VM ID SHALL be ignored.
When the pin directory exists but cannot be listed, the leg SHALL answer
`500` with body `pin store unavailable`; a missing directory SHALL count as
an empty store. The service SHALL NOT delete or rewrite any pin because of a
refusal.

#### Scenario: A clone with its source's EK is refused

- **GIVEN** `/var/lib/<package>/ek-pins/399` holding fingerprint `abc…`
- **WHEN** either enroll leg is called by VM `390` with a trusted EK whose
  fingerprint is `abc…`
- **THEN** the response is `401` with body
  `EK certificate is pinned to another VM`, no pin file for `390` exists,
  and the log has one line naming `vm=390` and `pinned=399`

#### Scenario: The VM that holds the pin still enrolls

- **GIVEN** `/var/lib/<package>/ek-pins/399` holding fingerprint `abc…`
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
