package bus

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/baiheng2012/fleet-telemetry/internal/sparkplug"
	"github.com/baiheng2012/fleet-telemetry/internal/vehicle"
)

type MQTTPublisher struct {
	c           mqtt.Client
	group, node string
	seq         atomic.Uint64
}

func NewMQTT(url, group, node string) (*MQTTPublisher, error) {
	opts := mqtt.NewClientOptions().AddBroker(url).SetClientID("sim-" + node).SetAutoReconnect(true)
	c := mqtt.NewClient(opts)
	t := c.Connect()
	if !t.WaitTimeout(5*time.Second) || t.Error() != nil {
		return nil, fmt.Errorf("mqtt connect %s: %v", url, t.Error())
	}
	return &MQTTPublisher{c: c, group: group, node: node}, nil
}

func (p *MQTTPublisher) Publish(_ context.Context, r vehicle.Reading) error {
	seq := p.seq.Add(1) % 256 // Sparkplug 的 seq 范围 0..255
	payload := sparkplug.Encode(sparkplug.FromReading(r, seq))
	tok := p.c.Publish(sparkplug.Topic(p.group, "DDATA", p.node, r.VehicleID), 1, false, payload)
	if !tok.WaitTimeout(5 * time.Second) {
		return errors.New("mqtt publish timeout")
	}
	return tok.Error()
}

func (p *MQTTPublisher) Close() { p.c.Disconnect(250) }
