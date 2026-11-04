package cdc

import (
	"encoding/json"
	"testing"

	"github.com/jackc/pglogrepl"
)

func TestTupleToMap(t *testing.T) {
	rel := &pglogrepl.RelationMessage{Columns: []*pglogrepl.RelationMessageColumn{
		{Name: "id", DataType: 20},
		{Name: "equip_code", DataType: 25},
		{Name: "coolant_c", DataType: 1700},
		{Name: "fault", DataType: 25},
	}}
	tup := &pglogrepl.TupleData{Columns: []*pglogrepl.TupleDataColumn{
		{DataType: 't', Data: []byte("7")},
		{DataType: 't', Data: []byte("L1")},
		{DataType: 't', Data: []byte("85.00")},
		{DataType: 'n'},
	}}
	b, err := json.Marshal(TupleToMap(rel, tup))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"coolant_c":85.00,"equip_code":"L1","fault":null,"id":7}`
	if string(b) != want {
		t.Fatalf("got %s want %s", b, want)
	}
}

func TestTupleToMapNil(t *testing.T) {
	if TupleToMap(&pglogrepl.RelationMessage{}, nil) != nil {
		t.Fatal("nil tuple should give nil map")
	}
}
