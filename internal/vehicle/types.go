package vehicle

import "time"

type Type string

const (
	LHD    Type = "LHD"
	Truck  Type = "Truck"
	Bolter Type = "Bolter"
)

type Reading struct {
	VehicleID         string    `json:"vehicle_id"`
	VehicleType       Type      `json:"vehicle_type"`
	Timestamp         time.Time `json:"ts"`
	EngineRPM         *float64  `json:"engine_rpm,omitempty"`
	CoolantTemp       *float64  `json:"coolant_temp,omitempty"`
	HydraulicPressure *float64  `json:"hydraulic_pressure,omitempty"`
	FuelLevel         *float64  `json:"fuel_level,omitempty"`
	HourMeter         float64   `json:"hour_meter"`
	FaultCode         string    `json:"fault_code,omitempty"`
}
