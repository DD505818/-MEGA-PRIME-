FROM golang:1.22-alpine AS builder
RUN apk add --no-cache gcc musl-dev librdkafka-dev pkgconf
WORKDIR /src
COPY apps ./apps
WORKDIR /src/apps/execution-service
RUN go mod download && go mod verify
RUN CGO_ENABLED=1 go build -tags musl -trimpath -o execution-service .

FROM alpine:3.19
RUN apk add --no-cache librdkafka ca-certificates
WORKDIR /app
COPY --from=builder /src/apps/execution-service/execution-service .
USER 65532:65532
EXPOSE 8081
CMD ["./execution-service"]
