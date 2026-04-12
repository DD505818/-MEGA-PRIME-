# Disaster Recovery

- Restore PostgreSQL from latest backup snapshot.
- Re-run `backend/db/schema.sql` migration for missing tables.
- Restart services in dependency order: Postgres -> Kafka -> Risk -> Allocator -> Execution.
- Verify leadership heartbeat updates in `leader_election`.
