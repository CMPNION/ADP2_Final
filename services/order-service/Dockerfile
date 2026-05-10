FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY proto ./proto
COPY services/order-service ./services/order-service
WORKDIR /src/services/order-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/order ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates curl && addgroup -S app && adduser -S app -G app
COPY --from=builder /bin/order /bin/order
USER app
EXPOSE 8082
HEALTHCHECK --interval=10s --timeout=3s --retries=3 CMD curl -fsS http://127.0.0.1:8082/healthz || exit 1
ENTRYPOINT ["/bin/order"]
