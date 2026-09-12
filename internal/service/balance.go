package service

import (
	"context"
	"strings"

	"github.com/stepantishhen/gofermart/internal/domain"
	"github.com/stepantishhen/gofermart/internal/luhn"
)

// Balance handles balance queries and withdrawals.
type Balance struct {
	balances    BalanceStore
	withdrawals WithdrawalStore
}

// NewBalance builds a Balance service.
func NewBalance(balances BalanceStore, withdrawals WithdrawalStore) *Balance {
	return &Balance{balances: balances, withdrawals: withdrawals}
}

// Get returns the user's current and withdrawn point totals.
func (b *Balance) Get(ctx context.Context, userID int64) (domain.Balance, error) {
	return b.balances.Get(ctx, userID)
}

// Withdraw debits sum against orderNumber. It returns domain.ErrInvalidOrderNumber
// for a bad number and domain.ErrInsufficientBalance when funds are short.
func (b *Balance) Withdraw(ctx context.Context, userID int64, orderNumber string, sum domain.Money) error {
	orderNumber = strings.TrimSpace(orderNumber)
	if !luhn.Valid(orderNumber) {
		return domain.ErrInvalidOrderNumber
	}
	if sum <= 0 {
		return ErrInvalidInput
	}
	return b.balances.Withdraw(ctx, userID, orderNumber, sum)
}

// Withdrawals returns the user's withdrawals newest first.
func (b *Balance) Withdrawals(ctx context.Context, userID int64) ([]domain.Withdrawal, error) {
	return b.withdrawals.ListByUser(ctx, userID)
}
