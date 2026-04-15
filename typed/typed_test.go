package typed_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/dpopsuev/battery/tool"
	"github.com/dpopsuev/battery/typed"
)

type analyzeInput struct {
	Path  string `json:"path"`
	Depth int    `json:"depth,omitempty"`
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

	// Should be an object type.
	if parsed["type"] != "object" {
		t.Errorf("schema type = %v, want object", parsed["type"])
	}

	// Should have "path" and "depth" as properties.
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
	tt := typed.New[analyzeInput]("analyze", "Analyze code", func(_ context.Context, in analyzeInput) (string, error) {
		return fmt.Sprintf("analyzed %s at depth %d", in.Path, in.Depth), nil
	})

	var _ tool.Tool = tt // compile-time check
}

func TestTypedTool_Execute(t *testing.T) {
	tt := typed.New[analyzeInput]("analyze", "Analyze code", func(_ context.Context, in analyzeInput) (string, error) {
		return fmt.Sprintf("path=%s depth=%d", in.Path, in.Depth), nil
	})

	result, err := tt.Execute(context.Background(), json.RawMessage(`{"path":"main.go","depth":3}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != "path=main.go depth=3" {
		t.Errorf("result = %q", result)
	}
}

func TestTypedTool_ExecuteOmittedOptional(t *testing.T) {
	tt := typed.New[analyzeInput]("analyze", "Analyze", func(_ context.Context, in analyzeInput) (string, error) {
		return fmt.Sprintf("depth=%d", in.Depth), nil
	})

	result, err := tt.Execute(context.Background(), json.RawMessage(`{"path":"main.go"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != "depth=0" {
		t.Errorf("result = %q, want depth=0 (zero value for omitted field)", result)
	}
}

func TestTypedTool_ExecuteInvalidInput(t *testing.T) {
	tt := typed.New[analyzeInput]("analyze", "Analyze", func(_ context.Context, _ analyzeInput) (string, error) {
		return "", nil
	})

	_, err := tt.Execute(context.Background(), json.RawMessage(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON input")
	}
}

func TestTypedTool_ExecuteEmptyInput(t *testing.T) {
	tt := typed.New[analyzeInput]("analyze", "Analyze", func(_ context.Context, in analyzeInput) (string, error) {
		return fmt.Sprintf("path=%s", in.Path), nil
	})

	result, err := tt.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute with nil input: %v", err)
	}
	if result != "path=" {
		t.Errorf("result = %q, want path= (zero value)", result)
	}
}

func TestTypedTool_Name(t *testing.T) {
	tt := typed.New[analyzeInput]("my-tool", "desc", func(_ context.Context, _ analyzeInput) (string, error) {
		return "", nil
	})
	if tt.Name() != "my-tool" {
		t.Errorf("Name() = %q", tt.Name())
	}
	if tt.Description() != "desc" {
		t.Errorf("Description() = %q", tt.Description())
	}
}

func TestTypedTool_RegisterInRegistry(t *testing.T) {
	tt := typed.New[analyzeInput]("analyze", "Analyze code", func(_ context.Context, in analyzeInput) (string, error) {
		return "ok:" + in.Path, nil
	})

	reg := tool.NewRegistry()
	reg.Register(tt)

	result, err := reg.Execute(context.Background(), "analyze", json.RawMessage(`{"path":"test.go"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != "ok:test.go" {
		t.Errorf("result = %q", result)
	}
}
