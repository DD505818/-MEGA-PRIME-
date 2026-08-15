FROM golang:1.26.5-alpine3.24 AS builder
ENV GOTOOLCHAIN=go1.26.6
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go version && go mod download
COPY . .
RUN CGO_ENABLED=1 go build -tags musl -o risk-service . && go version -m risk-service
FROM alpine:3.22
COPY --from=builder /app/risk-service /usr/local/bin/
CMD ["risk-service"]
