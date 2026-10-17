# Design

## Context

`remove-the-unverifiable-identity-endpoints` (archived 2026-10-16) removed the
attestation and JWT identity endpoints and left
`/latest/dynamic/instance-identity/{document,signature}` alone on purpose. Its
task 5.1 recorded the maintainer's answer of 2026-10-05: both paths stay *for
now*, the constant signature is a defect that gets its own change, and research
decides whether that change implements a real signature or removes the
constant. This is that change. The research result and the maintainer's
decision are summarised in `proposal.md`.

Four facts were established by reading the tree, and they shape the work:

1. The route is registered in `internal/server/server.go`, through the
   `handle` helper, which wraps it in the token check and — because the path
   is in `imds.IdentityBearingPaths` — in `RequireVMIdentity`. The handler
   itself is `imds.HandleInstanceIdentitySignature`.
2. The service has never served the other two AWS signature paths,
   `/latest/dynamic/instance-identity/pkcs7` and `.../rsa2048`, nor the
   directory listing `GET /latest/dynamic/instance-identity/`. All three already
   fall through to the catch-all `http.NotFound` in `internal/server/server.go`.
3. `mds.signing_key_path` is loaded into `config.MDSConfig.SigningKeyPath`,
   defaulted to `/var/lib/vtpm-mds/signing-key.pem`, and read by nothing. It
   was the slot a real signature would have used.
4. Nothing in the repository requests the path except its own test. A
   `git grep` for the path, for the constant and for
   `HandleInstanceIdentitySignature` finds the registration, the handler, four
   places in `imds/handlers_test.go`, the `instance-metadata` spec, and the
   documentation listed in `proposal.md` under Impact — no client, no script
   under `scripts/qemu-lab/`, no guest component.

## Goals

- Stop serving a value that has the form of a signature and is a constant.
- Leave the identity document exactly as it is.
- Make the specification, the man page, the README and the architecture
  document stop calling the path a signature.

## Non-Goals

- **Implementing a real signature** (`/signature`, `/rsa2048` or `/pkcs7`). The
  research found no consumer that would verify it against a key this service
  holds. If one appears, that is a new change with a consumer to design
  against, which this record does not have.
- **Removing or changing the document.** Decided 2026-10-05: it stays, as the
  genuine AWS path that cloud-init and the SDKs know.
- **Removing `mds.signing_key_path`.** It is dead configuration, but removing
  a key is a configuration migration of its own — the same reasoning
  `remove-the-unverifiable-identity-endpoints` applied to `jwks_path`,
  `jwt_ttl` and `attestation_ca`. Recommended as a follow-up together with
  those three.
- **Serving the `dynamic/instance-identity/` listing.** It would change what
  cloud-init does; see below.
- **Changing what `require_vm_identity` refuses** beyond removing a path that
  stops existing.

## Decisions

### Unregister the route rather than answer it with `404` or `410`

The path is removed from the route table, so the request reaches the
catch-all, which answers `404` with the body `404 page not found`. A dedicated
handler that answered `404` or `410 Gone` was considered and rejected:

- It would make `/signature` distinguishable from `/pkcs7` and `/rsa2048`, the
  two sibling paths this service never served, for no reader's benefit.
- `410` tells a client the resource existed. The only client that ever saw it
  was this repository's own test.
- It is what `remove-the-unverifiable-identity-endpoints` did with four
  endpoints, and that change's verification compared each one byte for byte to
  `GET /latest/no-such-thing`. Doing the same here makes the check reusable.

A consequence worth stating: today an unbound caller with a valid token gets
`404` (the `require_vm_identity` refusal), and a caller without a token gets
`401`. After this change **every** caller gets the catch-all `404`, with or
without a token, bound or not, because the catch-all checks neither. This is
the intended result — a path that does not exist — and the delta's scenario
pins it.

### The refused set shrinks; nothing is exposed by it

`IdentityBearingPaths` loses the signature entry, and `Refuse An Instance
Identity To Callers With No VM Record` enumerates two paths. The paragraph that
explained why a constant was refused alongside the document ("the two form one
feature …") is removed, not rewritten: once the path does not exist there is
nothing to justify. No path moves from refused to served.

### cloud-init is unaffected, and why that was checked rather than assumed

cloud-init's `DataSourceEc2` fetches the instance identity through
`get_instance_identity` in `cloudinit/sources/helpers/ec2.py`. That function
does **not** request `document` or `signature` by name: it fetches the
directory listing `dynamic/instance-identity/` and crawls the leaves the listing
names. A `404` on the listing is caught and yields an empty dictionary. This
service has never served that listing, so cloud-init has never reached
`signature` here, and this change does not alter what it sees. Read from
cloud-init's `main` branch on 2026-10-16; re-read it at implementation time
rather than citing this paragraph (task 1.3).

The same reading shows why the listing must **not** be added in passing: a
listing that names `signature` would turn a never-fetched leaf into a fatal
one, because `get_instance_identity` passes no exception callback for leaves.

### One requirement is replaced, the other modified

A `MODIFIED` block replaces the requirement whole at archive time, and the
OpenSpec CLI refuses one that omits a scenario the living spec still has — the
first draft of this delta was rejected for exactly that. The two requirements
are therefore handled differently, according to whether a scenario has to go.

- `Serve An Unsigned Instance Identity Document` has a scenario,
  `Identity signature is a constant placeholder`, whose outcome inverts. It
  cannot be kept under that name, so the requirement is **removed and
  re-added** as `Serve The Instance Identity Document Without A Signature`,
  the pattern `serve-a-public-address-or-omit-the-key` used for the same
  reason. Nothing outside the spec file refers to the old name, so the rename
  costs nothing. Every clause and scenario about the document is carried
  verbatim.
- `Refuse An Instance Identity To Callers With No VM Record` is **modified**
  under its own name, because that name is referenced from `imds/handlers.go`
  and from two places in `Derive The Instance Id From The Bound VM Record`;
  renaming it would widen this change into requirements it has no reason to
  touch. Its scenario `Unbound caller is refused both instance identity
  endpoints` stays true — both requests still answer `404` with the same body
  — so it keeps its name, and its outcome now says why each one does. The
  name reads a little oddly with one of the two paths gone; the alternative was
  a rename with ripple effects.

### Overlap with `serve-a-public-address-or-omit-the-key`

That change is being implemented at the same time, by another agent, and
touches `imds/handlers.go` too. Two consequences:

1. **Code.** This change edits `IdentityBearingPaths` and deletes one handler;
   that change edits `getPublicIP` and `HandleMetaDataIndex`. Different
   functions in one file: expect a textual merge at most, not a semantic
   conflict. Whichever merges second rebases.
2. **Spec.** `Refuse An Instance Identity To Callers With No VM Record` says
   "`public-ipv4` is always unavailable, and the index is a static listing".
   That change makes both halves untrue — `public-ipv4` will be served to a
   bound VM that declares an address, and the index will depend on the caller
   — but its delta does not modify this requirement. This delta carries the
   sentence **verbatim**, because correcting it is that change's scope, not
   this one's.
   Archiving this change will therefore re-assert the stale sentence. The
   archive order and the correction are the coordinator's, and the finding has
   been handed to them; it is recorded here so it is not lost if the message
   is.

## Risks

- **A consumer this repository cannot see reads `/signature`.** It would now
  get `404` instead of a constant. Nothing in the research or the tree points
  to one, and a consumer that treated the constant as meaningful was already
  misled. The release note in `debian/changelog` at packaging time should name
  the removal.
- **Someone reinstates a signature later without a consumer.** The removal
  clause in the delta is the guard: serving any signature path becomes a spec
  change, which has to say who verifies it.
