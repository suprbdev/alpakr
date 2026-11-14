package engine

import (
	"fmt"
	"testing"

	"github.com/suprbdev/alpakr/internal/config"
)

func BenchmarkRun_Each(b *testing.B) {
	e, err := New(cfg(map[string]config.HandlerConfig{
		"root": {
			Each:   true,
			Filter: ".status >= 400",
			Fields: fields(
				"id", field(".id"),
				"slug", field(".name | slugify"),
				"loc", inlineFields(".loc", fields(
					"city", field(".city"),
					"cc", field(".cc | ascii_upcase"),
				)),
				"tags", subHandler("tag", ".tags"),
			),
		},
		"tag": {Each: true, Fields: fields("t", field("."))},
	}))
	if err != nil {
		b.Fatal(err)
	}
	records := make([]interface{}, 1000)
	for i := range records {
		records[i] = map[string]interface{}{
			"id":     i,
			"name":   fmt.Sprintf("User %d", i),
			"status": 400 + i%2,
			"loc":    map[string]interface{}{"city": "Leeds", "cc": "gb"},
			"tags":   []interface{}{"a", "b"},
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := e.Run("root", records); err != nil {
			b.Fatal(err)
		}
	}
}
