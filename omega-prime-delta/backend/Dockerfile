FROM golang:1.22-alpine AS builder
WORKDIR /src

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG SERVICE=./cmd/risk-engine
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/service ${SERVICE}

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /out/service /usr/local/bin/omega-service

EXPOSE 3002
ENTRYPOINT ["/usr/local/bin/omega-service"]
