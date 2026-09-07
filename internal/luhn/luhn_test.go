package luhn

import "testing"

func TestValid(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", false},
		{"single zero", "0", true},
		{"spec sample 12345678903", "12345678903", true},
		{"spec sample 9278923470", "9278923470", true},
		{"spec sample 2377225624", "2377225624", true},
		{"wrong checksum", "12345678901", false},
		{"non-digit", "1234abc", false},
		{"leading spaces", " 12345678903", false},
		{"long valid", "79927398713", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Valid(tt.in); got != tt.want {
				t.Fatalf("Valid(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
