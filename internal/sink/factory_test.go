package sink

import (
	"context"
	"strings"
	"testing"
)

func TestMergeSQL(t *testing.T) {
	q := MergeSQL("raw.telemetry_events", 3)
	if strings.Count(q, "?") != 12 {
		t.Fatalf("want 12 placeholders, got %d", strings.Count(q, "?"))
	}
	if !strings.Contains(q, "MERGE INTO raw.telemetry_events") || !strings.Contains(q, "WHEN NOT MATCHED") {
		t.Fatal("unexpected SQL")
	}
}

func TestUnknownSink(t *testing.T) {
	if _, err := New(context.Background(), "nope", "", ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestPostgresRegistered(t *testing.T) {
	if _, ok := factories["postgres"]; !ok {
		t.Fatal("postgres sink should be registered")
	}
}
