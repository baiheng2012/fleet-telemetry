package bus

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/baiheng2012/fleet-telemetry/internal/vehicle"
)

const (
	StreamName    = "TELEMETRY"
	SubjectFilter = "telemetry.>"
)

// Subject 形如 telemetry.LHD.LHD-001
func Subject(r vehicle.Reading) string {
	return fmt.Sprintf("telemetry.%s.%s", r.VehicleType, r.VehicleID)
}

func Connect(url string) (*nats.Conn, jetstream.JetStream, error) {
	nc, err := nats.Connect(url, nats.Name("fleet-telemetry"))
	if err != nil {
		return nil, nil, fmt.Errorf("connect nats: %w", err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, nil, fmt.Errorf("jetstream: %w", err)
	}
	return nc, js, nil
}

// EnsureStream 幂等地创建/更新流，发布端和消费端启动时都调用。
func EnsureStream(ctx context.Context, js jetstream.JetStream) (jetstream.Stream, error) {
	return js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      StreamName,
		Subjects:  []string{SubjectFilter},
		Storage:   jetstream.FileStorage,
		Retention: jetstream.LimitsPolicy,
		MaxAge:    24 * time.Hour,
	})
}
