package engine

import (
	"alpakr/internal/transform"

	"github.com/itchyny/gojq"
)

func customFunctions() []gojq.CompilerOption {
	return []gojq.CompilerOption{
		gojq.WithFunction("round2", 0, 0, func(v interface{}, _ []interface{}) interface{} {
			return transform.Round2(v)
		}),
		gojq.WithFunction("slugify", 0, 0, func(v interface{}, _ []interface{}) interface{} {
			return transform.Slugify(v)
		}),
		gojq.WithFunction("to_int", 0, 0, func(v interface{}, _ []interface{}) interface{} {
			return transform.ToInt(v)
		}),
		gojq.WithFunction("to_float", 0, 0, func(v interface{}, _ []interface{}) interface{} {
			return transform.ToFloat(v)
		}),
	}
}
