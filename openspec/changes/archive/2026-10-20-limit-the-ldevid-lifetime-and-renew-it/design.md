# Design

## Context

Today `CA.IssueDevID` always sets `NotAfter: NoExpiration`
(`9999-12-31T23:59:59Z`). After #70 archives, the living issuance requirement
is `Issue A Non-Expiring LDevID Whose Subject Is Only The VM ID` (CN-only).
E2.3 replaces the far-future validity with a finite, configurable lifetime and
a guest-side renew path.

SPIRE `tpm_devid` (tag `v1.14.0`) builds the agent SPIFFE ID as:

```text
spiffe://<trust_domain>/spire/agent/tpm_devid/<fingerprint>
```

where `<fingerprint>` is the **SHA-1 of the ASN.1 DER of the LDevID
certificate** — not of the public key. Server plugin sets `CanReattest: true`.

## Decisions

### D-a. Lifetime config key, default, and upper bound

- Key: `mds.devid_lifetime` (string).
- Parse with `time.ParseDuration` (same family as `token_ttl`).
- Default when omitted: `2160h` (90 × 24 hours).
- Reject ≤ 0, unparseable values, and values **greater than `8760h` (one
  calendar year of hours)** at `config.Load`. A year is already long for a
  guest LDevID in this deployment; unbounded lifetimes recreate the defect
  this change removes.
- Pass the duration into the CA; `IssueDevID` computes
  `notAfter = notBefore.Add(lifetime)`.

`time.ParseDuration` has no `d` unit; 90 days is spelled `2160h` in YAML. The
packaged config comment states that.

### D-b. SPIRE agent ID and whether renew is visible (lab, SPIRE 1.14.0)

**Docs:** agent ID = SHA-1(cert DER). Same TPM key + new cert ⇒ new ID.

**Lab evidence (guest100, 2026-10-19):**

| Step | Result |
|---|---|
| Four prior enroll PEMs for `CN=100` | Distinct SHA-1 fingerprints (`4307f45a…`, `bce76c54…`, …) |
| Clear `agent-data.json`, start agent | `Node attestation was successful` → `…/tpm_devid/4307f45a…`, `reattestable=true` |
| Restart, **keep** SVID cache, same LDevID | `SVID loaded` for `…/4307f45a…` — **no** node attestation |
| Swap LDevID files to `devid-out-65` (`bce76c54…`), keep SVID cache, restart | Still `SVID loaded` for **`…/4307f45a…`** (old ID); agent healthy — **LDevID not consulted** |
| Clear SVID cache, restart with new LDevID | Attests as `…/tpm_devid/bce76c54…`; server `agent list` shows both IDs, `Can re-attest: true` |

**Answers for the coordinator:**

1. After a restart with a cached agent SVID, the agent **reuses the old agent
   ID** and does not re-attest; the on-disk LDevID is ignored until attestation
   runs.
2. An expired LDevID therefore does **not** stop a running agent that already
   holds an SVID (same code path as (1); not separately clock-skewed in the
   lab). Lifetime only bites at **node attestation** time.
3. Renew must **delete the agent SVID cache** (`agent-data.json` and key
   material under the agent `data_dir`) before `try-restart`, so the next start
   attests with the new certificate. `tpm_devid` is already reattestable
   (`CanReattest: true` in server plugin and in lab `agent list`).

**Risk for dembaca:** if a guest runs without clearing the SVID cache after
renew, SPIRE keeps the old agent identity and never learns the new LDevID —
finite lifetime would not enforce anything for that agent until the SVID is
gone. The unit in this change clears the cache on cert change; operators who
renew by hand must do the same.

Registration should use `tpm_devid:subject:cn:<vmid>`, not the fingerprint
path.

### D-c. Keep the DevID TPM key across renewals

Because the agent ID changes whenever the certificate changes, regenerating
the TPM key does **not** preserve SPIRE identity. Keeping the key avoids extra
TPM creates and still requires E2.4 cleanup for the previous agent SPIFFE ID.
Generate a new key only when blobs are missing or cannot load under the SPIRE
H-1 parent.

### D-d. When the guest renews, and SPIRE restart

Renew when any of: no cert; `notAfter` ≤ now; remaining lifetime `<` one third
of `(notAfter - notBefore)` of **that** certificate.

Order:

1. Oneshot **before** `spire-agent.service` (`Before=` / `Wants=` only — **not**
   `Requires=` / `Requisite=`, so a guest without SPIRE still succeeds).
2. Daily timer.
3. If and only if the certificate PEM bytes changed: remove the SPIRE agent
   SVID cache under the configured agent `data_dir`, then
   `systemctl try-restart spire-agent.service` (no failure if the unit is
   absent or inactive).

### D-e. Stale SPIRE agents (E2.4) — man page

Verified on SPIRE **1.14.0** (`/opt/spire/bin/spire-server`):

| Command | Flags |
|---|---|
| `agent evict` | `-spiffeID`, `-socketPath`, `-output` |
| `agent purge` | `-expiredFor` (default `720h`), `-dryRun`, `-socketPath`, `-output` |

Document the beta playbook in the **`devid-enroll` man page** (this change's
packaging task). The coordinator adds a short pointer in the root README when
archiving. No reconciler.

### D-f. Packaging surface

`devid-enroll` gains `devid-enroll.service` (oneshot), `devid-enroll.timer`,
timer enable in `postinst`, man page updates. Host package: config key only.

### D-g. Serial includes validity (RFC 5280 §4.1.2.2)

`uniqueSerial` today hashes CA DER + subject + public key only. With D-c, a
renew would mint a **different** certificate with the **same** serial under
the same CA — forbidden by RFC 5280 §4.1.2.2, and `devid issued serial=` would
stop identifying a single issuance.

**Decision:** include `notBefore` and `notAfter` (UTC, encoded stably) in the
serial hash. Same key + same subject + same validity window still collide
(acceptable); a renew with a new `notBefore` gets a new serial. Spec scenario:
"renewal with the same key yields a new serial".

(Alternative considered: 128-bit random serial. Rejected for this record so
logs stay correlatable for a single issuance without a side store; revisit if
hashing validity proves awkward.)

## Non-goals

- Intermediate CA hierarchy / per-cluster CA.
- Automatic eviction/purge reconciler.
- Changing enroll HTTP paths or JSON fields.
- Making LDevID lifetime an IMDS metadata key.
- Editing root `README.md` in this PR (coordinator at archive).

## Risks

- Fingerprint-keyed registration entries break on renew — use CN selectors.
- Forgetting to clear the agent SVID cache leaves SPIRE on the old identity
  (lab A); the unit must clear it; call out for dembaca if operators renew
  manually.
- Apply-order clash with #70 if both REMOVED the pre-#70 title — see proposal.
