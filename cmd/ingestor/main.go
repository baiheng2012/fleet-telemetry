package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/baiheng2012/fleet-telemetry/internal/bus"
	"github.com/baiheng2012/fleet-telemetry/internal/sink"
)

func main() {
	natsURL := flag.String("nats", "nats://localhost:4222", "NATS url")
	dsn := flag.String("pg", "postgres://postgres:dev@localhost:5433/warehouse", "warehouse DSN")
	batchSize := flag.Int("batch", 500, "max messages per batch")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	nc, js, err := bus.Connect(*natsURL)
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Drain()

	stream, err := bus.EnsureStream(ctx, js)
	if err != nil {
		log.Fatal(err)
	}
	cons, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       "raw-ingestor",
		FilterSubject: bus.SubjectFilter,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       30 * time.Second,
		MaxDeliver:    10,
	})
	if err != nil {
		log.Fatal(err)
	}

	var snk sink.Sink
	snk, err = sink.NewPostgres(ctx, *dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer snk.Close()

	log.Println("ingestor started")
	for ctx.Err() == nil {
		batch, err := cons.Fetch(*batchSize, jetstream.FetchMaxWait(time.Second))
		if err != nil {
			log.Println("fetch:", err)
			time.Sleep(time.Second)
			continue
		}

		var msgs []jetstream.Msg
		var events []sink.Event
		for m := range batch.Messages() {
			if !json.Valid(m.Data()) {
				log.Printf("invalid json on %s, terminating message", m.Subject())
				_ = m.Term() // 毒丸消息：不再重投（后续可送死信）
				continue
			}
			md, err := m.Metadata()
			if err != nil {
				log.Println("metadata:", err)
				_ = m.Nak()
				continue
			}
			msgs = append(msgs, m)
			events = append(events, sink.Event{
				StreamSeq:   int64(md.Sequence.Stream),
				Subject:     m.Subject(),
				PublishedAt: md.Timestamp,
				Payload:     m.Data(),
			})
		}
		if len(msgs) == 0 {
			continue
		}

		// 先写库，成功后才 Ack
		inserted, err := snk.WriteBatch(ctx, events)
		if err != nil {
			log.Println("write failed, will redeliver:", err)
			for _, m := range msgs {
				_ = m.NakWithDelay(2 * time.Second)
			}
			continue
		}
		for _, m := range msgs {
			_ = m.Ack()
		}
		log.Printf("batch: fetched=%d inserted=%d dup_skipped=%d", len(msgs), inserted, int64(len(msgs))-inserted)
	}
	log.Println("ingestor stopped")
}
