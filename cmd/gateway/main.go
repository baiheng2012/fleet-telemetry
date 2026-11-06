package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/baiheng2012/fleet-telemetry/internal/bus"
	"github.com/baiheng2012/fleet-telemetry/internal/sparkplug"
)

func main() {
	mqttURL := flag.String("mqtt", "tcp://localhost:1883", "MQTT broker")
	natsURL := flag.String("nats", "nats://localhost:4222", "NATS url")
	group := flag.String("group", "fleet", "Sparkplug group id")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pub, err := bus.NewNATS(ctx, *natsURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pub.Close()

	var okN, badN atomic.Int64
	handler := func(_ mqtt.Client, m mqtt.Message) {
		_, _, device, err := sparkplug.ParseTopic(m.Topic())
		if err != nil {
			badN.Add(1)
			return
		}
		p, err := sparkplug.Decode(m.Payload())
		if err != nil {
			badN.Add(1)
			log.Printf("decode %s: %v", m.Topic(), err)
			return
		}
		r, err := sparkplug.ToReading(device, p)
		if err != nil {
			badN.Add(1)
			log.Println("convert:", err)
			return
		}
		pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := pub.Publish(pctx, r); err != nil {
			badN.Add(1)
			log.Println("publish to jetstream:", err)
			return
		}
		okN.Add(1)
	}

	filter := "spBv1.0/" + *group + "/DDATA/+/+"
	opts := mqtt.NewClientOptions().
		AddBroker(*mqttURL).
		SetClientID("fleet-gateway").
		SetCleanSession(false). // 持久会话：断线期间 broker 为它排队 QoS1 消息
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetOnConnectHandler(func(c mqtt.Client) { // 重连后自动重新订阅
			t := c.Subscribe(filter, 1, handler)
			t.Wait()
			if t.Error() != nil {
				log.Println("subscribe:", t.Error())
				return
			}
			log.Println("subscribed:", filter)
		})
	c := mqtt.NewClient(opts)
	if t := c.Connect(); t.Wait() && t.Error() != nil {
		log.Fatal(t.Error())
	}

	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			c.Disconnect(500)
			log.Println("gateway stopped")
			return
		case <-tick.C:
			log.Printf("forwarded=%d dropped=%d", okN.Load(), badN.Load())
		}
	}
}
