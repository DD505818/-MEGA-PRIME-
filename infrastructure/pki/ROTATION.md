# PKI rotation — ΩMEGA PRIME Δ

## Scheduled rotation

| Material | Validity | Rotation trigger |
|---|---|---|
| Root CA (`ca.crt`/`ca.key`) | 10 years | Calendar: re-run `make-ca.sh` in a NEW directory; cross-sign transition |
| Service certs (`certs/<svc>/`) | 2 years | 90 days before expiry: re-run `issue-cert.sh <svc>`, rolling restart |
| Kafka SCRAM passwords | — | Rotate per `docs/kafka-acls.md` bootstrap (alter user, rolling client restart) |

Service-cert rotation procedure (no downtime):

1. `issue-cert.sh <service>` → new `<service>.crt` (same key or new key).
2. Copy new cert next to the old one on the deployment host.
3. Rolling restart the service (`docker compose up -d <service>`); healthcheck
   must go green before moving to the next service.
4. Verify: `openssl s_client -connect localhost:<port> -CAfile ca.crt` shows
   the new dates; old cert removed from the host after all services confirm.

Root CA rotation (rare, planned): generate the new CA, issue all service certs
from it, distribute a bundle trust anchor (`ca.crt` + `ca-new.crt`) so old and
new certs verify during the transition window, then remove the old anchor.

## Emergency revocation (compromised key)

There is no OCSP/CRL infrastructure in this stack — revocation = replacement:

1. Kill-switch the affected service (`docker compose stop <service>`).
2. Re-issue its cert from a NEW key (`issue-cert.sh` always generates a fresh key).
3. If the CA itself is compromised: full PKI regeneration (new CA dir), re-issue
   all certs, rotate Kafka SCRAM passwords, rotate `AEGIS_APPROVAL_PRIVKEY`
   (see `docs/hsm-signing.md`), then bring services back one at a time with
   healthcheck verification.
4. Record everything in TruthCore/audit log: what was compromised, when rotation
   completed, who performed each step.

The CA private key must never exist on the deployment host. If it ever does
(someone copied it), treat the CA as compromised and rotate.
