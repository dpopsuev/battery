package tool

// Measurement represents a single metric reported by a tool after execution.
type Measurement struct {
	Name  string  `json:"name"` // e.g. "tokens_in", "bytes_read", "api_calls"
	Value float64 `json:"value"`
	Unit  string  `json:"unit"` // e.g. "tokens", "bytes", "count"
}

// Gauged is an optional interface for tools that report execution measurements.
// Called after Execute() — the tool records metrics during execution and
// reports them via LastMeasurement().
//
//	if g, ok := t.(Gauged); ok {
//	    measurements := g.LastMeasurement()
//	}
type Gauged interface {
	LastMeasurement() []Measurement
}
