package reconcile

import "sort"

type PartKey struct{ Equip, Day string }

func (k PartKey) String() string { return k.Equip + "/" + k.Day }

// PartStat 是一个分区（设备×天）的摘要。
type PartStat struct {
	N      int64
	SumPSI string
	Hash   string
}

type PartDiff struct {
	Key      PartKey
	Kind     string // missing_in_target | extra_in_target | mismatch
	Src, Dst PartStat
}

func DiffParts(src, dst map[PartKey]PartStat) []PartDiff {
	var out []PartDiff
	for k, s := range src {
		d, ok := dst[k]
		switch {
		case !ok:
			out = append(out, PartDiff{Key: k, Kind: "missing_in_target", Src: s})
		case s != d:
			out = append(out, PartDiff{Key: k, Kind: "mismatch", Src: s, Dst: d})
		}
	}
	for k, d := range dst {
		if _, ok := src[k]; !ok {
			out = append(out, PartDiff{Key: k, Kind: "extra_in_target", Dst: d})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key.String() < out[j].Key.String() })
	return out
}

// RowDiff 是行级差异：id 列表。
type RowDiff struct{ Missing, Extra, Changed []int64 }

func DiffRows(src, dst map[int64]string) RowDiff {
	var r RowDiff
	for id, h := range src {
		dh, ok := dst[id]
		switch {
		case !ok:
			r.Missing = append(r.Missing, id)
		case dh != h:
			r.Changed = append(r.Changed, id)
		}
	}
	for id := range dst {
		if _, ok := src[id]; !ok {
			r.Extra = append(r.Extra, id)
		}
	}
	for _, s := range [][]int64{r.Missing, r.Extra, r.Changed} {
		sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	}
	return r
}
