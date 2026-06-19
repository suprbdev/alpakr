package transform

import (
	"math"
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

// ToInt converts a string or float to int.
func ToInt(v interface{}) interface{} {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case string:
		i, err := strconv.Atoi(n)
		if err == nil {
			return i
		}
		f, err := strconv.ParseFloat(n, 64)
		if err == nil {
			return int(f)
		}
	}
	return v
}

// ToFloat converts a string or int to float64.
func ToFloat(v interface{}) interface{} {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case string:
		f, err := strconv.ParseFloat(n, 64)
		if err == nil {
			return f
		}
	}
	return v
}
