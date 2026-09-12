package domain

import (
	"encoding/json"
	"testing"
)

func TestMoneyJSON(t *testing.T) {
	tests := []struct {
		m    Money
		want string
	}{
		{NewMoneyFromFloat(500.5), "500.5"},
		{NewMoneyFromFloat(42), "42"},
		{NewMoneyFromFloat(0), "0"},
		{NewMoneyFromFloat(733.17), "733.17"},
	}
	for _, tt := range tests {
		b, err := json.Marshal(tt.m)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != tt.want {
			t.Fatalf("Marshal(%d) = %s, want %s", int64(tt.m), b, tt.want)
		}
		var back Money
		if err := json.Unmarshal(b, &back); err != nil {
			t.Fatal(err)
		}
		if back != tt.m {
			t.Fatalf("round trip: got %d, want %d", int64(back), int64(tt.m))
		}
	}
}

func TestMoneyArithmeticStaysExact(t *testing.T) {
	sum := NewMoneyFromFloat(0.1) + NewMoneyFromFloat(0.2)
	if sum != NewMoneyFromFloat(0.3) {
		t.Fatalf("0.1 + 0.2 = %v", sum.Float())
	}
}
