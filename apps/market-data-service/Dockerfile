FROM rust:1.75-alpine as builder
WORKDIR /app
COPY Cargo.toml ./
RUN mkdir src && echo "fn main() {}" > src/main.rs
RUN cargo build --release
COPY . .
RUN cargo build --release
FROM alpine:3.19
COPY --from=builder /app/target/release/market-data-service /usr/local/bin/
CMD ["market-data-service"]
