FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY proto ./proto
COPY services/inventory ./services/inventory
WORKDIR /src/services/inventory
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -mod=mod -o /bin/inventory ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates curl && addgroup -S app && adduser -S app -G app
COPY --from=builder /bin/inventory /bin/inventory
USER app
EXPOSE 50051
HEALTHCHECK --interval=10s --timeout=3s --retries=3 CMD curl -fsS http://127.0.0.1:9090/healthz || exit 1
ENTRYPOINT ["/bin/inventory"]
