FROM golang:1.26.5-alpine3.24 AS builder
ENV GOTOOLCHAIN=go1.26.6
RUN apk add --no-cache gcc musl-dev librdkafka-dev pkgconf
WORKDIR /src
COPY apps ./apps
WORKDIR /src/apps/fusion-engine
RUN go mod download && go mod verify
RUN CGO_ENABLED=1 go build -tags musl -trimpath -o fusion-engine .

FROM alpine:3.24
RUN apk add --no-cache librdkafka ca-certificates
WORKDIR /app
COPY --from=builder /src/apps/fusion-engine/fusion-engine .
USER 65532:65532
EXPOSE 8085
CMD ["./fusion-engine"]
