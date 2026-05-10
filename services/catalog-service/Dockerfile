FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY proto ./proto
COPY services/catalog-service ./services/catalog-service
WORKDIR /src/services/catalog-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -mod=mod -o /bin/catalog ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates curl && addgroup -S app && adduser -S app -G app
COPY --from=builder /bin/catalog /bin/catalog
USER app
EXPOSE 8081
HEALTHCHECK --interval=10s --timeout=3s --retries=3 CMD curl -fsS http://127.0.0.1:8081/healthz || exit 1
ENTRYPOINT ["/bin/catalog"]
