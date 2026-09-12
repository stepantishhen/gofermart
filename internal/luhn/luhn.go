// Package luhn validates numeric strings with the Luhn checksum algorithm.
package luhn

// Valid reports whether s consists only of decimal digits and satisfies the
// Luhn checksum. An empty string is not valid.
func Valid(s string) bool {
	if s == "" {
		return false
	}
	var sum int
	double := false
	for i := len(s) - 1; i >= 0; i-- {
		c := s[i]
		if c < '0' || c > '9' {
			return false
		}
		d := int(c - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}
