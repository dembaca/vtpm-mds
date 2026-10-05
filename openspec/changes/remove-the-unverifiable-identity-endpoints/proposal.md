# Proposal

## Why

Two endpoints affirm without checking, and the key set that would let anyone
check them cannot be reached by a verifier.

`POST /latest/attest` answers `{"valid": true, "identity": "attested-identity"}`
to **any** request whose nonce matches. The four steps that would make the
answer mean something — verify the quote signature under the EK certificate,
verify the nonce inside the quote, check PCRs against a policy, extract the
claims — are a `TODO` comment in `attest/handlers.go`. The route is registered
with `mux.HandleFunc` rather than the token-wrapped `handle`, so it is not even
behind an IMDSv2 session token.

`GET /latest/identity` signs its JWT with `HS256` under a secret compiled into
the binary, and `GET /.well-known/jwks.json` publishes one RSA entry whose
modulus is the literal string `placeholder-modulus`. The published key set
therefore cannot verify the token it accompanies, and the secret is neither
private nor per-deployment. `openspec/specs/workload-identity/spec.md` already
records all of this as the current contract.

An endpoint that confirms an attestation it did not perform is worse than no
endpoint: it is a check a reviewer can point at, and it passes everyone.

The reason this is a removal rather than a repair:

- **Nothing consumes either endpoint.** No client, no lab script, no guest
  component. They appear only in `ARCHITECTURE.md`, in the route registration
  in `internal/server/server.go`, and as a refused path in a test. SPIRE reads
  the DevID files from disk and runs its own credential activation against the
  `.priv`/`.pub` blobs.
- **The JWKS cannot reach an external verifier.** The service listens on
  `169.254.169.1:80`, reachable from guests on the IMDS bridge and from the
  host. A SPIRE server, Vault or Teleport outside cannot fetch
  `/.well-known/jwks.json` at all. Real signing keys do not fix that; it is the
  design, not the implementation.
- **The shape was borrowed from the wrong cloud.** `/latest/` is AWS, and AWS
  has no `/latest/identity` and no JWKS. A signed instance identity token
  verified against the provider's public JWKS is the GCP and Azure pattern,
  which works because Google's key set is on the public internet. The pattern
  was copied without the part that carries it.
- **The two real integrations need neither.** Teleport's TPM join was checked
  against its current documentation on 2026-10-04, as the known-gaps note
  requires, rather than from memory: the joining host reads the EK public key
  and the EKCert **from its own TPM** and sends the EK public key hash and the
  EKCert to the Teleport Auth Service, which checks them against the join
  token's `ek_public_hash`, `ekcert_allowed_cas` and `ek_certificate_serial`.
  It contacts no metadata service, no IMDS and no JWKS endpoint, and needs no
  JWT. SPIRE's `tpm_devid` attestor likewise consumes the DevID PEM and the
  TPM2B blobs that `devid-enrollment` already produces.

`ARCHITECTURE.md` contradicts itself on exactly this point: its goals call the
JWT "consumable by SPIRE, Teleport, and Vault", while its own integration table
says Teleport integrates by *restricting to the EK CA* and SPIRE by *DevID PEM
+ TPM2B blobs*. Those two rows are the real use cases. The three rows that
depend on the JWT — SPIRE's JWT path, Vault `auth/jwt`, Kubernetes node labels
— all hang on the unreachable key set.

## What Changes

- **BREAKING** `GET /latest/attest/nonce` and `POST /latest/attest` are
  removed. Requests fall through to the catch-all handler and are answered
  `404` with the body `404 page not found`. The `attest` package is deleted,
  including `NonceStore`, which nothing else uses.
- **BREAKING** `GET /latest/identity` and `GET /.well-known/jwks.json` are
  removed on the same terms. The `identity` package is deleted.
- The `workload-identity` capability is archived: all three of its requirements
  are `REMOVED`, with a migration note naming what a consumer should use
  instead.
- `mds.enable_tpm_attestation` is **kept and unchanged**. The same `if` block
  in `internal/server/server.go:92` also registers the DevID enrollment routes,
  which are the path SPIRE and Teleport actually use, and `devid-enrollment`
  specifies that gating. Only the four route registrations inside the block go.
  See `design.md` for why the obvious rename is deliberately *not* in scope.
- `mds.jwt_ttl`, `mds.jwks_path` and `mds.attestation_ca` become dead settings.
  They are left in place by this change and are the subject of a separate
  proposal, because removing a configuration key is a migration question for
  deployed `config.yaml` files, not a side effect of deleting a handler.
- `ARCHITECTURE.md` is corrected: the goals bullet, the component diagram, four
  rows of the endpoint table, three rows of the integration table and roadmap
  phases 3 and 6.
- `/latest/dynamic/instance-identity/document` and `.../signature` are **not**
  touched. See `design.md` — this is the one open scope question, and the
  recommendation is to keep them and write their defect up separately.
- No change to DevID enrollment, to the EC2 metadata tree, to the token store,
  to `/health`, or to anything a bound guest reads today.

## Capabilities

### Removed Capabilities

- `workload-identity`: the JWT identity document and the key set that
  accompanies it. Every requirement in it describes a placeholder — an
  unverifiable signature, a key set that cannot verify it, and a configured TTL
  that nothing reads. Writing that behaviour down is what made the removal
  obvious; the spec is not wasted work, it is the evidence.

### Modified Capabilities

- `instance-metadata`: `Refuse An Instance Identity To Callers With No VM
  Record` enumerates the refused paths and lists `GET /latest/identity` among
  them. That bullet goes with the endpoint. The other three paths, the refusal
  semantics and every scenario are unchanged.
- `vm-inventory`: `Let Each Handler Decide What An Unbound Caller Receives`
  carries a bullet delegating `GET /latest/identity` to `workload-identity`.
  That bullet goes with the capability. Nothing else in the requirement
  changes.

## Impact

- `attest/` — deleted (`handlers.go`, `nonce.go`).
- `identity/` — deleted (`handlers.go`, `handlers_test.go`).
- `internal/server/server.go` — four route registrations, the `attest` and
  `identity` imports, the `nonceStore` field and `attest.SetStore`.
- `imds/handlers.go` — one line: `/latest/identity` leaves
  `IdentityBearingPaths`, and the doc comment above it that explains why the
  entry is there. **This is the only overlap with other in-flight work** and is
  called out in `tasks.md`.
- `imds/handlers_test.go` — the test asserting `/latest/identity` is a
  refused path.
- `ARCHITECTURE.md` — as listed above.
- `openspec/specs/workload-identity/spec.md` — removed at archive time.
- Any operator whose `config.yaml` sets `jwt_ttl`, `jwks_path` or
  `attestation_ca`: the keys keep loading and keep doing nothing. `jwt_ttl`
  already did nothing before this change.
- Ordering: this change should be applied **after**
  `fix-client-ip-fallback`, which modifies `identity/handlers.go` — a file this
  change deletes. Applying this one first would silently drop that work.
