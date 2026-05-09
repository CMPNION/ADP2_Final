package natsinfra

import (
	"context"
	"log"

	"github.com/nats-io/nats.go"
)

type Publisher struct{ nc *nats.Conn }

func NewPublisher(url string) (*Publisher, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}
	return &Publisher{nc: nc}, nil
}

func (p *Publisher) Publish(subject string, payload []byte) error {
	return p.nc.Publish(subject, payload)
}

func (p *Publisher) Subscribe(subject string, handler func(msg *nats.Msg)) (*nats.Subscription, error) {
	sub, err := p.nc.Subscribe(subject, func(m *nats.Msg) { handler(m) })
	if err != nil {
		return nil, err
	}
	return sub, nil
}

func (p *Publisher) Close() {
	if p.nc != nil {
		p.nc.Close()
	}
}

// helper to subscribe with logging
func SubscribeWithLog(ctx context.Context, nc *nats.Conn, subject string, handler func(*nats.Msg)) error {
	_, err := nc.Subscribe(subject, func(m *nats.Msg) {
		log.Printf("nats recv %s len=%d", subject, len(m.Data))
		handler(m)
	})
	return err
}
