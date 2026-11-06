package sparkplug

import (
	"testing"
	"time"

	"github.com/baiheng2012/fleet-telemetry/internal/vehicle"
)

func TestRoundTrip(t *testing.T) {
	rpm, hyd, fuel := 1500.5, 200.0, 70.0
	in := vehicle.Reading{
		VehicleID: "LHD-001", VehicleType: vehicle.LHD,
		Timestamp: time.UnixMilli(1760000000123).UTC(),
		EngineRPM: &rpm, HydraulicPressure: &hyd, FuelLevel: &fuel, // CoolantTemp 缺失
		HourMeter: 4699.25, FaultCode: "E101",
	}
	p, err := Decode(Encode(FromReading(in, 7)))
	if err != nil {
		t.Fatal(err)
	}
	if p.Seq != 7 {
		t.Fatalf("seq %d", p.Seq)
	}
	out, err := ToReading("LHD-001", p)
	if err != nil {
		t.Fatal(err)
	}
	if out.CoolantTemp != nil {
		t.Fatal("missing signal must stay missing")
	}
	if *out.EngineRPM != rpm || *out.HydraulicPressure != hyd || *out.FuelLevel != fuel {
		t.Fatalf("signals changed: %+v", out)
	}
	if out.HourMeter != 4699.25 || out.FaultCode != "E101" || out.VehicleType != vehicle.LHD {
		t.Fatalf("fields changed: %+v", out)
	}
	if !out.Timestamp.Equal(in.Timestamp) {
		t.Fatalf("ts %v != %v", out.Timestamp, in.Timestamp)
	}
}

func TestParseTopic(t *testing.T) {
	g, n, d, err := ParseTopic(Topic("fleet", "DDATA", "mine1", "LHD-001"))
	if err != nil || g != "fleet" || n != "mine1" || d != "LHD-001" {
		t.Fatalf("%v %v %v %v", g, n, d, err)
	}
	if _, _, _, err := ParseTopic("spBv1.0/fleet/NBIRTH/mine1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestDecodeGarbage(t *testing.T) {
	if _, err := Decode([]byte{0xff}); err == nil {
		t.Fatal("truncated input must error")
	}
}
