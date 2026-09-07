package postgres

import (
	"context"

	"github.com/stepantishhen/gofermart/internal/domain"
)

// WithdrawalRepo lists a user's withdrawals.
type WithdrawalRepo struct {
	pool DB
}

// ListByUser returns the user's withdrawals newest first.
func (r *WithdrawalRepo) ListByUser(ctx context.Context, userID int64) ([]domain.Withdrawal, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT order_number, user_id, sum::float8, processed_at
		   FROM withdrawals WHERE user_id = $1 ORDER BY processed_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Withdrawal
	for rows.Next() {
		var w domain.Withdrawal
		var sum float64
		if err := rows.Scan(&w.OrderNumber, &w.UserID, &sum, &w.ProcessedAt); err != nil {
			return nil, err
		}
		w.Sum = domain.NewMoneyFromFloat(sum)
		out = append(out, w)
	}
	return out, rows.Err()
}
