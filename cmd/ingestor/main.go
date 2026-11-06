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
	source := flag.String("source", "stream", "stream (vehicle telemetry) | cdc (legacy changes)")
	sinkKind := flag.String("sink", "postgres", "warehouse sink: postgres | snowflake")
	sfDSN := flag.String("sf-dsn", os.Getenv("SNOWFLAKE_DSN"), "Snowflake DSN (prefer env var; don't pass secrets on the command line)")

	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	nc, js, err := bus.Connect(*natsURL)
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Drain()

	var (
		stream                 jetstream.Stream
		durable, filter, table string
	)
	switch *source {
	case "stream":
		stream, err = bus.EnsureStream(ctx, js)
		durable, filter, table = "raw-ingestor", bus.SubjectFilter, "raw.telemetry_events"
	case "cdc":
		stream, err = bus.EnsureCDCStream(ctx, js)
		durable, filter, table = "cdc-ingestor", bus.CDCSubjectFilter, "raw.legacy_cdc_events"
	default:
		log.Fatalf("unknown -source %q", *source)
	}
	if err != nil {
		log.Fatal(err)
	}

	cons, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       durable,
		FilterSubject: filter,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       30 * time.Second,
		MaxDeliver:    10,
	})
	if err != nil {
		log.Fatal(err)
	}

	
	target := *dsn
	if *sinkKind == "snowflake" {
		target = *sfDSN
	}
	snk, err := sink.New(ctx, *sinkKind, target, table)




	if err != nil {
		log.Fatal(err)
	}
	defer snk.Close()

	log.Printf("ingestor started: source=%s table=%s", *source, table)
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
				_ = m.Term()
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
