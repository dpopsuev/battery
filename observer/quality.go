package observer

import (
	"sort"
	"time"
)

// QualityMetrics summarizes tool execution quality from Ring data.
// Computed by AnalyzeRing — pure derivation, no side effects.
type QualityMetrics struct {
	ToolErrorRates  map[string]float64            // tool → error count / total calls
	ToolLatencyP50  map[string]time.Duration      // tool → 50th percentile latency
	ToolLatencyP95  map[string]time.Duration      // tool → 95th percentile latency
	CacheHitRatio   float64                       // hits / (hits + misses); -1 if no cache events
	ToolCallCounts  map[string]int                // tool → total call count
	TruncationCount int                           // count of truncation events (future)
	GaugeTotals     map[string]map[string]float64 // tool → metric name → cumulative value
}

// AnalyzeRing computes QualityMetrics from all events in the Ring.
func AnalyzeRing(r *Ring) QualityMetrics {
	events := r.Last(r.Stats().Count)
	return analyzeEvents(events)
}

// AnalyzeEvents computes QualityMetrics from a slice of events.
func AnalyzeEvents(events []Event) QualityMetrics {
	return analyzeEvents(events)
}

type eventAccumulator struct {
	toolLatencies map[string][]time.Duration
	toolErrors    map[string]int
	toolCalls     map[string]int
	cacheHits     int
	cacheMisses   int
	gaugeTotals   map[string]map[string]float64
}

func newAccumulator() *eventAccumulator {
	return &eventAccumulator{
		toolLatencies: make(map[string][]time.Duration),
		toolErrors:    make(map[string]int),
		toolCalls:     make(map[string]int),
		gaugeTotals:   make(map[string]map[string]float64),
	}
}

func (a *eventAccumulator) processCall(e *Event) {
	a.toolCalls[e.Tool]++
}

func (a *eventAccumulator) processCallDone(e *Event) {
	if e.Latency > 0 {
		a.toolLatencies[e.Tool] = append(a.toolLatencies[e.Tool], e.Latency)
	}
	if e.Error {
		a.toolErrors[e.Tool]++
	}
}

func (a *eventAccumulator) processGauge(e *Event) {
	if e.Tool == "" || len(e.Metadata) == 0 {
		return
	}
	if a.gaugeTotals[e.Tool] == nil {
		a.gaugeTotals[e.Tool] = make(map[string]float64)
	}
	for k, v := range e.Metadata {
		a.gaugeTotals[e.Tool][k] += parseLeadingFloat(v)
	}
}

func analyzeEvents(events []Event) QualityMetrics {
	acc := newAccumulator()

	for i := range events {
		e := &events[i]
		switch e.Action {
		case "call":
			acc.processCall(e)
		case "call" + ActionDoneSuffix:
			acc.processCallDone(e)
		case "cache_hit":
			acc.cacheHits++
		case "cache_miss":
			acc.cacheMisses++
		case "gauge":
			acc.processGauge(e)
		}
	}

	return acc.toMetrics()
}

func (a *eventAccumulator) toMetrics() QualityMetrics {
	m := QualityMetrics{
		ToolErrorRates: make(map[string]float64),
		ToolLatencyP50: make(map[string]time.Duration),
		ToolLatencyP95: make(map[string]time.Duration),
		CacheHitRatio:  -1,
		ToolCallCounts: make(map[string]int),
		GaugeTotals:    a.gaugeTotals,
	}

	for tool, calls := range a.toolCalls {
		m.ToolCallCounts[tool] = calls
		if calls > 0 {
			m.ToolErrorRates[tool] = float64(a.toolErrors[tool]) / float64(calls)
		}
	}

	for tool, latencies := range a.toolLatencies {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		m.ToolLatencyP50[tool] = percentile(latencies, 50)
		m.ToolLatencyP95[tool] = percentile(latencies, 95)
	}

	total := a.cacheHits + a.cacheMisses
	if total > 0 {
		m.CacheHitRatio = float64(a.cacheHits) / float64(total)
	}

	return m
}

func percentile(sorted []time.Duration, pct int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := len(sorted) * pct / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func parseLeadingFloat(s string) float64 {
	var result float64
	var decimal float64
	inDecimal := false
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
			if inDecimal {
				decimal /= 10
				result += float64(c-'0') * decimal
			} else {
				result = result*10 + float64(c-'0')
			}
		case c == '.' && !inDecimal:
			inDecimal = true
			decimal = 1
		default:
			return result
		}
	}
	return result
}
