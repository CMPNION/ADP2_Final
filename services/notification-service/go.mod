module github.com/cmpnion/adp-final/services/notification

go 1.25.0

require (
github.com/mailgun/mailgun-go/v4 v4.14.0
github.com/nats-io/nats.go v1.51.0
google.golang.org/grpc v1.53.0
google.golang.org/protobuf v1.36.8
github.com/cmpnion/adp-final/proto v0.0.0
)

replace github.com/cmpnion/adp-final/proto => ../../proto
