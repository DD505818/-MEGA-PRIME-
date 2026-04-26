FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o truth-core ./main.go

FROM alpine:3.19
COPY --from=builder /app/truth-core /usr/local/bin/
CMD ["truth-core"]
