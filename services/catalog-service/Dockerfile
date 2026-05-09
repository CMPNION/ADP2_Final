FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/service ./cmd
FROM alpine:3.18
COPY --from=builder /bin/service /bin/service
ENTRYPOINT ["/bin/service"]
