FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY proto ./proto
COPY api-gateway ./api-gateway
WORKDIR /src/api-gateway
RUN go mod download && go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -mod=mod -o /bin/gateway ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates curl && addgroup -S app && adduser -S app -G app
COPY --from=builder /bin/gateway /bin/gateway
USER app
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=3 CMD curl -fsS http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/bin/gateway"]
