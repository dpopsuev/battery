package tool_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/dpopsuev/battery/tool"
)

func TestResult_Text(t *testing.T) {
	r := tool.Result{
		Content: []tool.Content{
			tool.TextContent{Text: "line 1"},
			tool.ImageContent{MIMEType: "image/png", Data: []byte("img")},
			tool.TextContent{Text: "line 2"},
		},
	}
	got := r.Text()
	if got != "line 1\nline 2" {
		t.Errorf("Text() = %q, want %q", got, "line 1\nline 2")
	}
}

func TestResult_TextEmpty(t *testing.T) {
	r := tool.Result{
		Content: []tool.Content{
			tool.ImageContent{MIMEType: "image/png", Data: []byte("img")},
		},
	}
	if r.Text() != "" {
		t.Errorf("Text() = %q, want empty", r.Text())
	}
}

func TestResult_TextNilContent(t *testing.T) {
	r := tool.Result{}
	if r.Text() != "" {
		t.Errorf("Text() = %q, want empty", r.Text())
	}
}

func TestTextResult(t *testing.T) {
	r := tool.TextResult("hello")
	if r.Text() != "hello" {
		t.Errorf("Text() = %q", r.Text())
	}
	if r.IsError {
		t.Error("should not be error")
	}
	if len(r.Content) != 1 {
		t.Fatalf("Content len = %d", len(r.Content))
	}
}

func TestErrorResult(t *testing.T) {
	r := tool.ErrorResult(errors.New("something broke"))
	if !r.IsError {
		t.Error("expected IsError=true")
	}
	if r.Text() != "something broke" {
		t.Errorf("Text() = %q", r.Text())
	}
}

func TestStructuredResult(t *testing.T) {
	type output struct {
		Score int `json:"score"`
	}
	r, err := tool.StructuredResult(output{Score: 95})
	if err != nil {
		t.Fatal(err)
	}
	if r.StructuredContent == nil {
		t.Fatal("StructuredContent is nil")
	}
	if r.Text() != `{"score":95}` {
		t.Errorf("Text() = %q", r.Text())
	}
	raw, ok := r.StructuredContent.(json.RawMessage)
	if !ok {
		t.Fatalf("StructuredContent type = %T", r.StructuredContent)
	}
	var parsed output
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Score != 95 {
		t.Errorf("parsed.Score = %d", parsed.Score)
	}
}

func TestResult_JSONRoundTrip(t *testing.T) {
	r := tool.TextResult("hello")
	r.StructuredContent = json.RawMessage(`{"x":1}`)
	r.Content = append(r.Content, tool.ImageContent{MIMEType: "image/png", Data: []byte("img")})

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var r2 tool.Result
	if err := json.Unmarshal(data, &r2); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if r2.Text() != "hello" {
		t.Errorf("round-trip Text() = %q", r2.Text())
	}
	if r2.StructuredContent == nil {
		t.Error("round-trip StructuredContent is nil")
	}
	if len(r2.Content) != 2 {
		t.Errorf("round-trip Content len = %d, want 2", len(r2.Content))
	}
}
