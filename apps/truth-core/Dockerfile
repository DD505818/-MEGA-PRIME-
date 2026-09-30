FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY apps ./apps
WORKDIR /src/apps/truth-core
RUN go mod download && go mod verify
RUN CGO_ENABLED=0 go build -trimpath -o truth-core .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /src/apps/truth-core/truth-core .
USER 65532:65532
EXPOSE 8084
CMD ["./truth-core"]
