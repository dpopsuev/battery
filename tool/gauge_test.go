package tool_test

import (
	"testing"

	"github.com/dpopsuev/battery/tool"
)

// GaugedContract validates any Gauged implementation.
func GaugedContract(t *testing.T, newGauged func(measurements []tool.Measurement) tool.Gauged) {
	t.Helper()

	t.Run("ReturnsMeasurements", func(t *testing.T) {
		ms := []tool.Measurement{
			{Name: "tokens_in", Value: 42, Unit: "tokens"},
			{Name: "bytes_read", Value: 1024, Unit: "bytes"},
		}
		g := newGauged(ms)
		got := g.LastMeasurement()
		if len(got) != 2 {
			t.Fatalf("got %d measurements, want 2", len(got))
		}
		if got[0].Name != "tokens_in" || got[0].Value != 42 {
			t.Errorf("measurement[0] = %+v", got[0])
		}
		if got[1].Name != "bytes_read" || got[1].Value != 1024 {
			t.Errorf("measurement[1] = %+v", got[1])
		}
	})

	t.Run("EmptyMeasurements", func(t *testing.T) {
		g := newGauged(nil)
		got := g.LastMeasurement()
		if len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})
}

// Verify Gauged is an optional interface on Tool (type assertion check).
func TestGauged_OptionalOnTool(t *testing.T) {
	var plainTool tool.Tool = stubPlainTool{}
	if _, ok := plainTool.(tool.Gauged); ok {
		t.Error("plain Tool should not implement Gauged")
	}
}
