package session

import "encoding/json"

const approximateTokensPerChar = 4

// History manages conversation entries with optional token budget.
type History struct {
	entries   []Entry
	maxTokens int // 0 = unlimited
}

// NewHistory creates a history with an optional token budget.
func NewHistory(maxTokens int) *History {
	return &History{maxTokens: maxTokens}
}

// Append adds an entry. If a token budget is set, trims oldest entries.
func (h *History) Append(e Entry) {
	if e.TokenCount == 0 {
		e.TokenCount = estimateTokens(e.TextContent())
	}
	h.entries = append(h.entries, e)
	h.trimIfNeeded()
}

// Entries returns a copy of all entries.
func (h *History) Entries() []Entry {
	out := make([]Entry, len(h.entries))
	copy(out, h.entries)
	return out
}

// Len returns the number of entries.
func (h *History) Len() int { return len(h.entries) }

// TotalTokens returns the approximate total token count.
func (h *History) TotalTokens() int {
	total := 0
	for _, e := range h.entries {
		total += e.TokenCount
	}
	return total
}

// Clear removes all entries.
func (h *History) Clear() { h.entries = nil }

// SetEntries replaces all entries.
func (h *History) SetEntries(entries []Entry) { h.entries = entries }

// MarshalJSON serializes the history as a JSON array of entries.
func (h *History) MarshalJSON() ([]byte, error) { return json.Marshal(h.entries) }

// UnmarshalJSON deserializes a JSON array of entries into the history.
func (h *History) UnmarshalJSON(data []byte) error { return json.Unmarshal(data, &h.entries) }

func (h *History) trimIfNeeded() {
	if h.maxTokens <= 0 {
		return
	}
	for h.TotalTokens() > h.maxTokens && len(h.entries) > 1 {
		h.entries = h.entries[1:]
	}
}

func estimateTokens(text string) int {
	chars := len(text)
	if chars == 0 {
		return 1
	}
	tokens := chars / approximateTokensPerChar
	if tokens == 0 {
		tokens = 1
	}
	return tokens
}
