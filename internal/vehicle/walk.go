package vehicle

import "math/rand"

// Walk 是一个有边界的随机游走信号源。
type Walk struct {
	value, min, max, step float64
	rng                   *rand.Rand
}

func NewWalk(start, min, max, step float64, rng *rand.Rand) *Walk {
	return &Walk{value: start, min: min, max: max, step: step, rng: rng}
}

// Next 产生下一个值：上一个值 ± 最多 step，并夹在 [min, max] 内。
func (w *Walk) Next() float64 {
	w.value += (w.rng.Float64()*2 - 1) * w.step
	if w.value < w.min {
		w.value = w.min
	}
	if w.value > w.max {
		w.value = w.max
	}
	return w.value
}
