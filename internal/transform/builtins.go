package transform

import (
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

var nonAlphanumDash = regexp.MustCompile(`[^a-z0-9-]+`)
var multiDash = regexp.MustCompile(`-+`)

// Round2 rounds a float to 2 decimal places.
func Round2(v interface{}) interface{} {
	switch n := v.(type) {
	case float64:
		return math.Round(n*100) / 100
	case int:
		return float64(n)
	}
	return v
}

// Slugify lowercases a string, replaces spaces with dashes, strips non-alphanumeric.
func Slugify(v interface{}) interface{} {
	s, ok := v.(string)
	if !ok {
		return v
	}
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = nonAlphanumDash.ReplaceAllString(s, "")
	s = multiDash.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// ToInt converts a string or number to an integer, truncating any fraction.
// Values outside the int range become *big.Int. Other values, and strings
// that are not numeric, are returned unchanged.
func ToInt(v interface{}) interface{} {
	switch n := v.(type) {
	case float64:
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return v
		}
		t := math.Trunc(n)
		if t >= math.MinInt && t < math.MaxInt {
			return int(t)
		}
		b, _ := big.NewFloat(t).Int(nil)
		return b
	case int, *big.Int:
		return n
	case string:
		if i, err := strconv.Atoi(n); err == nil {
			return i
		}
		if b, ok := new(big.Int).SetString(n, 10); ok {
			return b
		}
		if f, err := strconv.ParseFloat(n, 64); err == nil {
			return ToInt(f)
		}
	}
	return v
}

// ToFloat converts a string or number to float64. Other values, and strings
// that are not numeric, are returned unchanged.
func ToFloat(v interface{}) interface{} {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case *big.Int:
		f, _ := new(big.Float).SetInt(n).Float64()
		return f
	case string:
		f, err := strconv.ParseFloat(n, 64)
		if err == nil {
			return f
		}
	}
	return v
}
