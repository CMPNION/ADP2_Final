FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/gateway ./cmd
FROM alpine:3.18
COPY --from=builder /bin/gateway /bin/gateway
EXPOSE 8080
ENTRYPOINT ["/bin/gateway"]
