// Package message defines structured communication types for agent conversations.
package message

import (
	"encoding/json"
	"strings"
)

// Block type constants.
const (
	BlockText       = "text"
	BlockToolUse    = "tool_use"
	BlockToolResult = "tool_result"
	BlockThinking   = "thinking"
)

// Stream event type constants.
const (
	EventText     = "text"
	EventThinking = "thinking"
	EventToolUse  = "tool_use"
	EventDone     = "done"
	EventError    = "error"
)

// Role constants.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// ContentBlock is one piece of a message.
type ContentBlock struct {
	Type       string      `json:"type"`
	Text       string      `json:"text,omitempty"`
	ToolCall   *ToolCall   `json:"tool_call,omitempty"`
	ToolResult *ToolResult `json:"tool_result,omitempty"`
	Thinking   string      `json:"thinking,omitempty"`
}

// ToolCall represents a tool invocation requested by the model.
type ToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolResult represents the output of a tool execution.
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Output     string `json:"output"`
	IsError    bool   `json:"is_error,omitempty"`
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// StreamEvent is a single event from a streaming response.
type StreamEvent struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	Thinking string    `json:"thinking,omitempty"`
	ToolCall *ToolCall `json:"tool_call,omitempty"`
	Usage    *Usage    `json:"usage,omitempty"`
	Error    string    `json:"error,omitempty"`
}

// RichMessage is a structured message with content blocks.
type RichMessage struct {
	Role    string         `json:"role"`
	Content string         `json:"content,omitempty"`
	Blocks  []ContentBlock `json:"blocks,omitempty"`
	Usage   *Usage         `json:"usage,omitempty"`
}

// TextContent returns concatenated text from all text blocks.
func (m RichMessage) TextContent() string {
	if len(m.Blocks) == 0 {
		return m.Content
	}
	var parts []string
	for _, b := range m.Blocks {
		if b.Type == BlockText && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	if len(parts) == 0 {
		return m.Content
	}
	return strings.Join(parts, "\n")
}

// ToolCalls returns all tool call blocks.
func (m RichMessage) ToolCalls() []ToolCall {
	var calls []ToolCall
	for _, b := range m.Blocks {
		if b.Type == BlockToolUse && b.ToolCall != nil {
			calls = append(calls, *b.ToolCall)
		}
	}
	return calls
}

// HasToolCalls returns true if the message contains tool call blocks.
func (m RichMessage) HasToolCalls() bool {
	for _, b := range m.Blocks {
		if b.Type == BlockToolUse && b.ToolCall != nil {
			return true
		}
	}
	return false
}

// ThinkingContent returns concatenated thinking content.
func (m RichMessage) ThinkingContent() string {
	var parts []string
	for _, b := range m.Blocks {
		if b.Type == BlockThinking && b.Thinking != "" {
			parts = append(parts, b.Thinking)
		}
	}
	return strings.Join(parts, "\n")
}

// NewTextBlock creates a text content block.
func NewTextBlock(text string) ContentBlock {
	return ContentBlock{Type: BlockText, Text: text}
}

// NewToolUseBlock creates a tool use content block.
func NewToolUseBlock(id, name string, input json.RawMessage) ContentBlock {
	return ContentBlock{Type: BlockToolUse, ToolCall: &ToolCall{ID: id, Name: name, Input: input}}
}

// NewToolResultBlock creates a tool result content block.
func NewToolResultBlock(toolCallID, output string, isError bool) ContentBlock {
	return ContentBlock{Type: BlockToolResult, ToolResult: &ToolResult{ToolCallID: toolCallID, Output: output, IsError: isError}}
}

// NewThinkingBlock creates a thinking content block.
func NewThinkingBlock(thinking string) ContentBlock {
	return ContentBlock{Type: BlockThinking, Thinking: thinking}
}
