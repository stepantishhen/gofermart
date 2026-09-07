package service

import (
	"context"
	"strings"

	"github.com/stepantishhen/gofermart/internal/domain"
	"github.com/stepantishhen/gofermart/internal/luhn"
)

// Order handles order uploads and listing.
type Order struct {
	orders OrderStore
}

// NewOrder builds an Order service.
func NewOrder(orders OrderStore) *Order {
	return &Order{orders: orders}
}

// Upload registers number for the user. It returns domain.ErrInvalidOrderNumber
// for a number that fails the Luhn check, domain.ErrOrderOwnedByUser if the same
// user already uploaded it, and domain.ErrOrderOwnedByAnother otherwise.
func (o *Order) Upload(ctx context.Context, userID int64, number string) error {
	number = strings.TrimSpace(number)
	if !luhn.Valid(number) {
		return domain.ErrInvalidOrderNumber
	}
	return o.orders.Create(ctx, number, userID)
}

// List returns the user's orders newest first.
func (o *Order) List(ctx context.Context, userID int64) ([]domain.Order, error) {
	return o.orders.ListByUser(ctx, userID)
}
