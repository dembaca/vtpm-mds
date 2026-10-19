# Spec Delta

## ADDED Requirements

### Requirement: Limit Enroll Attempts Per Peer

`POST /latest/devid/enroll/start` and `POST /latest/devid/enroll/finish` SHALL
count toward a per-peer enroll budget of at most 6 requests within each 60
second window, using the same peer key as the instance-metadata guest
throttle. A request that exceeds that budget SHALL be refused with status
`429`, `Content-Type: text/plain`, body `Too many requests` followed by a
newline, and a `Retry-After` header, and SHALL NOT create a session, consume a
session, or issue a certificate.

Those requests SHALL also count toward the general per-peer HTTP request
budget of the instance-metadata guest throttle. The enroll budget SHALL NOT
replace authentication, EK binding, or any other enroll check: a peer under
budget is still subject to every existing enroll requirement.

#### Scenario: Enroll flood from one peer is refused

- **WHEN** a single peer has already sent 6 enroll start or finish requests
  within 60 seconds and sends another
- **THEN** the service responds `429` with body `Too many requests` and does
  not create or consume an enroll session

#### Scenario: A normal enroll stays under budget

- **WHEN** a peer performs one successful `enroll/start` and one successful
  `enroll/finish` within 60 seconds and is under the general request budget
- **THEN** both legs are handled by the existing enroll requirements and are
  not refused as `429` for the enroll limit
