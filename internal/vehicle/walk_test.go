package vehicle

import (
	"math/rand"
	"testing"
)

func TestWalkStaysInRange(t *testing.T) {
	tests := []struct {
		name            string
		start, min, max float64
		step            float64
	}{
		{"rpm", 1500, 0, 3000, 100},
		{"coolant", 85, -20, 130, 2},
		{"big step clamps", 50, 0, 100, 500},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := NewWalk(tc.start, tc.min, tc.max, tc.step, rand.New(rand.NewSource(1)))
			for i := 0; i < 10000; i++ {
				v := w.Next()
				if v < tc.min || v > tc.max {
					t.Fatalf("step %d: value %v out of range [%v, %v]", i, v, tc.min, tc.max)
				}
			}
		})
	}
}

func TestWalkDeterministic(t *testing.T) {
	a := NewWalk(50, 0, 100, 5, rand.New(rand.NewSource(42)))
	b := NewWalk(50, 0, 100, 5, rand.New(rand.NewSource(42)))
	for i := 0; i < 100; i++ {
		if va, vb := a.Next(), b.Next(); va != vb {
			t.Fatalf("step %d: %v != %v, same seed should give same sequence", i, va, vb)
		}
	}
}

func TestWalkMovesContinuously(t *testing.T) {
	w := NewWalk(50, 0, 100, 1, rand.New(rand.NewSource(7)))
	prev, lo, hi := 50.0, 50.0, 50.0
	for i := 0; i < 1000; i++ {
		v := w.Next()
		if d := v - prev; d > 1.0001 || d < -1.0001 {
			t.Fatalf("step %d: jump %v exceeds step", i, d)
		}
		prev = v
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	if hi-lo <= 3 {
		t.Fatalf("range %v too small, walk is not drifting", hi-lo)
	}
}
