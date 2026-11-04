package reconcile

import (
	"reflect"
	"testing"
)

func TestDiffParts(t *testing.T) {
	a, b, c := PartKey{"L1", "2026-10-01"}, PartKey{"L2", "2026-10-01"}, PartKey{"T1", "2026-10-01"}
	src := map[PartKey]PartStat{a: {10, "1", "h1"}, b: {5, "2", "h2"}}
	dst := map[PartKey]PartStat{a: {10, "1", "h1"}, b: {5, "2", "XX"}, c: {1, "0", "h3"}}

	got := map[string]string{}
	for _, d := range DiffParts(src, dst) {
		got[d.Key.String()] = d.Kind
	}
	want := map[string]string{"L2/2026-10-01": "mismatch", "T1/2026-10-01": "extra_in_target"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if len(DiffParts(src, src)) != 0 {
		t.Fatal("identical inputs must produce no diff")
	}
}

func TestDiffRows(t *testing.T) {
	src := map[int64]string{1: "a", 2: "b", 3: "c"}
	dst := map[int64]string{1: "a", 2: "X", 4: "d"}
	r := DiffRows(src, dst)
	if !reflect.DeepEqual(r.Missing, []int64{3}) ||
		!reflect.DeepEqual(r.Changed, []int64{2}) ||
		!reflect.DeepEqual(r.Extra, []int64{4}) {
		t.Fatalf("unexpected: %+v", r)
	}
}
