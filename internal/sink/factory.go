package sink

import (
	"context"
	"fmt"
	"sort"
)

type Factory func(ctx context.Context, dsn, table string) (Sink, error)

var factories = map[string]Factory{}

func Register(kind string, f Factory) { factories[kind] = f }

func New(ctx context.Context, kind, dsn, table string) (Sink, error) {
	f, ok := factories[kind]
	if !ok {
		var ks []string
		for k := range factories {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		return nil, fmt.Errorf("unknown sink %q (available: %v)", kind, ks)
	}
	return f(ctx, dsn, table)
}

func init() {
	Register("postgres", func(ctx context.Context, dsn, table string) (Sink, error) {
		p, err := NewPostgres(ctx, dsn, table)
		if err != nil {
			return nil, err
		}
		return p, nil
	})
}
