module github.com/omega-prime-delta/execution-service

go 1.23

toolchain go1.23.6

require (
	github.com/confluentinc/confluent-kafka-go/v2 v2.3.0
	github.com/go-redis/redis/v8 v8.11.5
	github.com/google/uuid v1.6.0
)

require (
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
)

require (
	github.com/alicebob/miniredis/v2 v2.39.0
	github.com/omega-prime-delta/approval v0.0.0
	github.com/omega-prime-delta/modelock v0.0.0
	github.com/omega-prime-delta/truthclient v0.0.0
)

replace (
	github.com/omega-prime-delta/approval => ../approval
	github.com/omega-prime-delta/modelock => ../modelock
	github.com/omega-prime-delta/truthclient => ../truthclient
)
