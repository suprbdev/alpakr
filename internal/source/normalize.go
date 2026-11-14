package source

import (
	"encoding/json"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// normalizeJSON converts json.Number values produced by a UseNumber decoder
// into int, *big.Int or float64 so integers keep full precision. Numbers
// beyond the float64 range are clamped to ±math.MaxFloat64, as jq does.
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
	if math.IsInf(f, 0) {
		return math.Copysign(math.MaxFloat64, f)
	}
	return f
}
