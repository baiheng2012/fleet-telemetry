package sink

import (
	"fmt"
	"strings"
)

// MergeSQL 生成 n 行的幂等 MERGE。参数顺序：(stream_seq, subject, published_at, payload) × n。
func MergeSQL(table string, n int) string {
	vals := make([]string, n)
	for i := range vals {
		vals[i] = "(?, ?, ?, ?)"
	}
	return fmt.Sprintf(`MERGE INTO %s t
USING (SELECT column1 AS stream_seq, column2 AS subject,
              to_timestamp_tz(column3) AS published_at, parse_json(column4) AS payload
       FROM VALUES %s) s
ON t.stream_seq = s.stream_seq
WHEN NOT MATCHED THEN INSERT (stream_seq, subject, published_at, payload)
VALUES (s.stream_seq, s.subject, s.published_at, s.payload)`, table, strings.Join(vals, ", "))
}
