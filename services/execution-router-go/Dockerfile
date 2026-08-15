FROM golang:1.26.5-alpine3.24 AS builder
ENV GOTOOLCHAIN=go1.26.6
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go version && go mod download
COPY . .
RUN CGO_ENABLED=1 go build -tags musl -o execution-router ./main.go && go version -m execution-router

FROM alpine:3.22
COPY --from=builder /app/execution-router /usr/local/bin/
CMD ["execution-router"]
