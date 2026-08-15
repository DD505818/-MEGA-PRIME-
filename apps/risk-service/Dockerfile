FROM golang:1.22-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -tags musl -o risk-service .
FROM alpine:3.22
COPY --from=builder /app/risk-service /usr/local/bin/
CMD ["risk-service"]
