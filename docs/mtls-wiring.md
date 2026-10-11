# mTLS wiring design — ΩMEGA PRIME Δ

Goal: every service-to-service hop authenticates both ends with certificates
from the ΩMEGA PRIME Δ root CA (`infrastructure/pki/`). This doc is the
**design**; the PKI scripts are real and runnable, the service wiring is
specified here but the code changes are DEFERRED (see below).

## PKI layout (shipped in this PR)

```
infrastructure/pki/
  make-ca.sh        # one-time: create root CA (offline machine recommended)
  issue-cert.sh     # per service: issue <service>.key/.crt (SANs: service, localhost)
  ROTATION.md       # rotation + emergency revocation procedure
  ca/               # created by make-ca.sh — ca.key NEVER committed/deployed
  certs/<service>/  # created by issue-cert.sh
```

Trust anchor: `ca.crt` mounted read-only into every service container.
Identity: `<service>.key` + `<service>.crt`, file perms 600/644.

## Go services (risk, execution, portfolio, capital-allocator, fusion, truth-core)

Design (NOT yet implemented):

```go
// Load once at startup; fail closed if any material is missing.
caPool := x509.NewCertPool()
caPEM, err := os.ReadFile("/certs/ca.crt")          // fatal if missing
caPool.AppendCertsFromPEM(caPEM)

cert, err := tls.LoadX509KeyPair("/certs/svc.crt", "/certs/svc.key") // fatal if missing

srv := &http.Server{
    Addr: ":8080",
    TLSConfig: &tls.Config{
        Certificates: []tls.Certificate{cert},
        ClientAuth:   tls.RequireAndVerifyClientCert,  // mTLS, not just TLS
        ClientCAs:    caPool,
        MinVersion:   tls.VersionTLS13,
    },
}
srv.ListenAndServeTLS("", "") // certs already in TLSConfig
```

Outbound (service → service, service → Kafka): `tls.Config{RootCAs: caPool,
Certificates: []tls.Certificate{cert}}` on the dialer / franz-go `kgo.DialTLSConfig`.

**DEFERRED — explicit:** these Go changes are not in this PR. There is no Go
toolchain in the build environment to compile or test them, and hand-editing
TLS/crypto code without verification is worse than deferring. The wiring lands
as a separate PR built where `go build ./...` and the service tests run green.

## Python services (agent/strategy, feature-engine, llm-service)

```python
ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
ctx.load_verify_locations("/certs/ca.crt")
ctx.load_cert_chain("/certs/svc.crt", "/certs/svc.key")
ctx.verify_mode = ssl.CERT_REQUIRED
ctx.minimum_version = ssl.TLSVersion.TLSv1_3
# use ctx with httpx / kafka client socket params
```

Same deferral applies: specified here, implemented where the Python suites run.

## Kafka

Migration path: add an `SSL://kafka:9094` listener with
`KAFKA_SSL_KEYSTORE_LOCATION` / truststore from the PKI material (JKS via
`keytool -import`, or PEM with `ssl.keystore.type=PEM` on Kafka 3.x — our
7.5 supports PEM keystores). SCRAM (9093) and SSL (9094) compose: SASL_SSL.
Cutover order mirrors the SASL plan: stage SSL alongside, move clients, then
remove PLAINTEXT. Client SASL_SSL wiring is part of the deferred client change.

## Postgres / Redis

- Postgres: mount server cert/key, set `ssl=on`, `ssl_ca_file` for client-cert
  verification of the replica connection; clients use `sslmode=verify-full`.
- Redis: `tls-port 6379`, `tls-cert-file`/`tls-key-file`/`tls-ca-cert-file`,
  `tls-auth-clients yes`. Note: Sentinel and replicas need matching TLS config —
  do this as one coordinated change, not piecemeal.

## What "done" looks like

`ss -tlnp` on the deployment host shows no service accepting unauthenticated
plaintext on the compose network; a client presenting no certificate gets a
TLS handshake failure (not an HTTP 401 — failure happens before HTTP).
Negative test per service, logged in the chaos-drill evidence format.
