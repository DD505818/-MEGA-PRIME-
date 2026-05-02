FROM golang:1.22-alpine AS builder
RUN apk add --no-cache gcc musl-dev librdkafka-dev pkgconf
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o execution-service .

FROM alpine:3.19
RUN apk add --no-cache librdkafka ca-certificates
WORKDIR /app
COPY --from=builder /app/execution-service .
EXPOSE 8081
CMD ["./execution-service"]
