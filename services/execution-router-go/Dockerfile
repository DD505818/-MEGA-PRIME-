FROM golang:1.26.5-alpine3.24 AS builder
WORKDIR /app
RUN apk add --no-cache gcc musl-dev
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -tags musl -o execution-router .

FROM alpine:3.24.1
COPY --from=builder /app/execution-router /usr/local/bin/
CMD ["execution-router"]
