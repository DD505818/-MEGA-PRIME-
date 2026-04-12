FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SERVICE=cmd/risk-engine
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/service ./${SERVICE}

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=build /out/service /app/service
EXPOSE 3000
ENTRYPOINT ["/app/service"]
