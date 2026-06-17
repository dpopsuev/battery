// Package translate defines the canonical data model for cross-source
// event translation. Sources produce Records; targets consume them.
// Neither source nor target knows about the other — the canonical
// Record is the anti-corruption layer between bounded contexts.
package translate

import "encoding/json"

// Record is the canonical intermediate representation.
// Generic enough for any data source; structured enough for graph storage.
type Record struct {
	ID       string            `json:"id"`
	Kind     string            `json:"kind"`
	Title    string            `json:"title"`
	Labels   []string          `json:"labels,omitempty"`
	Sections []Section         `json:"sections,omitempty"`
	Extra    map[string]any    `json:"extra,omitempty"`
}

// Section is a named text block within a Record.
type Section struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

// Edge is a directed relationship between two Records.
type Edge struct {
	From     string `json:"from"`
	Relation string `json:"relation"`
	To       string `json:"to"`
}

// Result is what a Translator produces from source data.
type Result struct {
	Records []Record `json:"records"`
	Edges   []Edge   `json:"edges,omitempty"`
}

// Translator converts source-specific data into canonical Records.
type Translator func(sourceType string, data json.RawMessage) (Result, error)
