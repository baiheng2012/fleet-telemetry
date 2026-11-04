package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/baiheng2012/fleet-telemetry/internal/bus"
	"github.com/baiheng2012/fleet-telemetry/internal/cdc"
)

func main() {
	pgDSN := flag.String("pg", "postgres://postgres:dev@localhost:5434/legacy?replication=database", "legacy DSN (needs replication=database)")
	natsURL := flag.String("nats", "nats://localhost:4222", "NATS url")
	slot := flag.String("slot", "fleet_cdc", "replication slot")
	publication := flag.String("publication", "fleet_cdc", "publication name")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	nc, js, err := bus.Connect(*natsURL)
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Drain()
	if _, err := bus.EnsureCDCStream(ctx, js); err != nil {
		log.Fatal(err)
	}

	if err := run(ctx, js, *pgDSN, *slot, *publication); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
	log.Println("cdc stopped")
}

func run(ctx context.Context, js jetstream.JetStream, dsn, slot, publication string) error {
	conn, err := pgconn.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(context.Background())

	// 槽位必须在写入数据之前存在，之后的变更才不会丢
	if _, err := pglogrepl.CreateReplicationSlot(ctx, conn, slot, "pgoutput", pglogrepl.CreateReplicationSlotOptions{}); err != nil {
		var pe *pgconn.PgError
		if !errors.As(err, &pe) || pe.Code != "42710" { // 42710 = 已存在
			return fmt.Errorf("create slot: %w", err)
		}
		log.Printf("slot %q exists, resuming from its confirmed position", slot)
	} else {
		log.Printf("slot %q created", slot)
	}

	// startLSN=0：从槽位记录的 confirmed_flush_lsn 继续
	err = pglogrepl.StartReplication(ctx, conn, slot, 0, pglogrepl.StartReplicationOptions{
		PluginArgs: []string{"proto_version '1'", fmt.Sprintf("publication_names '%s'", publication)},
	})
	if err != nil {
		return fmt.Errorf("start replication: %w", err)
	}
	log.Println("cdc started")

	var (
		rels     = map[uint32]*pglogrepl.RelationMessage{}
		tx       []cdc.ChangeEvent
		finalLSN pglogrepl.LSN
		commitTS time.Time
		flushed  pglogrepl.LSN // 已确认发布到 JetStream 的位点
	)

	sendStatus := func(c context.Context) error {
		return pglogrepl.SendStandbyStatusUpdate(c, conn, pglogrepl.StandbyStatusUpdate{
			WALWritePosition: flushed, WALFlushPosition: flushed, WALApplyPosition: flushed,
		})
	}
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = sendStatus(c) // 尽力而为，位点没推进也只是下次重放
	}()

	mk := func(op string, relID uint32, before, after *pglogrepl.TupleData) (cdc.ChangeEvent, error) {
		rel, ok := rels[relID]
		if !ok {
			return cdc.ChangeEvent{}, fmt.Errorf("unknown relation %d", relID)
		}
		return cdc.ChangeEvent{
			Op: op, Schema: rel.Namespace, Table: rel.RelationName,
			LSN: int64(finalLSN), Seq: len(tx), CommitTS: commitTS,
			Before: cdc.TupleToMap(rel, before), After: cdc.TupleToMap(rel, after),
		}, nil
	}

	handle := func(msg pglogrepl.Message) error {
		var ev cdc.ChangeEvent
		var err error
		switch m := msg.(type) {
		case *pglogrepl.RelationMessage:
			rels[m.RelationID] = m
		case *pglogrepl.BeginMessage:
			tx, finalLSN, commitTS = tx[:0], m.FinalLSN, m.CommitTime
		case *pglogrepl.InsertMessage:
			if ev, err = mk("c", m.RelationID, nil, m.Tuple); err != nil {
				return err
			}
			tx = append(tx, ev)
		case *pglogrepl.UpdateMessage:
			if ev, err = mk("u", m.RelationID, m.OldTuple, m.NewTuple); err != nil {
				return err
			}
			tx = append(tx, ev)
		case *pglogrepl.DeleteMessage:
			if ev, err = mk("d", m.RelationID, m.OldTuple, nil); err != nil {
				return err
			}
			tx = append(tx, ev)
		case *pglogrepl.TruncateMessage:
			log.Println("WARNING: TRUNCATE not propagated")
		case *pglogrepl.CommitMessage:
			for _, e := range tx {
				data, err := json.Marshal(e)
				if err != nil {
					return err
				}
				subject := fmt.Sprintf("cdc.%s.%s", e.Schema, e.Table)
				msgID := fmt.Sprintf("%d-%d", e.LSN, e.Seq)
				if _, err := js.Publish(ctx, subject, data, jetstream.WithMsgID(msgID)); err != nil {
					return fmt.Errorf("publish: %w", err)
				}
			}
			flushed = m.TransactionEndLSN // 先发布成功，后推进位点
			if len(tx) > 0 {
				log.Printf("tx %s: %d events published", m.CommitLSN, len(tx))
			}
			tx = tx[:0]
		}
		return nil
	}

	nextStatus := time.Now().Add(5 * time.Second)
	for ctx.Err() == nil {
		if time.Now().After(nextStatus) {
			if err := sendStatus(ctx); err != nil {
				return fmt.Errorf("status update: %w", err)
			}
			nextStatus = time.Now().Add(5 * time.Second)
		}
		rctx, cancel := context.WithDeadline(ctx, nextStatus)
		raw, err := conn.ReceiveMessage(rctx)
		cancel()
		if err != nil {
			if pgconn.Timeout(err) {
				continue
			}
			return fmt.Errorf("receive: %w", err)
		}

		switch msg := raw.(type) {
		case *pgproto3.ErrorResponse:
			return pgconn.ErrorResponseToPgError(msg)
		case *pgproto3.CopyData:
			switch msg.Data[0] {
			case pglogrepl.PrimaryKeepaliveMessageByteID:
				ka, err := pglogrepl.ParsePrimaryKeepaliveMessage(msg.Data[1:])
				if err != nil {
					return err
				}
				if ka.ReplyRequested {
					nextStatus = time.Now()
				}
			case pglogrepl.XLogDataByteID:
				xld, err := pglogrepl.ParseXLogData(msg.Data[1:])
				if err != nil {
					return err
				}
				logical, err := pglogrepl.Parse(xld.WALData)
				if err != nil {
					return err
				}
				if err := handle(logical); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

