package bus

import (
	"testing"

	"github.com/baiheng2012/fleet-telemetry/internal/vehicle"
)

func TestSubject(t *testing.T) {
	r := vehicle.Reading{VehicleID: "LHD-001", VehicleType: vehicle.LHD}
	if got, want := Subject(r), "telemetry.LHD.LHD-001"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
