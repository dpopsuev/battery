package observer

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"
)

// Archive is a serializable snapshot of a ring buffer.
type Archive struct {
	SessionID string  `json:"session_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Filter    string    `json:"filter,omitempty"`
	Events    []Event   `json:"events"`
}

// Export creates an archive from the ring buffer.
func Export(r *Ring, component Component) *Archive {
	var events []Event
	if component != "" {
		events = r.ByComponent(component)
	} else {
		events = r.Last(r.Stats().Count)
	}
	return &Archive{
		CreatedAt: time.Now(),
		Filter:    string(component),
		Events:    events,
	}
}

// Import loads events from an archive into a ring buffer.
func Import(a *Archive, r *Ring) {
	for i := range a.Events {
		r.Append(a.Events[i])
	}
}

// SaveJSON writes an archive to a JSON file.
func (a *Archive) SaveJSON(path string) error {
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return fmt.Errorf("battery: marshal archive: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

// LoadArchive reads an archive from a JSON file.
func LoadArchive(path string) (*Archive, error) {
	data, err := os.ReadFile(path) //nolint:gosec // trusted path
	if err != nil {
		return nil, fmt.Errorf("battery: read archive: %w", err)
	}
	var a Archive
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("battery: unmarshal archive: %w", err)
	}
	return &a, nil
}

// DiffResult summarizes differences between two archives.
type DiffResult struct {
	EventCountBefore int            `json:"event_count_before"`
	EventCountAfter  int            `json:"event_count_after"`
	ErrorRateBefore  float64        `json:"error_rate_before"`
	ErrorRateAfter   float64        `json:"error_rate_after"`
	LatencyDeltas    []LatencyDelta `json:"latency_deltas,omitempty"`
	NewErrors        []Event        `json:"new_errors,omitempty"`
	ResolvedErrors   []Event        `json:"resolved_errors,omitempty"`
}

// LatencyDelta shows latency change for a server+tool pair.
type LatencyDelta struct {
	Server    string        `json:"server"`
	Tool      string        `json:"tool"`
	BeforeP50 time.Duration `json:"before_p50"`
	AfterP50  time.Duration `json:"after_p50"`
	Change    float64       `json:"change_pct"`
}

// Diff compares two archives.
func Diff(before, after *Archive) *DiffResult {
	result := &DiffResult{
		EventCountBefore: len(before.Events),
		EventCountAfter:  len(after.Events),
		ErrorRateBefore:  errorRate(before.Events),
		ErrorRateAfter:   errorRate(after.Events),
	}

	beforeLatencies := groupLatencies(before.Events)
	afterLatencies := groupLatencies(after.Events)

	for key, beforeVals := range beforeLatencies {
		afterVals := afterLatencies[key]
		if len(afterVals) == 0 {
			continue
		}
		bp50 := percentileDuration(beforeVals, 50) //nolint:mnd
		ap50 := percentileDuration(afterVals, 50)  //nolint:mnd
		change := 0.0
		if bp50 > 0 {
			change = float64(ap50-bp50) / float64(bp50) * 100
		}
		server, toolName := splitKey(key)
		result.LatencyDeltas = append(result.LatencyDeltas, LatencyDelta{
			Server: server, Tool: toolName,
			BeforeP50: bp50, AfterP50: ap50,
			Change: math.Round(change*10) / 10, //nolint:mnd
		})
	}

	beforeErrors := errorSet(before.Events)
	afterErrors := errorSet(after.Events)
	for i := range after.Events {
		e := &after.Events[i]
		if e.Error && !beforeErrors[e.Server+"."+e.Tool] {
			result.NewErrors = append(result.NewErrors, *e)
		}
	}
	for i := range before.Events {
		e := &before.Events[i]
		if e.Error && !afterErrors[e.Server+"."+e.Tool] {
			result.ResolvedErrors = append(result.ResolvedErrors, *e)
		}
	}

	return result
}

func errorRate(events []Event) float64 {
	if len(events) == 0 {
		return 0
	}
	errs := 0
	for i := range events {
		if events[i].Error {
			errs++
		}
	}
	return float64(errs) / float64(len(events))
}

func groupLatencies(events []Event) map[string][]time.Duration {
	m := make(map[string][]time.Duration)
	for i := range events {
		if events[i].Latency > 0 {
			key := events[i].Server + "|" + events[i].Tool
			m[key] = append(m[key], events[i].Latency)
		}
	}
	return m
}

func splitKey(key string) (server, toolName string) {
	for i, c := range key {
		if c == '|' {
			return key[:i], key[i+1:]
		}
	}
	return key, ""
}

func errorSet(events []Event) map[string]bool {
	m := make(map[string]bool)
	for i := range events {
		if events[i].Error {
			m[events[i].Server+"."+events[i].Tool] = true
		}
	}
	return m
}

func percentileDuration(vals []time.Duration, pct int) time.Duration {
	if len(vals) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(vals))
	copy(sorted, vals)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	idx := (pct * len(sorted)) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
