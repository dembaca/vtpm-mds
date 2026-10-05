# Spec Delta

## REMOVED Requirements

### Requirement: Register The Identity Endpoints With TPM Attestation

**Reason**: The two routes it registers are removed, so there is nothing left
to gate. The requirement also ties their registration to
`mds.enable_tpm_attestation`; that setting survives this change because the
same block registers DevID enrollment, but it no longer has anything to say
about identity endpoints. Its second paragraph — that neither route depends on
`mds.enable_ec2_compat`, a DevID CA or a TPM device — becomes vacuous with the
routes gone.

**Migration**: None for a caller that observed the `enable_tpm_attestation:
false` behaviour: `GET /latest/identity` and `GET /.well-known/jwks.json` are
answered `404` with the body `404 page not found` by the catch-all handler,
which is byte for byte what this requirement already specified for that case.
A caller that had them enabled now sees that same `404` unconditionally.
DevID enrollment keeps its existing gating, specified unchanged by
`devid-enrollment`.

### Requirement: Require A Session Token For The Identity Document

**Reason**: It specifies the IMDSv2 token check on `GET /latest/identity` and
the absence of one on `GET /.well-known/jwks.json`. Both endpoints are removed,
so both halves describe routes that no longer exist.

**Migration**: None. A request that would have been answered `401` with
`X-Aws-Ec2-Metadata-Token: required` is now answered `404` by the catch-all,
because the route is gone before the token middleware is reached. A caller that
distinguishes the two learns only that the endpoint does not exist, which is
true. The token store itself is unchanged and still protects the EC2 metadata
tree.

### Requirement: Name The Bound VM In The Identity Document

**Reason**: It specifies which VM the JWT names, that `X-Forwarded-For` is
never consulted for it, and how an unbound caller is refused under
`mds.require_vm_identity`. With the endpoint removed there is no document to
name anyone.

Note that this requirement is the **only** place `workload-identity` says
something that was not a placeholder: it was written by
`refuse-unbound-metadata-callers` to close a real defect, where
`/latest/identity` ignored the inventory entirely and let any caller choose its
own subject via `X-Forwarded-For`. That fix is not being reverted — the
endpoint it fixed is being deleted, which refuses every caller rather than
merely the unbound ones.

**Migration**: A workload that needs to know which VM the service believes it
is talking to SHALL read `GET /latest/meta-data/instance-id`, which
`instance-metadata` specifies with the same inventory binding, the same refusal
of unbound callers under `mds.require_vm_identity`, and the same rule that no
request header may name the caller. It returns the identical `i-<vm id>` value
that the `sub` and `instance_id` claims carried, as plain text rather than
inside a JWT. The difference is only the envelope, because the JWT's signature
was never verifiable: it was `HS256` under a secret compiled into the binary,
against a published key set whose single entry carried the literal modulus
`placeholder-modulus`.

A verifier that wanted a *cryptographically checkable* statement about the
guest SHALL use the `devid-enrollment` capability instead. It issues an IEEE
802.1AR LDevID certificate whose subject is the authenticated VM id and whose
residency in the guest's TPM is proven by credential activation against the EK
certificate. That is the path SPIRE's `tpm_devid` node attestor consumes, and
the path a Teleport TPM join is restricted to via its EK CA allow-list. Neither
consumed this JWT, and neither could have: the service listens on
`169.254.169.1`, so no verifier outside the IMDS bridge can fetch the key set.

### Requirement: Serve The Identity Document As An Unverifiable Placeholder

**Reason**: Its subject is the placeholder itself — the `HS256` signature under
a compiled-in secret, the key set whose only entry carries the literal modulus
`placeholder-modulus`, the `level` claim fixed at `unattested`, the `attest`
claim fixed at false because no attestation result is consulted, and the
`mds.jwt_ttl` setting that nothing reads. The requirement is accurate and that
is precisely the problem: it records as a contract an endpoint that affirms an
identity it cannot substantiate. Removing the endpoint is the only change that
makes the contract true.

**Migration**: See the migration note on `Name The Bound VM In The Identity
Document`. Additionally: `mds.jwt_ttl`, `mds.jwks_path` and
`mds.attestation_ca` remain accepted in `config.yaml` after this change and now
have no effect at all. `mds.jwt_ttl` already had none — this requirement
specified that the five minute lifetime was fixed in code. Their removal is a
separate proposal, because dropping a configuration key is a migration question
for deployed files rather than a consequence of deleting a handler.
