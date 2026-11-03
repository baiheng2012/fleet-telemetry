package contract

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Signal struct {
	Unit      string     `yaml:"unit"`
	Type      string     `yaml:"type"`
	Range     [2]float64 `yaml:"range"`
	AppliesTo []string   `yaml:"applies_to"`
}

type Contract struct {
	Version    int               `yaml:"version"`
	Signals    map[string]Signal `yaml:"signals"`
	FaultCodes []string          `yaml:"fault_codes"`
}

func Load(path string) (*Contract, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read contract: %w", err)
	}
	var c Contract
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse contract: %w", err)
	}
	return &c, nil
}

// Range 返回信号的 [min, max]。
func (c *Contract) Range(name string) (float64, float64, error) {
	s, ok := c.Signals[name]
	if !ok {
		return 0, 0, fmt.Errorf("signal %q not in contract", name)
	}
	return s.Range[0], s.Range[1], nil
}
