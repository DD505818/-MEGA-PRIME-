FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o risk-engine ./main.go

FROM alpine:3.19
COPY --from=builder /app/risk-engine /usr/local/bin/
CMD ["risk-engine"]
