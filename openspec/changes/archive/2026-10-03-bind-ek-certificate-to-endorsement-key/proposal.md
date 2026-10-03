# Proposal

## Why

The EK factor of enroll authentication proves possession of a public
certificate, not possession of the TPM that certificate describes.

`VerifyEKCertificate` takes the endorsement key from the signing request and
discards it — `_ = pub // optional future: compare pub vs cert.PublicKey` — so
verification is chain building only. An attacker who obtains any EK certificate
that chains to `ek_ca_chain` can present it in `X-vtpm-mds-ek-cert` and in its
own signing request, while the signing request carries *its own* TPM's
endorsement key. The credential-activation challenge is then wrapped to the
attacker's endorsement key, the attacker activates it with its own TPM, and the
service issues an LDevID. Reproduced against a software TPM.

EK certificates are not secret. They are readable from NV index `0x01c00002` on
any guest that holds one, and are exchanged in the clear on both enroll legs.
So the factor that is supposed to anchor an LDevID in a specific vTPM is
satisfied by a value that any guest which ever enrolled can hand to another.

`ek_sha256` inventory pinning inherits the same weakness: the pin is compared
against the presented certificate, which is exactly the value that is not bound
to anything.

The `devid-enrollment` spec records this as current behaviour under
`Trust The EK Certificate Through The Configured EK CA Chain`, including a
scenario that ends `enrollment succeeds and a certificate is issued`. Closing
the gap means changing that contract.

## What Changes

- **BREAKING** On `enroll/start`, the public key of the presented EK
  certificate must equal the endorsement key carried in the signing request.
  A mismatch is rejected with `401` and the body
  `EK certificate does not match the endorsement key in the signing request`.
- **BREAKING** A signing request that carries an endorsement certificate but no
  endorsement key public area is rejected with `401`, because the binding
  cannot be checked. Today such a request reaches challenge creation and fails
  there with `400`.
- Chain building, the SAN workaround, validity checking, the header/request DER
  equality check and the inventory pin are unchanged. The pin becomes
  meaningful rather than changing behaviour.
- `enroll/finish` keeps verifying the chain only. It has no signing request, and
  the session already binds the EK certificate fingerprint proven at start.
- No change to the wire format of either leg, to the session lifetime, to the
  issued certificate, or to the guest client. A guest enrolling with its own
  TPM sees no difference.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `devid-enrollment`: `Trust The EK Certificate Through The Configured EK CA
  Chain` is replaced by `Trust The EK Certificate And Bind It To The
  Endorsement Key`. Its paragraph beginning `Verification is chain building
  only` and its scenario `EK certificate public key is not bound to the TPM`
  specify the defect and cannot be carried forward.
  `Prove TPM Residency With A Credential Activation Challenge` gains the
  statement that the challenge is now provably wrapped to the certified TPM.

## Impact

- `internal/devid/verify.go`: `VerifyEKCertificate` — the discarded `pub`
  parameter is the defect.
- `internal/devid/auth.go`: `AuthenticateEnrollCaller` needs the signing
  request's endorsement key on the start leg.
- `internal/devid/http.go`: `HandleStart` already decodes the signing request
  before authenticating and can pass it.
- `internal/devid/enroll.go`: `Enroller.Start` currently calls
  `VerifyEKCertificate` a second time.
- `internal/devid/devid_test.go`, `internal/devid/auth_test.go`: gain the
  mismatch case, which is the regression test for this defect.
- `internal/devid/enroll_swtpm_test.go` and
  `scripts/qemu-lab/e2e-devid-guest.sh`: an swtpm EK certificate is issued over
  the TPM's own endorsement key, so both should pass unchanged — that is the
  check that the fix does not break legitimate enrollment.
- The `devid-enrollment` capability's `## Purpose` describes the EK factor as
  a certificate that chains to a configured EK CA. It needs one sentence
  updated when this change is archived.
