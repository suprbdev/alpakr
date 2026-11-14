package engine

import (
	"reflect"
	"strings"
	"testing"

	"github.com/suprbdev/alpakr/internal/config"
	"github.com/suprbdev/alpakr/internal/record"
)

func cfg(handlers map[string]config.HandlerConfig) *config.Config {
	return &config.Config{
		Source:   config.SourceConfig{Path: "/tmp/x.json", Format: "json"},
		Output:   config.OutputConfig{Format: "json"},
		Handlers: handlers,
	}
}

func field(expr string) config.FieldConfig { return config.FieldConfig{Expr: expr} }

// fields builds an ordered field list from alternating names and configs.
func fields(kv ...interface{}) config.Fields {
	fs := make(config.Fields, 0, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		fs = append(fs, config.Field{Name: kv[i].(string), FieldConfig: kv[i+1].(config.FieldConfig)})
	}
	return fs
}

// plain converts record.Objects in v to maps so results can be compared
// with map literals.
func plain(v interface{}) interface{} {
	switch x := v.(type) {
	case *record.Object:
		m := make(map[string]interface{}, len(x.Keys))
		for i, k := range x.Keys {
			m[k] = plain(x.Values[i])
		}
		return m
	case []interface{}:
		a := make([]interface{}, len(x))
		for i, e := range x {
			a[i] = plain(e)
		}
		return a
	}
	return v
}

func runPlain(e *Engine, name string, data interface{}) (interface{}, error) {
	v, err := e.Run(name, data)
	return plain(v), err
}

func runLimitPlain(e *Engine, name string, data interface{}, limit int) (interface{}, error) {
	v, err := e.RunLimit(name, data, limit)
	return plain(v), err
}

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
			Fields: fields(
				"id", field(".id"),
				"upper", field(".name | ascii_upcase"),
			),
		},
	}))

	data := map[string]interface{}{"id": float64(1), "name": "alice"}
	result, err := runPlain(e, "root", data)
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
			Fields: fields(
				"name", field(".name"),
			),
		},
	}))

	data := []interface{}{
		map[string]interface{}{"name": "alice"},
		map[string]interface{}{"name": "bob"},
	}
	result, err := runPlain(e, "root", data)
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
			Fields: fields(
				"name", field(".name"),
			),
		},
	}))

	data := []interface{}{
		map[string]interface{}{"name": "alice", "active": true},
		map[string]interface{}{"name": "bob", "active": false},
		map[string]interface{}{"name": "carol", "active": true},
	}
	result, err := runPlain(e, "root", data)
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
			Fields: fields(
				"name", field(".name"),
			),
		},
	}))

	data := map[string]interface{}{
		"data": []interface{}{
			map[string]interface{}{"name": "alice"},
		},
	}
	result, err := runPlain(e, "root", data)
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
			Fields: fields(
				"name", field(".name"),
				"location", subHandler("loc", ".loc"),
			),
		},
		"loc": {
			Fields: fields(
				"city", field(".city"),
				"country", field(".country | ascii_upcase"),
			),
		},
	}))

	data := map[string]interface{}{
		"name": "alice",
		"loc":  map[string]interface{}{"city": "London", "country": "gb"},
	}
	result, err := runPlain(e, "root", data)
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
			Fields: fields(
				"rounded", field(".miles * 1.60934 | round2"),
				"slug", field(".label | slugify"),
				"as_int", field(".score | to_int"),
				"as_float", field(".count | to_float"),
			),
		},
	}))

	data := map[string]interface{}{
		"miles": 8.5,
		"label": "Peak District",
		"score": float64(7),
		"count": 3,
	}
	result, err := runPlain(e, "root", data)
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
			Fields: fields("name", field(".name")),
		},
	}))

	_, err := runPlain(e, "root", map[string]interface{}{"name": "alice"})
	if err == nil {
		t.Fatal("expected error for each on non-array")
	}
}

func TestRun_UnknownHandler(t *testing.T) {
	e := mustEngine(t, cfg(map[string]config.HandlerConfig{
		"root": {Fields: fields("id", field(".id"))},
	}))

	_, err := runPlain(e, "missing", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for unknown handler")
	}
}

func inlineFields(input string, fs config.Fields) config.FieldConfig {
	return config.FieldConfig{Input: input, Fields: fs}
}

func TestRun_InlineFields(t *testing.T) {
	e := mustEngine(t, cfg(map[string]config.HandlerConfig{
		"root": {
			Fields: fields(
				"name", field(".name"),
				"location", inlineFields(".loc", fields(
					"city", field(".city"),
					"country", field(".country | ascii_upcase"),
				)),
			),
		},
	}))

	data := map[string]interface{}{
		"name": "alice",
		"loc":  map[string]interface{}{"city": "London", "country": "gb"},
	}
	result, err := runPlain(e, "root", data)
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

func TestRun_InlineFields_NoInput(t *testing.T) {
	e := mustEngine(t, cfg(map[string]config.HandlerConfig{
		"root": {
			Fields: fields(
				"meta", inlineFields("", fields(
					"id", field(".id"),
					"slug", field(".name | slugify"),
				)),
			),
		},
	}))

	data := map[string]interface{}{"id": float64(7), "name": "Hello World"}
	result, err := runPlain(e, "root", data)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	meta := result.(map[string]interface{})["meta"].(map[string]interface{})
	if meta["id"] != float64(7) {
		t.Errorf("id = %v, want 7", meta["id"])
	}
	if meta["slug"] != "hello-world" {
		t.Errorf("slug = %v, want hello-world", meta["slug"])
	}
}

func TestRun_InlineFields_DeepNesting(t *testing.T) {
	e := mustEngine(t, cfg(map[string]config.HandlerConfig{
		"root": {
			Fields: fields(
				"a", inlineFields("", fields(
					"b", inlineFields("", fields(
						"val", field(".x"),
					)),
				)),
			),
		},
	}))

	data := map[string]interface{}{"x": "deep"}
	result, err := runPlain(e, "root", data)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	a := result.(map[string]interface{})["a"].(map[string]interface{})
	b := a["b"].(map[string]interface{})
	if b["val"] != "deep" {
		t.Errorf("val = %v, want deep", b["val"])
	}
}

func TestCompile_BadJQExpression(t *testing.T) {
	_, err := New(cfg(map[string]config.HandlerConfig{
		"root": {
			Fields: fields(
				"bad", field(".foo ||| invalid"),
			),
		},
	}))
	if err == nil {
		t.Fatal("expected compile error for invalid jq")
	}
}

func TestExpr_PlainPathMatchesJQ(t *testing.T) {
	exprs := []string{".", ".a", ".a.b", ".a_1.B2", "._x", ".a.b.c"}
	inputs := []interface{}{
		nil,
		"str",
		42,
		[]interface{}{1},
		map[string]interface{}{},
		map[string]interface{}{"a": nil},
		map[string]interface{}{"a": 1},
		map[string]interface{}{"a": []interface{}{1}},
		map[string]interface{}{"a": map[string]interface{}{"b": "x"}},
		map[string]interface{}{"a": map[string]interface{}{"b": map[string]interface{}{"c": true}}},
		map[string]interface{}{"a_1": map[string]interface{}{"B2": 3}, "_x": "y"},
	}
	for _, src := range exprs {
		x, err := compileExpr(src, customFunctions())
		if err != nil {
			t.Fatalf("compile %q: %v", src, err)
		}
		if x.path == nil {
			t.Errorf("%q: expected plain path fast path", src)
		}
		slow := &expr{code: x.code}
		for _, in := range inputs {
			got, gotErr := x.eval(in)
			want, wantErr := slow.eval(in)
			if (gotErr != nil) != (wantErr != nil) || !reflect.DeepEqual(got, want) {
				t.Errorf("%q on %#v: fast = (%#v, %v), jq = (%#v, %v)", src, in, got, gotErr, want, wantErr)
			}
		}
	}
}

func TestExpr_NotPlainPath(t *testing.T) {
	for _, src := range []string{".a?", ".a[0]", ".[\"a\"]", ".a | .b", " .a", ".a.b // 1", "..", ".a-b"} {
		x, err := compileExpr(src, customFunctions())
		if err != nil {
			continue
		}
		if x.path != nil {
			t.Errorf("%q should not use the plain path fast path", src)
		}
	}
}

func TestCompile_InvalidInputSelectors(t *testing.T) {
	for _, f := range fields(
		"sub-handler input", subHandler("other", ".[[["),
		"inline input", inlineFields(".[[[", fields("x", field("."))),
		"nested inline", inlineFields("", fields("x", field(".a |||"))),
		"empty expression", field(""),
	) {
		name := f.Name
		_, err := New(cfg(map[string]config.HandlerConfig{
			"root":  {Fields: fields("f", f.FieldConfig)},
			"other": {Fields: fields("x", field("."))},
		}))
		if err == nil {
			t.Errorf("%s: expected compile error", name)
		}
	}
}

func TestCompile_UndefinedSubHandler(t *testing.T) {
	_, err := New(cfg(map[string]config.HandlerConfig{
		"root": {Fields: fields("f", subHandler("missing", ""))},
	}))
	if err == nil {
		t.Fatal("expected error for undefined sub-handler")
	}
}

func TestRun_DottedFieldNameDoesNotCollide(t *testing.T) {
	e := mustEngine(t, cfg(map[string]config.HandlerConfig{
		"root": {Fields: fields(
			"a.b", field(`"flat"`),
			"a", inlineFields("", fields("b", field(`"nested"`))),
		)},
	}))
	got, err := runPlain(e, "root", map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{"a.b": "flat", "a": map[string]interface{}{"b": "nested"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestRun_RecursiveHandler(t *testing.T) {
	e := mustEngine(t, cfg(map[string]config.HandlerConfig{
		"node": {Fields: fields(
			"name", field(".name"),
			"children", subHandler("nodes", ".children"),
		)},
		"nodes": {Each: true, Input: ". // []", Fields: fields(
			"name", field(".name"),
			"children", subHandler("nodes", ".children"),
		)},
	}))
	tree := map[string]interface{}{"name": "a", "children": []interface{}{
		map[string]interface{}{"name": "b"},
	}}
	got, err := runPlain(e, "node", tree)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{"name": "a", "children": []interface{}{
		map[string]interface{}{"name": "b", "children": []interface{}{}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestRun_ErrorPaths(t *testing.T) {
	e := mustEngine(t, cfg(map[string]config.HandlerConfig{
		"root": {Fields: fields(
			"loc", inlineFields(".loc", fields("city", field(".city | ascii_upcase"))),
		)},
	}))
	_, err := runPlain(e, "root", map[string]interface{}{"loc": map[string]interface{}{"city": 1}})
	if err == nil || !strings.HasPrefix(err.Error(), "root.loc.city: ") {
		t.Errorf("err = %v, want prefix root.loc.city:", err)
	}
}

func TestRunLimit(t *testing.T) {
	e := mustEngine(t, cfg(map[string]config.HandlerConfig{
		"root": {
			Each:   true,
			Filter: ".keep",
			Fields: fields("id", field(".id")),
		},
	}))
	var items []interface{}
	for i := 0; i < 10; i++ {
		items = append(items, map[string]interface{}{"id": i, "keep": i%2 == 1})
	}
	for _, tc := range []struct {
		limit int
		want  []interface{}
	}{
		{0, []interface{}{1, 3, 5, 7, 9}},
		{2, []interface{}{1, 3}},
		{100, []interface{}{1, 3, 5, 7, 9}},
	} {
		got, err := runLimitPlain(e, "root", items, tc.limit)
		if err != nil {
			t.Fatal(err)
		}
		var ids []interface{}
		for _, r := range got.([]interface{}) {
			ids = append(ids, r.(map[string]interface{})["id"])
		}
		if !reflect.DeepEqual(ids, tc.want) {
			t.Errorf("limit %d: ids = %v, want %v", tc.limit, ids, tc.want)
		}
	}
}

func TestRunLimit_StopsEarly(t *testing.T) {
	e := mustEngine(t, cfg(map[string]config.HandlerConfig{
		"root": {Each: true, Fields: fields("x", field(".x | ascii_upcase"))},
	}))
	items := []interface{}{
		map[string]interface{}{"x": "a"},
		map[string]interface{}{"x": 1},
	}
	if _, err := runLimitPlain(e, "root", items, 1); err != nil {
		t.Errorf("record past the limit was evaluated: %v", err)
	}
	if _, err := runLimitPlain(e, "root", items, 0); err == nil {
		t.Error("expected error from second record without limit")
	}
}

func TestRun_SelfReferenceFailsCleanly(t *testing.T) {
	e := mustEngine(t, cfg(map[string]config.HandlerConfig{
		"root": {Fields: fields("self", subHandler("root", ""))},
	}))
	_, err := runPlain(e, "root", map[string]interface{}{})
	if err == nil || !strings.Contains(err.Error(), "nested more than") {
		t.Fatalf("err = %v, want depth error", err)
	}
	if len(err.Error()) > 200 {
		t.Errorf("depth error should not repeat the path per level: %d bytes", len(err.Error()))
	}
}

func TestRun_JQEnvironment(t *testing.T) {
	t.Setenv("ALPAKR_REGION", "eu")
	e := mustEngine(t, cfg(map[string]config.HandlerConfig{
		"root": {Fields: fields(
			"a", field("$ENV.ALPAKR_REGION"),
			"b", field("env.ALPAKR_REGION"),
		)},
	}))
	got, err := runPlain(e, "root", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{"a": "eu", "b": "eu"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestRun_EachOnNullIsEmpty(t *testing.T) {
	e := mustEngine(t, cfg(map[string]config.HandlerConfig{
		"root": {Fields: fields("tags", subHandler("tag", ".tags"))},
		"tag":  {Each: true, Fields: fields("t", field("."))},
	}))
	for _, in := range []interface{}{
		map[string]interface{}{"tags": nil},
		map[string]interface{}{},
	} {
		got, err := runPlain(e, "root", in)
		if err != nil {
			t.Fatalf("%v: %v", in, err)
		}
		want := map[string]interface{}{"tags": []interface{}{}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%v: got %#v, want %#v", in, got, want)
		}
	}
}

func TestRun_EachOnNonArrayNamesJQType(t *testing.T) {
	e := mustEngine(t, cfg(map[string]config.HandlerConfig{
		"root": {Each: true, Fields: fields("x", field("."))},
	}))
	for in, want := range map[string]interface{}{"object": map[string]interface{}{}, "string": "s", "number": 1.5, "boolean": true} {
		_, err := e.Run("root", want)
		if err == nil || !strings.HasSuffix(err.Error(), "got "+in) {
			t.Errorf("%s: err = %v", in, err)
		}
	}
}

func TestRun_Vars(t *testing.T) {
	c := cfg(map[string]config.HandlerConfig{
		"root": {
			Each:   true,
			Input:  ".[$section]",
			Filter: ".n >= $min",
			Fields: fields("n", field(".n"), "region", field("$region"), "both", field(`"\($region)-\(.n)"`)),
		},
	})
	e, err := New(c, WithVars(map[string]interface{}{"region": "eu", "min": 2, "section": "items"}))
	if err != nil {
		t.Fatal(err)
	}
	got, err := runPlain(e, "root", map[string]interface{}{"items": []interface{}{
		map[string]interface{}{"n": 1}, map[string]interface{}{"n": 3},
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := []interface{}{map[string]interface{}{"n": 3, "region": "eu", "both": "eu-3"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestCompile_UndefinedVar(t *testing.T) {
	_, err := New(cfg(map[string]config.HandlerConfig{
		"root": {Fields: fields("x", field("$missing"))},
	}))
	if err == nil || !strings.Contains(err.Error(), "$missing") {
		t.Errorf("err = %v, want undefined variable error", err)
	}
}

func TestRunLimit_ParallelMatchesSequential(t *testing.T) {
	c := cfg(map[string]config.HandlerConfig{
		"root": {
			Each:   true,
			Filter: ".i % 3 != 0",
			Fields: fields("i", field(".i"), "sq", field(".i * .i"), "sub", subHandler("sub", ".")),
		},
		"sub": {Fields: fields("s", field(`"n\(.i)"`))},
	})
	seq := mustEngine(t, c)
	par, err := New(c, WithWorkers(8))
	if err != nil {
		t.Fatal(err)
	}
	items := make([]interface{}, 5000)
	for i := range items {
		items[i] = map[string]interface{}{"i": i}
	}
	for _, limit := range []int{0, 1, 300, 3333, 10000} {
		want, err := runLimitPlain(seq, "root", items, limit)
		if err != nil {
			t.Fatal(err)
		}
		got, err := runLimitPlain(par, "root", items, limit)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("limit %d: parallel result differs (len %d vs %d)", limit, len(got.([]interface{})), len(want.([]interface{})))
		}
	}
}

func TestRunLimit_ParallelReportsFirstError(t *testing.T) {
	c := cfg(map[string]config.HandlerConfig{
		"root": {Each: true, Fields: fields("x", field(".x | ascii_upcase"))},
	})
	par, err := New(c, WithWorkers(8))
	if err != nil {
		t.Fatal(err)
	}
	items := make([]interface{}, 4000)
	for i := range items {
		items[i] = map[string]interface{}{"x": "ok"}
	}
	items[3900] = map[string]interface{}{"x": 2}
	items[700] = map[string]interface{}{"x": 1}
	_, err = par.RunLimit("root", items, 0)
	if err == nil || !strings.Contains(err.Error(), "(1)") {
		t.Errorf("err = %v, want the error from record 700", err)
	}
	if _, err := par.RunLimit("root", items, 500); err != nil {
		t.Errorf("records past the limit should not fail the run: %v", err)
	}
}
