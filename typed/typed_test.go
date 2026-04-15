package typed_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dpopsuev/battery/tool"
	"github.com/dpopsuev/battery/typed"
)

type analyzeInput struct {
	Path  string `json:"path"`
	Depth int    `json:"depth,omitempty"`
}

type analyzeOutput struct {
	Path  string `json:"path"`
	Depth int    `json:"depth"`
}

func TestSchemaFor_Struct(t *testing.T) {
	schema := typed.SchemaFor[analyzeInput]()
	if len(schema) == 0 {
		t.Fatal("schema is empty")
	}

	var parsed map[string]any
	if err := json.Unmarshal(schema, &parsed); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}

	if parsed["type"] != "object" {
		t.Errorf("schema type = %v, want object", parsed["type"])
	}

	props, ok := parsed["properties"].(map[string]any)
	if !ok {
		t.Fatalf("missing properties in schema: %v", parsed)
	}
	if _, ok := props["path"]; !ok {
		t.Error("missing 'path' property")
	}
	if _, ok := props["depth"]; !ok {
		t.Error("missing 'depth' property")
	}
}

func TestSchemaForSafe_Success(t *testing.T) {
	schema, err := typed.SchemaForSafe[analyzeInput]()
	if err != nil {
		t.Fatalf("SchemaForSafe: %v", err)
	}
	if len(schema) == 0 {
		t.Fatal("schema is empty")
	}
}

func TestTypedTool_ImplementsTool(t *testing.T) {
	tt := typed.New("analyze", "Analyze code", func(_ context.Context, in analyzeInput) (analyzeOutput, error) {
		return analyzeOutput(in), nil
	})
	var _ tool.Tool = tt
}

func TestTypedTool_Execute(t *testing.T) {
	tt := typed.New("analyze", "Analyze code", func(_ context.Context, in analyzeInput) (analyzeOutput, error) {
		return analyzeOutput(in), nil
	})

	result, err := tt.Execute(context.Background(), json.RawMessage(`{"path":"main.go","depth":3}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// StructuredContent should be set.
	if result.StructuredContent == nil {
		t.Fatal("StructuredContent is nil")
	}

	// Text fallback should contain JSON.
	var parsed analyzeOutput
	if err := json.Unmarshal([]byte(result.Text()), &parsed); err != nil {
		t.Fatalf("unmarshal text: %v", err)
	}
	if parsed.Path != "main.go" || parsed.Depth != 3 {
		t.Errorf("parsed = %+v", parsed)
	}
}

func TestTypedTool_ExecuteOmittedOptional(t *testing.T) {
	tt := typed.New("analyze", "Analyze", func(_ context.Context, in analyzeInput) (analyzeOutput, error) {
		return analyzeOutput{Depth: in.Depth}, nil
	})

	result, err := tt.Execute(context.Background(), json.RawMessage(`{"path":"main.go"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var parsed analyzeOutput
	if err := json.Unmarshal([]byte(result.Text()), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Depth != 0 {
		t.Errorf("depth = %d, want 0 (zero value for omitted field)", parsed.Depth)
	}
}

func TestTypedTool_ExecuteInvalidInput(t *testing.T) {
	tt := typed.New("analyze", "Analyze", func(_ context.Context, _ analyzeInput) (analyzeOutput, error) {
		return analyzeOutput{}, nil
	})

	_, err := tt.Execute(context.Background(), json.RawMessage(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON input")
	}
}

func TestTypedTool_ExecuteEmptyInput(t *testing.T) {
	tt := typed.New("analyze", "Analyze", func(_ context.Context, in analyzeInput) (analyzeOutput, error) {
		return analyzeOutput{Path: in.Path}, nil
	})

	result, err := tt.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute with nil input: %v", err)
	}

	var parsed analyzeOutput
	if err := json.Unmarshal([]byte(result.Text()), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Path != "" {
		t.Errorf("path = %q, want empty (zero value)", parsed.Path)
	}
}

func TestTypedTool_Name(t *testing.T) {
	tt := typed.New("my-tool", "desc", func(_ context.Context, _ analyzeInput) (analyzeOutput, error) {
		return analyzeOutput{}, nil
	})
	if tt.Name() != "my-tool" {
		t.Errorf("Name() = %q", tt.Name())
	}
	if tt.Description() != "desc" {
		t.Errorf("Description() = %q", tt.Description())
	}
}

func TestTypedTool_OutputSchema(t *testing.T) {
	tt := typed.New("analyze", "Analyze", func(_ context.Context, _ analyzeInput) (analyzeOutput, error) {
		return analyzeOutput{}, nil
	})

	schema := tt.OutputSchema()
	if len(schema) == 0 {
		t.Fatal("OutputSchema is empty")
	}

	var parsed map[string]any
	if err := json.Unmarshal(schema, &parsed); err != nil {
		t.Fatalf("OutputSchema is not valid JSON: %v", err)
	}
	if parsed["type"] != "object" {
		t.Errorf("OutputSchema type = %v, want object", parsed["type"])
	}
}

func TestTypedTool_NotCacheableByDefault(t *testing.T) {
	tt := typed.New("analyze", "Analyze", func(_ context.Context, _ analyzeInput) (analyzeOutput, error) {
		return analyzeOutput{}, nil
	})
	var t2 tool.Tool = tt
	if _, ok := t2.(tool.Cacheable); ok {
		t.Error("TypedTool should NOT implement Cacheable by default")
	}
}

func TestTypedTool_WithDefaultCacheKey(t *testing.T) {
	ct := typed.New("analyze", "Analyze", func(_ context.Context, _ analyzeInput) (analyzeOutput, error) {
		return analyzeOutput{}, nil
	}).WithDefaultCacheKey()

	var t2 tool.Tool = ct
	c, ok := t2.(tool.Cacheable)
	if !ok {
		t.Fatal("WithDefaultCacheKey should implement Cacheable")
	}
	key, cok := c.CacheKey(context.Background(), json.RawMessage(`{"path":"x"}`))
	if !cok {
		t.Fatal("expected cacheable")
	}
	if key == "" {
		t.Error("expected non-empty key")
	}
}

func TestTypedTool_RegisterInRegistry(t *testing.T) {
	tt := typed.New("analyze", "Analyze code", func(_ context.Context, in analyzeInput) (analyzeOutput, error) {
		return analyzeOutput{Path: in.Path, Depth: 42}, nil
	})

	reg := tool.NewRegistry()
	reg.Register(tt)

	result, err := reg.Execute(context.Background(), "analyze", json.RawMessage(`{"path":"test.go"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var parsed analyzeOutput
	if err := json.Unmarshal([]byte(result.Text()), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Path != "test.go" || parsed.Depth != 42 {
		t.Errorf("parsed = %+v", parsed)
	}
}
