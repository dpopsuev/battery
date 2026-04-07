// Package session manages conversation history and context for multi-turn agent interactions.
package session

import (
	"strings"
	"time"
)

// Content block type constants.
const (
	BlockText       = "text"
	BlockToolUse    = "tool_use"
	BlockToolResult = "tool_result"
	BlockThinking   = "thinking"
)

// ContentBlock is one piece of a message (text, tool call, thinking).
// Consumers using any-llm-go map their provider's block types to this.
type ContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Thinking string `json:"thinking,omitempty"`
}

// Entry represents a single turn in a conversation.
type Entry struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	Blocks     []ContentBlock `json:"blocks,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
	TokenCount int            `json:"token_count,omitempty"`
}

// TextContent returns the text from this entry.
func (e Entry) TextContent() string {
	if len(e.Blocks) == 0 {
		return e.Content
	}
	var parts []string
	for _, b := range e.Blocks {
		if b.Type == BlockText && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	if len(parts) == 0 {
		return e.Content
	}
	return strings.Join(parts, "\n")
}

// SimpleEntry creates an entry with text content only.
func SimpleEntry(role, content string) Entry {
	return Entry{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// RichEntry creates an entry with content blocks.
func RichEntry(role string, blocks []ContentBlock) Entry {
	var parts []string
	for _, b := range blocks {
		if b.Type == BlockText && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return Entry{
		Role:      role,
		Content:   strings.Join(parts, "\n"),
		Blocks:    blocks,
		Timestamp: time.Now(),
	}
}
