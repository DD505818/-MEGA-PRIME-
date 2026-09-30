FROM golang:1.26.5-alpine3.24 AS builder
ENV GOTOOLCHAIN=go1.26.6
RUN apk add --no-cache gcc musl-dev
WORKDIR /src
COPY apps ./apps
WORKDIR /src/apps/risk-service
RUN go version && go mod download && go mod verify
RUN CGO_ENABLED=1 go build -tags musl -trimpath -o risk-service . && go version -m risk-service

FROM alpine:3.24
RUN apk add --no-cache ca-certificates
COPY --from=builder /src/apps/risk-service/risk-service /usr/local/bin/risk-service
USER 65532:65532
CMD ["/usr/local/bin/risk-service"]
