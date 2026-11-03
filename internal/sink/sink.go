package sink

import (
	"context"
	"time"
)

// Event 是写入 RAW 层的一条原始记录。
type Event struct {
	StreamSeq   int64
	Subject     string
	PublishedAt time.Time
	Payload     []byte
}

type Sink interface {
	// WriteBatch 必须幂等：同一批事件重复写入，结果不变。
	WriteBatch(ctx context.Context, events []Event) (inserted int64, err error)
	Close()
}
