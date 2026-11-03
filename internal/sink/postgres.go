package sink

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pg pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pg ping: %w", err)
	}
	return &Postgres{pool: pool}, nil
}

const insertSQL = `
INSERT INTO raw.telemetry_events (stream_seq, subject, published_at, payload)
SELECT s, subj, t, p::jsonb
FROM unnest($1::bigint[], $2::text[], $3::timestamptz[], $4::text[]) AS u(s, subj, t, p)
ON CONFLICT (stream_seq) DO NOTHING`

func (p *Postgres) WriteBatch(ctx context.Context, events []Event) (int64, error) {
	if len(events) == 0 {
		return 0, nil
	}
	seqs := make([]int64, len(events))
	subjects := make([]string, len(events))
	times := make([]time.Time, len(events))
	payloads := make([]string, len(events))
	for i, e := range events {
		seqs[i], subjects[i], times[i], payloads[i] = e.StreamSeq, e.Subject, e.PublishedAt, string(e.Payload)
	}
	tag, err := p.pool.Exec(ctx, insertSQL, seqs, subjects, times, payloads)
	if err != nil {
		return 0, fmt.Errorf("insert batch: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (p *Postgres) Close() { p.pool.Close() }
