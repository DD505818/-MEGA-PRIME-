# Kafka SASL/SCRAM + ACLs — ΩMEGA PRIME Δ

Threat model: a compromised or buggy strategy/agent service must not be able to
publish `orders.approved`, and no service except the execution engine may consume
it. The authority chain (agent proposes → AEGIS authorizes → VULTURE executes) is
enforced at the transport layer, not just in code.

## What this PR configures (broker side)

`docker-compose.yml` (kafka service):

- Two listeners: `PLAINTEXT://kafka:9092` (existing clients) and
  `SASL_PLAINTEXT://kafka:9093` (hardened path).
- `KAFKA_SASL_ENABLED_MECHANISMS: SCRAM-SHA-512` with a JAAS config mounted from
  `infrastructure/kafka/kafka_server_jaas.conf` (SCRAM credentials live in
  Zookeeper, not in the file).
- `KAFKA_AUTHORIZER_CLASS_NAME: kafka.security.auth.SimpleAclAuthorizer`
  (ZK-backed authorizer for the cp-kafka 7.5 / ZK deployment).
- `KAFKA_ALLOW_EVERYONE_IF_NO_ACL_FOUND: "false"` — deny by default. Any
  principal without an explicit ACL gets nothing.

Keeping PLAINTEXT during the transition is deliberate: current Go/Python clients
have no SASL wiring (code change deferred — see below). Removing PLAINTEXT is the
final cutover step, only after clients authenticate on 9093.

## Principal map

| Service | SCRAM user | Allowed operations |
|---|---|---|
| strategy-engine (agent-service) | `User:strategy` | PRODUCE `orders.proposed` only |
| risk-engine (AEGIS) | `User:risk` | CONSUME `orders.proposed`; PRODUCE `orders.approved` only |
| execution-engine (VULTURE) | `User:execution` | CONSUME `orders.approved` only |
| TruthCore / audit readers | `User:audit` | CONSUME `fills.executed`, `audit.events` |
| operator (break-glass) | `User:admin` | ALL (human use only, documented per use) |

Nobody else may produce `orders.approved` — that is the whole point.

## One-time bootstrap procedure (operator, on the deployment host)

Prerequisites: stack up, kafka healthy on 9092.

```bash
# 1. Create SCRAM credentials in Zookeeper (use a password manager; never commit).
for u in strategy risk execution audit admin; do
  read -s -p "password for $u: " pw; echo
  docker compose exec kafka kafka-configs.sh --zookeeper zookeeper:2181 \
    --alter --add-config "SCRAM-SHA-512=[password=${pw}]" \
    --entity-type users --entity-name "$u"
done

# 2. Create the ACLs (run as admin; client config holds admin's SCRAM creds).
cat > /tmp/admin-scram.properties <<'EOF'
security.protocol=SASL_PLAINTEXT
sasl.mechanism=SCRAM-SHA-512
sasl.jaas.config=org.apache.kafka.common.security.scram.ScramLoginModule required username="admin" password="ADMIN_PW";
EOF
ACL="docker compose exec kafka kafka-acls.sh --bootstrap-server kafka:9093 \
  --command-config /tmp/admin-scram.properties \
  --authorizer-properties zookeeper.connect=zookeeper:2181"

$ACL --add --allow-principal User:strategy   --producer --topic orders.proposed
$ACL --add --allow-principal User:risk       --consumer --topic orders.proposed --group risk-group
$ACL --add --allow-principal User:risk       --producer --topic orders.approved
$ACL --add --allow-principal User:execution  --consumer --topic orders.approved --group execution-group
$ACL --add --allow-principal User:audit      --consumer --topic fills.executed --group audit-group
$ACL --add --allow-principal User:audit      --consumer --topic audit.events   --group audit-group

# 3. Verify (deny-by-default check): an unauthenticated producer must fail,
#    and each principal sees ONLY its ACLs.
docker compose exec kafka kafka-acls.sh --bootstrap-server kafka:9093 \
  --command-config /tmp/admin-scram.properties \
  --authorizer-properties zookeeper.connect=zookeeper:2181 --list

# 4. Negative test: produce to orders.approved as User:strategy → must be denied.
```

## Client cutover (DEFERRED — code change, not in this PR)

Each Kafka client needs SASL wiring (Go: franz-go/sarama SASL/SCRAM dialer;
Python: `sasl_mechanism`/`sasl_plain_username`/`sasl_plain_password` in the
client kwargs). Contract for that change:

```yaml
# environment (compose) — values injected, never committed
KAFKA_BROKERS: kafka:9093
KAFKA_SECURITY_PROTOCOL: SASL_PLAINTEXT
KAFKA_SASL_MECHANISM: SCRAM-SHA-512
KAFKA_SASL_USERNAME: strategy   # per-service principal
KAFKA_SASL_PASSWORD: ${STRATEGY_KAFKA_PASSWORD:?missing}
```

Until the clients are wired, they stay on PLAINTEXT://kafka:9092 and the ACLs
do not constrain them — the broker config above is staged, not yet enforcing.
Cutover order: wire clients → verify on 9093 → remove the PLAINTEXT listener →
re-verify. The mode contract is unaffected throughout (modelock still fails
closed independently of transport auth).
