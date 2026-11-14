package source

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// normalizeJSON converts json.Number values produced by a UseNumber decoder
// into int, *big.Int or float64 so integers keep full precision.
func normalizeJSON(v interface{}) interface{} {
	switch x := v.(type) {
	case json.Number:
		return jsonNumber(x)
	case map[string]interface{}:
		for k, e := range x {
			x[k] = normalizeJSON(e)
		}
	case []interface{}:
		for i, e := range x {
			x[i] = normalizeJSON(e)
		}
	}
	return v
}

func jsonNumber(n json.Number) interface{} {
	s := string(n)
	if !strings.ContainsAny(s, ".eE") {
		if i, err := strconv.ParseInt(s, 10, 0); err == nil {
			return int(i)
		}
		if b, ok := new(big.Int).SetString(s, 10); ok {
			return b
		}
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// stringifyTimestamps retags implicit and explicit !!timestamp scalars as
// strings so they decode to their source text instead of time.Time.
func stringifyTimestamps(n *yaml.Node) {
	if n.Kind == yaml.ScalarNode && n.ShortTag() == "!!timestamp" {
		n.Tag = "!!str"
	}
	for _, c := range n.Content {
		stringifyTimestamps(c)
	}
}

// normalizeYAML converts values yaml.v3 can produce but gojq cannot handle:
// maps with non-string keys, sized integer types and time.Time.
func normalizeYAML(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		for k, e := range x {
			x[k] = normalizeYAML(e)
		}
	case map[interface{}]interface{}:
		m := make(map[string]interface{}, len(x))
		for k, e := range x {
			m[yamlKey(k)] = normalizeYAML(e)
		}
		return m
	case []interface{}:
		for i, e := range x {
			x[i] = normalizeYAML(e)
		}
	case int64:
		return new(big.Int).SetInt64(x)
	case uint64:
		return new(big.Int).SetUint64(x)
	case time.Time:
		return x.Format(time.RFC3339Nano)
	}
	return v
}

func yamlKey(k interface{}) string {
	switch x := k.(type) {
	case nil:
		return "null"
	case string:
		return x
	}
	return fmt.Sprint(normalizeYAML(k))
}
