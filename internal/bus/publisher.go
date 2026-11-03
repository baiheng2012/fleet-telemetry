package bus

import (
	"context"
	"encoding/json"
	"os"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/baiheng2012/fleet-telemetry/internal/vehicle"
)

type Publisher interface {
	Publish(ctx context.Context, r vehicle.Reading) error
	Close()
}

// ---- stdout ----

type StdoutPublisher struct{ enc *json.Encoder }

func NewStdout() *StdoutPublisher { return &StdoutPublisher{enc: json.NewEncoder(os.Stdout)} }

func (p *StdoutPublisher) Publish(_ context.Context, r vehicle.Reading) error {
	return p.enc.Encode(r)
}
func (p *StdoutPublisher) Close() {}

// ---- NATS JetStream ----

type NATSPublisher struct {
	nc *nats.Conn
	js jetstream.JetStream
}

func NewNATS(ctx context.Context, url string) (*NATSPublisher, error) {
	nc, js, err := Connect(url)
	if err != nil {
		return nil, err
	}
	if _, err := EnsureStream(ctx, js); err != nil {
		nc.Close()
		return nil, err
	}
	return &NATSPublisher{nc: nc, js: js}, nil
}

// Publish 同步等待 JetStream 的持久化确认。
// 目前流量小够用；后面压测时可改成 PublishAsync 提高吞吐。
func (p *NATSPublisher) Publish(ctx context.Context, r vehicle.Reading) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = p.js.Publish(ctx, Subject(r), data)
	return err
}

func (p *NATSPublisher) Close() { _ = p.nc.Drain() }
