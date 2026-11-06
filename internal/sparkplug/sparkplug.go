package sparkplug

import (
	"fmt"
	"math"
	"strings"

	"google.golang.org/protobuf/encoding/protowire"
)

// Sparkplug B 数据类型编号
const (
	TypeDouble = 10
	TypeString = 12
)

type Metric struct {
	Name      string
	Timestamp uint64 // 毫秒
	Type      uint32
	Num       float64
	Str       string
}

type Payload struct {
	Timestamp uint64 // 毫秒
	Metrics   []Metric
	Seq       uint64
}

// Topic: spBv1.0/{group}/{DDATA}/{edge_node}/{device}
func Topic(group, msgType, node, device string) string {
	return fmt.Sprintf("spBv1.0/%s/%s/%s/%s", group, msgType, node, device)
}

func ParseTopic(t string) (group, node, device string, err error) {
	p := strings.Split(t, "/")
	if len(p) != 5 || p[0] != "spBv1.0" || p[2] != "DDATA" {
		return "", "", "", fmt.Errorf("not a DDATA topic: %q", t)
	}
	return p[1], p[3], p[4], nil
}

func encodeMetric(m Metric) []byte {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.BytesType)
	b = protowire.AppendString(b, m.Name)
	b = protowire.AppendTag(b, 3, protowire.VarintType)
	b = protowire.AppendVarint(b, m.Timestamp)
	b = protowire.AppendTag(b, 4, protowire.VarintType)
	b = protowire.AppendVarint(b, uint64(m.Type))
	switch m.Type {
	case TypeDouble:
		b = protowire.AppendTag(b, 13, protowire.Fixed64Type)
		b = protowire.AppendFixed64(b, math.Float64bits(m.Num))
	case TypeString:
		b = protowire.AppendTag(b, 15, protowire.BytesType)
		b = protowire.AppendString(b, m.Str)
	}
	return b
}

func Encode(p Payload) []byte {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.VarintType)
	b = protowire.AppendVarint(b, p.Timestamp)
	for _, m := range p.Metrics {
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendBytes(b, encodeMetric(m))
	}
	b = protowire.AppendTag(b, 3, protowire.VarintType)
	b = protowire.AppendVarint(b, p.Seq)
	return b
}

func decodeMetric(b []byte) (Metric, error) {
	var m Metric
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return m, protowire.ParseError(n)
		}
		b = b[n:]
		var used int
		switch {
		case num == 1 && typ == protowire.BytesType:
			m.Name, used = protowire.ConsumeString(b)
		case num == 3 && typ == protowire.VarintType:
			m.Timestamp, used = protowire.ConsumeVarint(b)
		case num == 4 && typ == protowire.VarintType:
			var v uint64
			v, used = protowire.ConsumeVarint(b)
			m.Type = uint32(v)
		case (num == 10 || num == 11) && typ == protowire.VarintType: // int / long
			var v uint64
			v, used = protowire.ConsumeVarint(b)
			m.Num = float64(v)
		case num == 12 && typ == protowire.Fixed32Type: // float
			var v uint32
			v, used = protowire.ConsumeFixed32(b)
			m.Num = float64(math.Float32frombits(v))
		case num == 13 && typ == protowire.Fixed64Type: // double
			var v uint64
			v, used = protowire.ConsumeFixed64(b)
			m.Num = math.Float64frombits(v)
		case num == 15 && typ == protowire.BytesType:
			m.Str, used = protowire.ConsumeString(b)
		default:
			used = protowire.ConsumeFieldValue(num, typ, b)
		}
		if used < 0 {
			return m, protowire.ParseError(used)
		}
		b = b[used:]
	}
	return m, nil
}

func Decode(b []byte) (Payload, error) {
	var p Payload
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return p, protowire.ParseError(n)
		}
		b = b[n:]
		var used int
		switch {
		case num == 1 && typ == protowire.VarintType:
			p.Timestamp, used = protowire.ConsumeVarint(b)
		case num == 2 && typ == protowire.BytesType:
			var raw []byte
			raw, used = protowire.ConsumeBytes(b)
			if used >= 0 {
				m, err := decodeMetric(raw)
				if err != nil {
					return p, err
				}
				p.Metrics = append(p.Metrics, m)
			}
		case num == 3 && typ == protowire.VarintType:
			p.Seq, used = protowire.ConsumeVarint(b)
		default:
			used = protowire.ConsumeFieldValue(num, typ, b)
		}
		if used < 0 {
			return p, protowire.ParseError(used)
		}
		b = b[used:]
	}
	return p, nil
}
