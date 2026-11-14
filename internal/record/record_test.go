package record

import (
	"encoding/json"
	"math/big"
	"testing"
)

func TestAppendJSON_Order(t *testing.T) {
	inner := &Object{Keys: []string{"z", "a"}, Values: []interface{}{1, "<&>"}}
	b, _ := new(big.Int).SetString("123456789012345678901234567890", 10)
	o := &Object{
		Keys:   []string{"second", "first", "list", "plain", "big"},
		Values: []interface{}{inner, nil, []interface{}{inner, 2}, map[string]interface{}{"y": 1, "x": 2}, b},
	}
	got, err := AppendJSON(nil, o)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"second":{"z":1,"a":"<&>"},"first":null,"list":[{"z":1,"a":"<&>"},2],"plain":{"x":2,"y":1},"big":123456789012345678901234567890}`
	if string(got) != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
	if !json.Valid(got) {
		t.Error("invalid JSON")
	}
}

func TestMarshalJSON(t *testing.T) {
	o := &Object{Keys: []string{"b", "a"}, Values: []interface{}{1, 2}}
	got, err := json.Marshal(map[string]interface{}{"o": o})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"o":{"b":1,"a":2}}` {
		t.Errorf("got %s", got)
	}
}

func TestGet(t *testing.T) {
	o := &Object{Keys: []string{"a", "b"}, Values: []interface{}{1, nil}}
	if v, ok := o.Get("b"); !ok || v != nil {
		t.Errorf("Get(b) = %v, %v", v, ok)
	}
	if _, ok := o.Get("c"); ok {
		t.Error("Get(c) should be absent")
	}
}
