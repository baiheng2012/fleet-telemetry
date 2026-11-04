package cdc

import (
	"encoding/json"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pglogrepl"
)

// ChangeEvent 是发布到 JetStream 的一条行级变更。
type ChangeEvent struct {
	Op       string         `json:"op"` // c=insert u=update d=delete
	Schema   string         `json:"schema"`
	Table    string         `json:"table"`
	LSN      int64          `json:"lsn"` // 事务提交位点，全序
	Seq      int            `json:"seq"` // 事务内序号
	CommitTS time.Time      `json:"commit_ts"`
	Before   map[string]any `json:"before,omitempty"`
	After    map[string]any `json:"after,omitempty"`
}

// TupleToMap 把一行元组按列名转成 map。
func TupleToMap(rel *pglogrepl.RelationMessage, t *pglogrepl.TupleData) map[string]any {
	if t == nil {
		return nil
	}
	m := make(map[string]any, len(t.Columns))
	for i, c := range t.Columns {
		if i >= len(rel.Columns) {
			break
		}
		col := rel.Columns[i]
		switch c.DataType {
		case pglogrepl.TupleDataTypeNull:
			m[col.Name] = nil
		case pglogrepl.TupleDataTypeText:
			m[col.Name] = convert(col.DataType, string(c.Data))
		}
		// 'u'：未变化的 TOAST 列，本表没有，忽略
	}
	return m
}

// 数值类型保持为 JSON 数字（json.Number 不丢精度），其余一律字符串。
func convert(oid uint32, s string) any {
	switch oid {
	case 20, 21, 23, 700, 701, 1700: // int8 int2 int4 float4 float8 numeric
		if f, err := strconv.ParseFloat(s, 64); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
			return json.Number(s)
		}
	}
	return s
}
