# Simple Dockerfile for the inventory service
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/inventory ./cmd/inventory

FROM alpine:3.18
COPY --from=builder /bin/inventory /bin/inventory
EXPOSE 50051
ENTRYPOINT ["/bin/inventory"]
