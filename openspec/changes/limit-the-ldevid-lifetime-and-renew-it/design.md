# Design

## Context

Today `CA.IssueDevID` always sets `NotAfter: NoExpiration`
(`9999-12-31T23:59:59Z`). The living requirement "Issue A Non-Expiring LDevID
Bound To The Authenticated VM" records that. E2.3 replaces it with a finite,
configurable lifetime and a guest-side renew path.

SPIRE `tpm_devid` (documented at tag `v1.14.0`) builds the agent SPIFFE ID as:

```text
spiffe://<trust_domain>/spire/agent/tpm_devid/<fingerprint>
```

where `<fingerprint>` is the **SHA-1 of the ASN.1 DER of the LDevID
certificate** — not of the public key. Any new certificate therefore yields a
new agent ID.

## Decisions

### D-a. Lifetime config key and default

- Key: `mds.devid_lifetime` (string).
- Parse with `time.ParseDuration` (same family as `token_ttl`).
- Default when omitted: `2160h` (90 × 24 hours).
- Reject ≤ 0 and parse errors at `config.Load`.
- Pass the duration into the CA (constructor or field); `IssueDevID` computes
  `notAfter = notBefore.Add(lifetime)`.

`time.ParseDuration` has no `d` unit; 90 days is spelled `2160h` in YAML. The
packaged config comment states that.

### D-b. SPIRE agent ID on renewal (lab + docs)

**Finding (SPIRE 1.14.0 docs):** agent ID tracks the **certificate**
fingerprint. Same TPM key + new cert ⇒ new ID. New key + new cert ⇒ new ID.

**Lab evidence (cloud lab, guest100 shared outputs, 2026-10-19):** four
enrollments for `CN=100` produced four distinct SHA-1 cert fingerprints, e.g.
`4307f45a…`, `bce76c54…`, `4e5306bf…`, `385a8974…`. Those runs also used
different public keys because today's client always creates a new DevID key;
the docs alone already settle the same-key case.

**Implication for registration:** prefer selectors
`tpm_devid:subject:cn:<vmid>` (and issuer CN as needed). Do not bind workload
entries to `…/tpm_devid/<fingerprint>` if renewals are expected.

### D-c. Keep the DevID TPM key across renewals

Because the agent ID changes whenever the certificate changes, regenerating
the TPM key does **not** preserve SPIRE identity. Keeping the key:

- avoids an extra create under the SRK on every renew (less TPM traffic / DA
  surface);
- keeps proof-of-residency material continuous for the same key;
- still requires the E2.4 cleanup for the previous agent SPIFFE ID.

Generate a new key only when `devid.priv.blob` / `devid.pub.blob` are missing
or fail to load under the SPIRE H-1 parent (existing enroll path).

Renewal enroll: client presents the existing DevID public key in the signing
request (same wire protocol as today). Server serial stays deterministic from
CA + subject + public key; validity dates change, so the DER and fingerprint
change.

### D-d. When the guest renews

Renew (call enroll, replace `devid.crt.pem`, leave blobs if key kept) when any
of:

1. no certificate file at the configured out path;
2. certificate `notAfter` ≤ now (expired);
3. `notAfter - now` < `(notAfter - notBefore) / 3` (less than one third of the
   **certificate's own** lifetime remains).

Using the cert's own validity interval (not the current config default) means
a guest that was issued under a longer lifetime still renews on the one-third
rule of that cert after a config shortening.

Order:

1. `devid-enroll.service` oneshot (renew-or-skip) **before** `spire-agent.service`
   (`Before=` / `Wants=` as appropriate; fail the oneshot only on enroll
   errors, not on "still fresh").
2. `devid-enroll.timer` daily (and `OnBootSec` covered by the oneshot).
3. After a successful renew that changed the cert, restart `spire-agent` so it
   re-attests with the new materials (document in unit; avoid restart loops if
   unchanged).

Default out path remains the path SPIRE is configured to read (today's
package/docs: under `/var/lib/spire/agent/` or the packaged default — align
the unit's `-out` with the agent conf shipped or documented for the lab).

### D-e. Stale SPIRE agents (E2.4) — document only

Verified on this host's SPIRE **1.14.0** binary (`/opt/spire/bin/spire-server`):

| Command | Flags (1.14.0) |
|---|---|
| `agent evict` | `-spiffeID` (required), `-socketPath`, `-output` |
| `agent purge` | `-expiredFor` (default `720h`), `-dryRun`, `-socketPath`, `-output` |

Beta playbook (non-goal to automate):

- On VM delete or EK-pin delete: `spire-server agent evict -spiffeID
  spiffe://<td>/spire/agent/tpm_devid/<old-fingerprint>`.
- Periodic garbage collection of agents whose SVID expired:
  `spire-server agent purge -expiredFor 720h` (optionally `-dryRun` first).

No reconciler in this change.

### D-f. Packaging surface

**BREAKING relative to today's packaging claim:** `debian-packaging` currently
requires that `devid-enroll` install only the binary and man page. This change
adds:

- `devid-enroll.service` (oneshot),
- `devid-enroll.timer`,
- enable the timer in the guest package `postinst` (or equivalent),

and updates the man page for renew flags / behaviour. Host package untouched
except `mds.devid_lifetime` in the example config.

## Non-goals

- Intermediate CA hierarchy / per-cluster CA.
- Stripping guest subject attributes (E2.5).
- Automatic eviction/purge reconciler.
- Changing enroll HTTP paths or JSON fields.
- Making LDevID lifetime visible through IMDS metadata keys.

## Risks

- **Registration entries keyed by fingerprint** break on first renew — call
  out in Impact and operator notes; prefer CN selectors before enabling
  renew in production.
- **Agent ID churn** leaves stale agents until evict/purge — acceptable for
  beta per E2.4; monitor `agent list` after renew tests.
- **Daily timer + boot oneshot** must not stampede the enroll rate limit
  (throttle change): one renew per guest when due is enough; "still fresh"
  is a no-op without an enroll call.
