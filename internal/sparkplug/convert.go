package sparkplug

import (
	"fmt"
	"time"

	"github.com/baiheng2012/fleet-telemetry/internal/vehicle"
)

func FromReading(r vehicle.Reading, seq uint64) Payload {
	ts := uint64(r.Timestamp.UnixMilli())
	p := Payload{Timestamp: ts, Seq: seq}
	num := func(name string, v float64) {
		p.Metrics = append(p.Metrics, Metric{Name: name, Timestamp: ts, Type: TypeDouble, Num: v})
	}
	opt := func(name string, v *float64) {
		if v != nil {
			num(name, *v)
		}
	}
	opt("engine_rpm", r.EngineRPM)
	opt("coolant_temp", r.CoolantTemp)
	opt("hydraulic_pressure", r.HydraulicPressure)
	opt("fuel_level", r.FuelLevel)
	num("hour_meter", r.HourMeter)
	p.Metrics = append(p.Metrics, Metric{Name: "vehicle_type", Timestamp: ts, Type: TypeString, Str: string(r.VehicleType)})
	if r.FaultCode != "" {
		p.Metrics = append(p.Metrics, Metric{Name: "fault_code", Timestamp: ts, Type: TypeString, Str: r.FaultCode})
	}
	return p
}

func ToReading(device string, p Payload) (vehicle.Reading, error) {
	if p.Timestamp == 0 {
		return vehicle.Reading{}, fmt.Errorf("payload without timestamp")
	}
	r := vehicle.Reading{VehicleID: device, Timestamp: time.UnixMilli(int64(p.Timestamp)).UTC()}
	ptr := func(v float64) *float64 { return &v }
	for _, m := range p.Metrics {
		switch m.Name {
		case "engine_rpm":
			r.EngineRPM = ptr(m.Num)
		case "coolant_temp":
			r.CoolantTemp = ptr(m.Num)
		case "hydraulic_pressure":
			r.HydraulicPressure = ptr(m.Num)
		case "fuel_level":
			r.FuelLevel = ptr(m.Num)
		case "hour_meter":
			r.HourMeter = m.Num
		case "vehicle_type":
			r.VehicleType = vehicle.Type(m.Str)
		case "fault_code":
			r.FaultCode = m.Str
		}
	}
	if r.VehicleType == "" {
		return r, fmt.Errorf("device %s: no vehicle_type metric", device)
	}
	return r, nil
}
