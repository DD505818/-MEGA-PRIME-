FROM golang:1.22-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go mod tidy && go mod verify
RUN CGO_ENABLED=1 go build -tags musl -trimpath -o risk-service .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/risk-service /usr/local/bin/risk-service
USER 65532:65532
CMD ["/usr/local/bin/risk-service"]
