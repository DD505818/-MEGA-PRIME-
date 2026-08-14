FROM rust:1.85-bookworm AS builder
RUN apt-get update \
    && apt-get install -y --no-install-recommends cmake libssl-dev pkg-config zlib1g-dev \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY Cargo.toml ./
RUN mkdir src \
    && printf 'fn main() {}\n' > src/main.rs \
    && cargo generate-lockfile \
    && cargo build --release --locked
COPY . .
RUN cargo build --release --locked

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates libssl3 zlib1g \
    && rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/target/release/market-data-service /usr/local/bin/market-data-service
USER 65532:65532
CMD ["/usr/local/bin/market-data-service"]
