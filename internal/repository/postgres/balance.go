package postgres

import (
	"context"

	"github.com/stepantishhen/gofermart/internal/domain"
)

// BalanceRepo reads balances and performs withdrawals.
type BalanceRepo struct {
	pool DB
}

// Get returns the user's current and lifetime-withdrawn point totals.
func (r *BalanceRepo) Get(ctx context.Context, userID int64) (domain.Balance, error) {
	var current, withdrawn float64
	err := r.pool.QueryRow(ctx,
		`SELECT current::float8, withdrawn::float8 FROM balances WHERE user_id = $1`, userID,
	).Scan(&current, &withdrawn)
	if err != nil {
		return domain.Balance{}, err
	}
	return domain.Balance{
		Current:   domain.NewMoneyFromFloat(current),
		Withdrawn: domain.NewMoneyFromFloat(withdrawn),
	}, nil
}

// Withdraw debits sum from the user's balance and records the withdrawal against
// orderNumber, all in one transaction. It returns domain.ErrInsufficientBalance
// when the balance is too low.
func (r *BalanceRepo) Withdraw(ctx context.Context, userID int64, orderNumber string, sum domain.Money) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var current float64
	if err := tx.QueryRow(ctx,
		`SELECT current::float8 FROM balances WHERE user_id = $1 FOR UPDATE`, userID,
	).Scan(&current); err != nil {
		return err
	}
	if domain.NewMoneyFromFloat(current) < sum {
		return domain.ErrInsufficientBalance
	}

	if _, err := tx.Exec(ctx,
		`UPDATE balances SET current = current - $2, withdrawn = withdrawn + $2 WHERE user_id = $1`,
		userID, sum.Float(),
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO withdrawals (user_id, order_number, sum) VALUES ($1, $2, $3)`,
		userID, orderNumber, sum.Float(),
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
