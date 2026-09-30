module github.com/omega-prime-delta/portfolio-service

go 1.22

require (
	github.com/confluentinc/confluent-kafka-go/v2 v2.3.0
	github.com/go-redis/redis/v8 v8.11.5
	github.com/jackc/pgx/v5 v5.5.0
)

require (
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	golang.org/x/crypto v0.9.0 // indirect
	golang.org/x/sync v0.1.0 // indirect
	golang.org/x/text v0.9.0 // indirect
)

require github.com/omega-prime-delta/modelock v0.0.0

replace github.com/omega-prime-delta/modelock => ../modelock
