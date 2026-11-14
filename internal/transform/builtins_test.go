package transform

import (
	"math"
	"math/big"
	"testing"
)

func TestRound2(t *testing.T) {
	tests := []struct {
		in   interface{}
		want interface{}
	}{
		{3.14159, 3.14},
		{1.505, 1.51},
		{0.0, 0.0},
		{1.60934, 1.61},
		{42, float64(42)},
		{"not a number", "not a number"},
	}
	for _, tt := range tests {
		got := Round2(tt.in)
		if got != tt.want {
			t.Errorf("Round2(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		in   interface{}
		want interface{}
	}{
		{"Peak District", "peak-district"},
		{"  hello world  ", "hello-world"},
		{"Gwynedd", "gwynedd"},
		{"Hello & World!", "hello-world"},
		{"multiple---dashes", "multiple-dashes"},
		{42, 42},
		{"", ""},
	}
	for _, tt := range tests {
		got := Slugify(tt.in)
		if got != tt.want {
			t.Errorf("Slugify(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestToInt(t *testing.T) {
	tests := []struct {
		in   interface{}
		want interface{}
	}{
		{float64(3.9), 3},
		{42, 42},
		{"7", 7},
		{"3.14", 3},
		{"not a number", "not a number"},
		{nil, nil},
	}
	for _, tt := range tests {
		got := ToInt(tt.in)
		if got != tt.want {
			t.Errorf("ToInt(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestToFloat(t *testing.T) {
	tests := []struct {
		in   interface{}
		want interface{}
	}{
		{float64(3.14), 3.14},
		{42, float64(42)},
		{"1.5", 1.5},
		{"not a number", "not a number"},
		{nil, nil},
	}
	for _, tt := range tests {
		got := ToFloat(tt.in)
		if got != tt.want {
			t.Errorf("ToFloat(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestToInt_Large(t *testing.T) {
	for _, in := range []interface{}{"12345678901234567890", 1.2345678901234567e19} {
		b, ok := ToInt(in).(*big.Int)
		if !ok {
			t.Errorf("ToInt(%v) = %#v, want *big.Int", in, ToInt(in))
			continue
		}
		if b.Cmp(big.NewInt(0)) <= 0 || len(b.String()) != 20 {
			t.Errorf("ToInt(%v) = %s", in, b)
		}
	}
	if got := ToInt(math.Inf(1)); got != math.Inf(1) {
		t.Errorf("ToInt(+Inf) = %v, want unchanged", got)
	}
	if got := ToInt(-2.7); got != -2 {
		t.Errorf("ToInt(-2.7) = %v, want -2", got)
	}
}

func TestToFloat_BigInt(t *testing.T) {
	b, _ := new(big.Int).SetString("12345678901234567890", 10)
	if got := ToFloat(b); got != 1.2345678901234567e19 {
		t.Errorf("ToFloat(big) = %v", got)
	}
}
