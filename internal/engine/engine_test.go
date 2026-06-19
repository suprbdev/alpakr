package engine

import (
	"alpakr/internal/config"
	"testing"
)

func cfg(handlers map[string]config.HandlerConfig) *config.Config {
	return &config.Config{
		Source:   config.SourceConfig{Path: "/tmp/x.json", Format: "json"},
		Output:   config.OutputConfig{Format: "json", Indent: 2},
		Handlers: handlers,
	}
}

func field(expr string) config.FieldConfig { return config.FieldConfig{Expr: expr} }

func subHandler(handler, input string) config.FieldConfig {
	return config.FieldConfig{Handler: handler, Input: input}
}

func mustEngine(t *testing.T, c *config.Config) *Engine {
	t.Helper()
	e, err := New(c)
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	return e
}

func TestRun_SimpleFields(t *testing.T) {
	e := mustEngine(t, cfg(map[string]config.HandlerConfig{
		"root": {
			Fields: map[string]config.FieldConfig{
				"id":    field(".id"),
				"upper": field(".name | ascii_upcase"),
			},
		},
	}))

	data := map[string]interface{}{"id": float64(1), "name": "alice"}
	result, err := e.Run("root", data)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := result.(map[string]interface{})
	if out["id"] != float64(1) {
		t.Errorf("id = %v, want 1", out["id"])
	}
	if out["upper"] != "ALICE" {
		t.Errorf("upper = %v, want ALICE", out["upper"])
	}
}

func TestRun_Each(t *testing.T) {
	e := mustEngine(t, cfg(map[string]config.HandlerConfig{
		"root": {
			Each: true,
			Fields: map[string]config.FieldConfig{
				"name": field(".name"),
			},
		},
	}))

	data := []interface{}{
		map[string]interface{}{"name": "alice"},
		map[string]interface{}{"name": "bob"},
	}
	result, err := e.Run("root", data)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	items := result.([]interface{})
	if len(items) != 2 {
		t.Fatalf("len = %d, want 2", len(items))
	}
	if items[0].(map[string]interface{})["name"] != "alice" {
		t.Errorf("items[0].name = %v, want alice", items[0].(map[string]interface{})["name"])
	}
}

func TestRun_Filter(t *testing.T) {
	e := mustEngine(t, cfg(map[string]config.HandlerConfig{
		"root": {
			Each:   true,
			Filter: ".active",
			Fields: map[string]config.FieldConfig{
				"name": field(".name"),
			},
		},
	}))

	data := []interface{}{
		map[string]interface{}{"name": "alice", "active": true},
		map[string]interface{}{"name": "bob", "active": false},
		map[string]interface{}{"name": "carol", "active": true},
	}
	result, err := e.Run("root", data)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	items := result.([]interface{})
	if len(items) != 2 {
		t.Fatalf("len = %d, want 2 (bob filtered out)", len(items))
	}
}

func TestRun_InputSelector(t *testing.T) {
	e := mustEngine(t, cfg(map[string]config.HandlerConfig{
		"root": {
			Input: ".data",
			Each:  true,
			Fields: map[string]config.FieldConfig{
				"name": field(".name"),
			},
		},
	}))

	data := map[string]interface{}{
		"data": []interface{}{
			map[string]interface{}{"name": "alice"},
		},
	}
	result, err := e.Run("root", data)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	items := result.([]interface{})
	if len(items) != 1 {
		t.Fatalf("len = %d, want 1", len(items))
	}
}

func TestRun_SubHandler(t *testing.T) {
	e := mustEngine(t, cfg(map[string]config.HandlerConfig{
		"root": {
			Fields: map[string]config.FieldConfig{
				"name":     field(".name"),
				"location": subHandler("loc", ".loc"),
			},
		},
		"loc": {
			Fields: map[string]config.FieldConfig{
				"city":    field(".city"),
				"country": field(".country | ascii_upcase"),
			},
		},
	}))

	data := map[string]interface{}{
		"name": "alice",
		"loc":  map[string]interface{}{"city": "London", "country": "gb"},
	}
	result, err := e.Run("root", data)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := result.(map[string]interface{})
	loc := out["location"].(map[string]interface{})
	if loc["city"] != "London" {
		t.Errorf("city = %v, want London", loc["city"])
	}
	if loc["country"] != "GB" {
		t.Errorf("country = %v, want GB", loc["country"])
	}
}

func TestRun_CustomFunctions(t *testing.T) {
	e := mustEngine(t, cfg(map[string]config.HandlerConfig{
		"root": {
			Fields: map[string]config.FieldConfig{
				"rounded": field(".miles * 1.60934 | round2"),
				"slug":    field(".label | slugify"),
				"as_int":  field(".score | to_int"),
				"as_float": field(".count | to_float"),
			},
		},
	}))

	data := map[string]interface{}{
		"miles": 8.5,
		"label": "Peak District",
		"score": float64(7),
		"count": 3,
	}
	result, err := e.Run("root", data)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := result.(map[string]interface{})
	if out["rounded"] != 13.68 {
		t.Errorf("rounded = %v, want 13.68", out["rounded"])
	}
	if out["slug"] != "peak-district" {
		t.Errorf("slug = %v, want peak-district", out["slug"])
	}
	if out["as_int"] != 7 {
		t.Errorf("as_int = %v, want 7", out["as_int"])
	}
	if out["as_float"] != float64(3) {
		t.Errorf("as_float = %v, want 3.0", out["as_float"])
	}
}

func TestRun_EachOnNonArray(t *testing.T) {
	e := mustEngine(t, cfg(map[string]config.HandlerConfig{
		"root": {
			Each:   true,
			Fields: map[string]config.FieldConfig{"name": field(".name")},
		},
	}))

	_, err := e.Run("root", map[string]interface{}{"name": "alice"})
	if err == nil {
		t.Fatal("expected error for each on non-array")
	}
}

func TestRun_UnknownHandler(t *testing.T) {
	e := mustEngine(t, cfg(map[string]config.HandlerConfig{
		"root": {Fields: map[string]config.FieldConfig{"id": field(".id")}},
	}))

	_, err := e.Run("missing", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for unknown handler")
	}
}

func TestCompile_BadJQExpression(t *testing.T) {
	_, err := New(cfg(map[string]config.HandlerConfig{
		"root": {
			Fields: map[string]config.FieldConfig{
				"bad": field(".foo ||| invalid"),
			},
		},
	}))
	if err == nil {
		t.Fatal("expected compile error for invalid jq")
	}
}
