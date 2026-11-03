package vehicle

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/baiheng2012/fleet-telemetry/internal/contract"
)

type profile struct {
	rpmStart, rpmStep   float64
	coolStart, coolStep float64
	hydStart, hydStep   float64
	fuelRate            float64 // 每秒下降的百分比
}

var profiles = map[Type]profile{
	LHD:    {1400, 60, 85, 0.5, 200, 8, 0.004},
	Truck:  {1800, 80, 88, 0.6, 150, 6, 0.006},
	Bolter: {1200, 40, 80, 0.4, 250, 10, 0.003},
}

type Vehicle struct {
	id        string
	vtype     Type
	rpm       *Walk
	coolant   *Walk
	hydraulic *Walk
	fuel      float64
	fuelRate  float64
	hourMeter float64
	faultCode string
	faultLeft int
	faultList []string
	rng       *rand.Rand
}

func New(id string, t Type, seed int64, c *contract.Contract) (*Vehicle, error) {
	p, ok := profiles[t]
	if !ok {
		return nil, fmt.Errorf("unknown vehicle type %q", t)
	}
	rng := rand.New(rand.NewSource(seed))
	mk := func(signal string, start, step float64) (*Walk, error) {
		lo, hi, err := c.Range(signal)
		if err != nil {
			return nil, err
		}
		return NewWalk(start, lo, hi, step, rand.New(rand.NewSource(rng.Int63()))), nil
	}
	rpm, err := mk("engine_rpm", p.rpmStart, p.rpmStep)
	if err != nil {
		return nil, err
	}
	cool, err := mk("coolant_temp", p.coolStart, p.coolStep)
	if err != nil {
		return nil, err
	}
	hyd, err := mk("hydraulic_pressure", p.hydStart, p.hydStep)
	if err != nil {
		return nil, err
	}
	return &Vehicle{
		id: id, vtype: t, rpm: rpm, coolant: cool, hydraulic: hyd,
		fuel: 100, fuelRate: p.fuelRate,
		hourMeter: float64(rng.Intn(5000)), // 每辆车初始小时数不同
		faultList: c.FaultCodes,
		rng:       rng,
	}, nil
}

func (v *Vehicle) Tick(now time.Time) Reading {
	v.hourMeter += 1.0 / 3600
	v.fuel -= v.fuelRate
	if v.fuel < 5 {
		v.fuel = 100 // 加油
	}

	// 故障：小概率触发，持续 5~15 个 tick
	if v.faultLeft > 0 {
		v.faultLeft--
		if v.faultLeft == 0 {
			v.faultCode = ""
		}
	} else if len(v.faultList) > 0 && v.rng.Float64() < 0.002 {
		v.faultCode = v.faultList[v.rng.Intn(len(v.faultList))]
		v.faultLeft = 5 + v.rng.Intn(11)
	}

	rpm, cool, hyd, fuel := v.rpm.Next(), v.coolant.Next(), v.hydraulic.Next(), v.fuel
	return Reading{
		VehicleID: v.id, VehicleType: v.vtype, Timestamp: now,
		EngineRPM: &rpm, CoolantTemp: &cool, HydraulicPressure: &hyd, FuelLevel: &fuel,
		HourMeter: v.hourMeter, FaultCode: v.faultCode,
	}
}
