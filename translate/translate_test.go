package translate_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/dpopsuev/battery/translate"
)

const kindSource = "knowledge.source"

func TestRegistry_RegisterAndTranslate(t *testing.T) {
	reg := translate.NewRegistry()

	reg.Register("locus.scan_complete", func(_ string, _ json.RawMessage) (translate.Result, error) {
		return translate.Result{
			Records: []translate.Record{
				{ID: "auth", Kind: kindSource, Title: "Auth Module", Labels: []string{"source:locus"}},
				{ID: "db", Kind: kindSource, Title: "Database", Labels: []string{"source:locus"}},
			},
			Edges: []translate.Edge{
				{From: "auth", Relation: "depends_on", To: "db"},
			},
		}, nil
	})

	result, err := reg.Translate("locus.scan_complete", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 2 {
		t.Fatalf("records = %d; want 2", len(result.Records))
	}
	if result.Records[0].Kind != kindSource {
		t.Errorf("kind = %q; want knowledge.source", result.Records[0].Kind)
	}
	if len(result.Edges) != 1 || result.Edges[0].Relation != "depends_on" {
		t.Errorf("edges = %v; want 1 depends_on edge", result.Edges)
	}
}

func TestRegistry_UnknownTypeReturnsError(t *testing.T) {
	reg := translate.NewRegistry()

	_, err := reg.Translate("unknown.event", json.RawMessage(`{}`))
	if !errors.Is(err, translate.ErrNoTranslator) {
		t.Errorf("err = %v; want ErrNoTranslator", err)
	}
}

func TestRegistry_Has(t *testing.T) {
	reg := translate.NewRegistry()
	reg.Register("x", func(_ string, _ json.RawMessage) (translate.Result, error) {
		return translate.Result{}, nil
	})

	if !reg.Has("x") {
		t.Error("Has(x) = false; want true")
	}
	if reg.Has("y") {
		t.Error("Has(y) = true; want false")
	}
}

func TestRegistry_Types(t *testing.T) {
	reg := translate.NewRegistry()
	reg.Register("a", func(_ string, _ json.RawMessage) (translate.Result, error) {
		return translate.Result{}, nil
	})
	reg.Register("b", func(_ string, _ json.RawMessage) (translate.Result, error) {
		return translate.Result{}, nil
	})

	types := reg.Types()
	if len(types) != 2 {
		t.Fatalf("types = %d; want 2", len(types))
	}
}

func TestRecord_SectionsAndExtra(t *testing.T) {
	r := translate.Record{
		ID:    "comp-1",
		Kind:  kindSource,
		Title: "Auth Service",
		Sections: []translate.Section{
			{Name: "architecture", Text: "Handles JWT tokens"},
			{Name: "metrics", Text: "LOC: 500, Churn: 12"},
		},
		Extra: map[string]any{
			"loc":   500,
			"churn": 12,
		},
	}

	if len(r.Sections) != 2 {
		t.Errorf("sections = %d; want 2", len(r.Sections))
	}
	if r.Extra["loc"] != 500 {
		t.Errorf("extra.loc = %v; want 500", r.Extra["loc"])
	}
}

func TestResult_JSONRoundTrip(t *testing.T) {
	original := translate.Result{
		Records: []translate.Record{
			{ID: "a", Kind: "test", Title: "Test A"},
		},
		Edges: []translate.Edge{
			{From: "a", Relation: "relates_to", To: "b"},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}

	var decoded translate.Result
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if len(decoded.Records) != 1 || decoded.Records[0].ID != "a" {
		t.Errorf("roundtrip records = %v", decoded.Records)
	}
	if len(decoded.Edges) != 1 || decoded.Edges[0].From != "a" {
		t.Errorf("roundtrip edges = %v", decoded.Edges)
	}
}
