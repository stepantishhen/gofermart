package domain

import (
	"errors"
	"math"
	"strconv"
)

// Money is a monetary amount of loyalty points stored as an integer number of
// hundredths (kopecks). One point equals one unit of local currency, so 100
// units of Money is one point. It marshals to and from JSON as a decimal number
// (for example 500.5), matching the loyalty API contract.
type Money int64

// ErrNegativeMoney is returned when a value that must be non-negative is negative.
var ErrNegativeMoney = errors.New("money amount is negative")

// NewMoneyFromFloat converts a decimal points value into Money, rounding to the
// nearest kopeck.
func NewMoneyFromFloat(v float64) Money {
	return Money(math.Round(v * 100))
}

// Float returns the amount as a decimal number of points.
func (m Money) Float() float64 {
	return float64(m) / 100
}

// MarshalJSON renders the amount as a JSON number with no trailing zeros.
func (m Money) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatFloat(m.Float(), 'f', -1, 64)), nil
}

// UnmarshalJSON parses a JSON number of points into Money.
func (m *Money) UnmarshalJSON(data []byte) error {
	v, err := strconv.ParseFloat(string(data), 64)
	if err != nil {
		return err
	}
	*m = NewMoneyFromFloat(v)
	return nil
}
