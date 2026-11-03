package vehicle

import "math/rand"

type Chaos struct {
	rng        *rand.Rand
	DropMsg    float64 // 整条消息丢失
	DupMsg     float64 // 消息重复
	Delay      float64 // 消息延迟，到下一条之后才发出（乱序）
	NullSignal float64 // 某个信号缺失
	held       *Reading
}

func NewChaos(seed int64, drop, dup, delay, null float64) *Chaos {
	return &Chaos{rng: rand.New(rand.NewSource(seed)), DropMsg: drop, DupMsg: dup, Delay: delay, NullSignal: null}
}

// Apply 输入一条正常消息，返回 0~N 条要真正发出的消息。
func (c *Chaos) Apply(r Reading) []Reading {
	if c.rng.Float64() < c.DropMsg {
		return nil
	}
	if c.rng.Float64() < c.NullSignal {
		r.CoolantTemp = nil
	}
	var out []Reading
	switch {
	case c.held != nil: // 先发新的，再发之前扣下的旧消息 => 乱序
		out = append(out, r, *c.held)
		c.held = nil
	case c.rng.Float64() < c.Delay:
		c.held = &r
		return nil
	default:
		out = append(out, r)
	}
	if c.rng.Float64() < c.DupMsg {
		out = append(out, r)
	}
	return out
}
