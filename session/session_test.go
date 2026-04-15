package session_test

import (
	"encoding/json"
	"testing"

	"github.com/dpopsuev/battery/session"
)

func TestNew(t *testing.T) {
	s := session.New("sess-1", "claude-4", "/home/user")
	if s.ID != "sess-1" {
		t.Errorf("ID = %q", s.ID)
	}
	if s.Model != "claude-4" {
		t.Errorf("Model = %q", s.Model)
	}
	if s.WorkDir != "/home/user" {
		t.Errorf("WorkDir = %q", s.WorkDir)
	}
	if s.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if s.History == nil {
		t.Error("History should be initialized")
	}
}

func TestSession_Append(t *testing.T) {
	s := session.New("sess-1", "claude-4", "/home/user")
	s.Append(session.SimpleEntry("user", "hello"))
	s.Append(session.SimpleEntry("assistant", "hi"))

	entries := s.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Role != "user" || entries[0].Content != "hello" {
		t.Errorf("entry[0] = %+v", entries[0])
	}
	if entries[1].Role != "assistant" || entries[1].Content != "hi" {
		t.Errorf("entry[1] = %+v", entries[1])
	}
}

func TestSession_AppendAutoTimestamp(t *testing.T) {
	s := session.New("sess-1", "claude-4", "/home/user")
	e := session.Entry{Role: "user", Content: "test"}
	s.Append(e)

	entries := s.Entries()
	if entries[0].Timestamp.IsZero() {
		t.Error("Append should auto-set timestamp when zero")
	}
}

func TestSession_UpdatedAt(t *testing.T) {
	s := session.New("sess-1", "claude-4", "/home/user")
	created := s.UpdatedAt
	s.Append(session.SimpleEntry("user", "hello"))
	if !s.UpdatedAt.After(created) || s.UpdatedAt.Equal(created) {
		t.Error("UpdatedAt should advance after Append")
	}
}

func TestSession_TotalTokens(t *testing.T) {
	s := session.New("sess-1", "claude-4", "/home/user")
	s.Append(session.SimpleEntry("user", "hello world")) // ~3 tokens (11 chars / 4)
	if s.TotalTokens() == 0 {
		t.Error("TotalTokens should be > 0")
	}
}

func TestHistory_TokenBudgetTrimming(t *testing.T) {
	h := session.NewHistory(10) // 10 token budget
	// Add entries that exceed budget.
	h.Append(session.SimpleEntry("user", "aaaa bbbb cccc dddd")) // ~5 tokens
	h.Append(session.SimpleEntry("user", "eeee ffff gggg hhhh")) // ~5 tokens
	h.Append(session.SimpleEntry("user", "iiii jjjj kkkk llll")) // ~5 tokens, triggers trim

	if h.Len() > 2 {
		t.Errorf("expected oldest trimmed, got %d entries", h.Len())
	}
	if h.TotalTokens() > 10 {
		t.Errorf("TotalTokens = %d, should be <= 10", h.TotalTokens())
	}
}

func TestHistory_UnlimitedBudget(t *testing.T) {
	h := session.NewHistory(0) // unlimited
	for range 100 {
		h.Append(session.SimpleEntry("user", "hello world this is a long message"))
	}
	if h.Len() != 100 {
		t.Errorf("expected 100 entries with unlimited budget, got %d", h.Len())
	}
}

func TestHistory_Clear(t *testing.T) {
	h := session.NewHistory(0)
	h.Append(session.SimpleEntry("user", "hello"))
	h.Clear()
	if h.Len() != 0 {
		t.Errorf("Len after Clear = %d", h.Len())
	}
}

func TestHistory_JSONRoundTrip(t *testing.T) {
	h := session.NewHistory(0)
	h.Append(session.SimpleEntry("user", "hello"))
	h.Append(session.SimpleEntry("assistant", "hi"))

	data, err := json.Marshal(h)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	h2 := session.NewHistory(0)
	if err := json.Unmarshal(data, h2); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if h2.Len() != 2 {
		t.Errorf("round-trip Len = %d, want 2", h2.Len())
	}
	entries := h2.Entries()
	if entries[0].Content != "hello" || entries[1].Content != "hi" {
		t.Errorf("round-trip entries = %+v", entries)
	}
}

func TestEntry_TextContent_Simple(t *testing.T) {
	e := session.SimpleEntry("user", "hello world")
	if e.TextContent() != "hello world" {
		t.Errorf("TextContent = %q", e.TextContent())
	}
}

func TestEntry_TextContent_Blocks(t *testing.T) {
	e := session.RichEntry("assistant", []session.ContentBlock{
		{Type: session.BlockThinking, Thinking: "let me think"},
		{Type: session.BlockText, Text: "answer 1"},
		{Type: session.BlockToolUse, Text: ""},
		{Type: session.BlockText, Text: "answer 2"},
	})
	got := e.TextContent()
	if got != "answer 1\nanswer 2" {
		t.Errorf("TextContent = %q, want %q", got, "answer 1\nanswer 2")
	}
}

func TestEntry_TextContent_FallbackToContent(t *testing.T) {
	e := session.Entry{Role: "user", Content: "fallback", Blocks: []session.ContentBlock{
		{Type: session.BlockThinking, Thinking: "no text blocks"},
	}}
	if e.TextContent() != "fallback" {
		t.Errorf("TextContent = %q, want fallback", e.TextContent())
	}
}

func TestRichEntry_SetsContent(t *testing.T) {
	e := session.RichEntry("assistant", []session.ContentBlock{
		{Type: session.BlockText, Text: "hello"},
		{Type: session.BlockText, Text: "world"},
	})
	if e.Content != "hello\nworld" {
		t.Errorf("Content = %q, want joined text blocks", e.Content)
	}
	if len(e.Blocks) != 2 {
		t.Errorf("Blocks = %d", len(e.Blocks))
	}
}

func TestSimpleEntry_Timestamp(t *testing.T) {
	e := session.SimpleEntry("user", "test")
	if e.Timestamp.IsZero() {
		t.Error("SimpleEntry should set Timestamp")
	}
}
