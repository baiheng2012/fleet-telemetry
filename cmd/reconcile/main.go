package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/baiheng2012/fleet-telemetry/internal/reconcile"
)

type config struct {
	legacy, warehouse, srcTable, dstTable string
	cutoff, wait                          time.Duration
	retries, maxParts, maxRows            int
	record                                bool
}

func main() {
	var c config
	flag.StringVar(&c.legacy, "legacy", "postgres://postgres:dev@localhost:5434/legacy", "source DSN")
	flag.StringVar(&c.warehouse, "warehouse", "postgres://postgres:dev@localhost:5433/warehouse", "target DSN")
	flag.StringVar(&c.srcTable, "src-table", "legacy.telemetry_log", "source table")
	flag.StringVar(&c.dstTable, "dst-table", "curated.legacy_telemetry", "target table")
	flag.DurationVar(&c.cutoff, "cutoff", 30*time.Second, "ignore rows newer than this (still in flight)")
	flag.IntVar(&c.retries, "retries", 3, "re-check this many times while waiting for convergence")
	flag.DurationVar(&c.wait, "wait", 10*time.Second, "wait between re-checks")
	flag.IntVar(&c.maxParts, "max-parts", 5, "drill down into at most N differing partitions")
	flag.IntVar(&c.maxRows, "max-rows", 20, "print at most N row ids per category")
	flag.BoolVar(&c.record, "record", false, "write the result into dq.check_results")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, c))
}

func run(ctx context.Context, c config) int {
	src, err := pgxpool.New(ctx, c.legacy)
	if err != nil {
		log.Println(err)
		return 2
	}
	defer src.Close()
	dst, err := pgxpool.New(ctx, c.warehouse)
	if err != nil {
		log.Println(err)
		return 2
	}
	defer dst.Close()

	var sp, dp map[reconcile.PartKey]reconcile.PartStat
	var diffs []reconcile.PartDiff
	for attempt := 0; ; attempt++ {
		if sp, err = fetchParts(ctx, src, reconcile.PartSQL(c.srcTable, "id"), c.cutoff); err != nil {
			log.Println("source:", err)
			return 2
		}
		if dp, err = fetchParts(ctx, dst, reconcile.PartSQL(c.dstTable, "legacy_id"), c.cutoff); err != nil {
			log.Println("target:", err)
			return 2
		}
		diffs = reconcile.DiffParts(sp, dp)
		if len(diffs) == 0 || attempt >= c.retries {
			break
		}
		log.Printf("check %d: %d partitions differ, waiting %s for convergence", attempt+1, len(diffs), c.wait)
		select {
		case <-time.After(c.wait):
		case <-ctx.Done():
			return 2
		}
	}

	srcN, dstN := total(sp), total(dp)
	fmt.Printf("source rows: %d   target rows: %d   partitions: %d vs %d\n", srcN, dstN, len(sp), len(dp))

	status := "pass"
	if len(diffs) == 0 {
		fmt.Println("OK: all partitions match (count, sum, row-hash)")
	} else {
		status = "fail"
		fmt.Printf("MISMATCH: %d partitions differ\n", len(diffs))
		for i, d := range diffs {
			fmt.Printf("  %-22s %-18s src(n=%d) dst(n=%d)\n", d.Key, d.Kind, d.Src.N, d.Dst.N)
			if i < c.maxParts {
				if err := drill(ctx, src, dst, c, d.Key); err != nil {
					log.Println("drill-down:", err)
				}
			}
		}
	}

	if c.record {
		details, _ := json.Marshal(map[string]any{"src_rows": srcN, "dst_rows": dstN, "differing_partitions": len(diffs)})
		_, err := dst.Exec(ctx, `insert into dq.check_results
			(run_id, check_name, scope, value, warn_threshold, fail_threshold, status, details)
			values (gen_random_uuid(), 'migration_reconcile', $1, $2, 0, 0, $3, $4::jsonb)`,
			c.srcTable, float64(len(diffs)), status, string(details))
		if err != nil {
			log.Println("record:", err)
			return 2
		}
	}
	if status == "pass" {
		return 0
	}
	return 1
}

func fetchParts(ctx context.Context, pool *pgxpool.Pool, sql string, cutoff time.Duration) (map[reconcile.PartKey]reconcile.PartStat, error) {
	rows, err := pool.Query(ctx, sql, cutoff.Seconds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[reconcile.PartKey]reconcile.PartStat{}
	for rows.Next() {
		var k reconcile.PartKey
		var s reconcile.PartStat
		if err := rows.Scan(&k.Equip, &k.Day, &s.N, &s.SumPSI, &s.Hash); err != nil {
			return nil, err
		}
		out[k] = s
	}
	return out, rows.Err()
}

func fetchRows(ctx context.Context, pool *pgxpool.Pool, sql string, c config, k reconcile.PartKey) (map[int64]string, error) {
	rows, err := pool.Query(ctx, sql, c.cutoff.Seconds(), k.Equip, k.Day)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var h string
		if err := rows.Scan(&id, &h); err != nil {
			return nil, err
		}
		out[id] = h
	}
	return out, rows.Err()
}

func drill(ctx context.Context, src, dst *pgxpool.Pool, c config, k reconcile.PartKey) error {
	s, err := fetchRows(ctx, src, reconcile.RowSQL(c.srcTable, "id"), c, k)
	if err != nil {
		return err
	}
	d, err := fetchRows(ctx, dst, reconcile.RowSQL(c.dstTable, "legacy_id"), c, k)
	if err != nil {
		return err
	}
	r := reconcile.DiffRows(s, d)
	show := func(label string, ids []int64) {
		if len(ids) == 0 {
			return
		}
		if len(ids) > c.maxRows {
			fmt.Printf("      %s (%d): %v ...\n", label, len(ids), ids[:c.maxRows])
			return
		}
		fmt.Printf("      %s (%d): %v\n", label, len(ids), ids)
	}
	show("missing in target", r.Missing)
	show("extra in target  ", r.Extra)
	show("changed          ", r.Changed)
	return nil
}

func total(m map[reconcile.PartKey]reconcile.PartStat) (n int64) {
	for _, s := range m {
		n += s.N
	}
	return
}
