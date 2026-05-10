FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY services/auth-service ./services/auth-service
WORKDIR /src/services/auth-service/cmd
RUN go mod init omnichannel/auth-service || true
RUN go get github.com/golang-jwt/jwt/v5 golang.org/x/crypto/bcrypt
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/auth ./main.go

FROM alpine:3.20
RUN apk add --no-cache ca-certificates curl && addgroup -S app && adduser -S app -G app
COPY --from=builder /bin/auth /bin/auth
USER app
EXPOSE 8090
HEALTHCHECK --interval=10s --timeout=3s --retries=3 CMD curl -fsS http://127.0.0.1:8090/healthz || exit 1
ENTRYPOINT ["/bin/auth"]
