// Package service contains the business logic of the loyalty system: user
// registration and login, order uploads, and balance operations. It depends on
// small storage interfaces so it can be unit-tested with fakes.
package service

import (
	"context"

	"github.com/stepantishhen/gofermart/internal/domain"
)

// UserStore persists and looks up user accounts.
type UserStore interface {
	Create(ctx context.Context, login, passwordHash string) (domain.User, error)
	GetByLogin(ctx context.Context, login string) (domain.User, error)
}

// OrderStore persists uploaded orders and lists them per user.
type OrderStore interface {
	Create(ctx context.Context, number string, userID int64) error
	ListByUser(ctx context.Context, userID int64) ([]domain.Order, error)
}

// BalanceStore reads balances and performs withdrawals.
type BalanceStore interface {
	Get(ctx context.Context, userID int64) (domain.Balance, error)
	Withdraw(ctx context.Context, userID int64, orderNumber string, sum domain.Money) error
}

// WithdrawalStore lists a user's withdrawals.
type WithdrawalStore interface {
	ListByUser(ctx context.Context, userID int64) ([]domain.Withdrawal, error)
}

// TokenIssuer creates authentication tokens for a user ID.
type TokenIssuer interface {
	Issue(userID int64) (string, error)
}
