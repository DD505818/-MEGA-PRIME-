# HSM signing design — AEGIS approval key

Current state: `AEGIS_APPROVAL_PRIVKEY` is injected as an environment variable
(fail-fast if unset — good). Target: the private key **never exists as bytes
outside a hardware device**. Signing happens on the device; risk-service only
ever sees signatures.

## Design

- Device: YubiKey 5 (PIV slot 9c — digital signature) now; any PKCS#11 HSM
  later without changing the interface.
- risk-service holds: PKCS#11 URI
  `pkcs11:token=OMEGA-AEGIS;object=aegis-sign;type=private` + the PIN via a
  prompter (never env, never file — PIN entry at process start or via
  `ssh-agent`-style prompter; exact mechanism chosen at implementation).
- Signing flow: risk-service builds the approval payload (fields 1–14),
  sends the digest to the device, receives the ECDSA signature, attaches it
  as field 15. Verification path (VULTURE, TruthCore) is UNCHANGED — they
  already verify against the pinned public key.
- Touch policy: `always` (physical touch per signature) for the highest
  assurance, or `cached` with a short timeout for PAPER throughput. This is
  Devon's call at setup; the design supports both.

## Operator steps — DEVON'S HARDWARE, DEVON'S HANDS

These steps run on the machine where the YubiKey is plugged in. **I cannot do
them: I cannot generate, see, or handle your device material, and no part of
this design asks me to.** The private key is generated ON the device and never
leaves it.

```bash
# 1. Inspect the key
ykman piv info

# 2. Change the default PIN/PUK (defaults are public: 123456 / 12345678)
ykman piv access change-pin
ykman piv access change-puk
# Optionally set a management key (protects PIV admin operations)
ykman piv access change-management-key --generate --protect

# 3. Generate the signing key ON the device (slot 9c, ECDSA P-256,
#    PIN required always, touch required per signature)
ykman piv keys generate --touch-policy always --pin-policy always \
  9c /tmp/aegis-pub.pem

# 4. Self-sign a device certificate (or have your CA sign the CSR — the CSR
#    never exposes the private key)
ykman piv certificates generate --subject "CN=OMEGA-AEGIS-SIGN" 9c /tmp/aegis-pub.pem

# 5. Extract the PUBLIC key and pin it in risk-service config
#    (this is the ONLY key material that ever leaves the device)
openssl x509 -in <(ykman piv certificates export 9c -) -pubkey -noout > aegis-sign.pub.pem
#    → set AEGIS_APPROVAL_PUBKEY (or config file) to this value; risk-service
#      and VULTURE verify against it exactly as they do today.

# 6. Record the slot, token label, and touch/PIN policy in the operator
#    runbook. Store the PUK in the same vault as the break-glass docs —
#    losing PIN+PUK bricks the slot.
```

## What changes in risk-service (implementation, deferred)

- Replace `approval.ParsePrivateKey(os.Getenv(...))` with a PKCS#11 signer
  (Go: `github.com/ThalesIgnite/crypto11` or `github.com/miekg/pkcs11`).
- Startup still fails closed: no device / wrong PIN / missing token →
  risk-service refuses to start, exactly like a missing env key today.
- The approval payload, signature scheme (ECDSA), and verification code do
  not change — only where the private key lives.

## Boundary (explicit)

- No private key material is generated, stored, or transmitted by any agent,
  script, or CI job in this program. Ever.
- If the YubiKey is lost: the pinned public key is replaced (new device, new
  key, new pinning) and the old public key is revoked in config — same
  procedure as the PKI emergency rotation (`infrastructure/pki/ROTATION.md`).
