// Package session manages conversation history and context for multi-turn agent interactions.
package session

import (
	"strings"
	"time"

	"github.com/dpopsuev/battery/message"
)

// Entry represents a single turn in a conversation.
type Entry struct {
	Role       string                 `json:"role"`
	Content    string                 `json:"content,omitempty"`
	Blocks     []message.ContentBlock `json:"blocks,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
	TokenCount int                    `json:"token_count,omitempty"`
}

// TextContent returns the text from this entry.
func (e Entry) TextContent() string {
	if len(e.Blocks) == 0 {
		return e.Content
	}
	rm := message.RichMessage{Content: e.Content, Blocks: e.Blocks}
	return rm.TextContent()
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
func RichEntry(role string, blocks []message.ContentBlock) Entry {
	var parts []string
	for _, b := range blocks {
		if b.Type == message.BlockText && b.Text != "" {
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
