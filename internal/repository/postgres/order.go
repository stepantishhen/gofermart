package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/stepantishhen/gofermart/internal/domain"
)

// OrderRepo stores uploaded orders and applies accrual results.
type OrderRepo struct {
	pool DB
}

// Create inserts a new order in status NEW owned by userID. If the number
// already exists it returns domain.ErrOrderOwnedByUser when the existing owner
// is userID, otherwise domain.ErrOrderOwnedByAnother.
func (r *OrderRepo) Create(ctx context.Context, number string, userID int64) error {
	ct, err := r.pool.Exec(ctx,
		`INSERT INTO orders (number, user_id) VALUES ($1, $2) ON CONFLICT (number) DO NOTHING`,
		number, userID,
	)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 1 {
		return nil
	}

	var owner int64
	if err := r.pool.QueryRow(ctx, `SELECT user_id FROM orders WHERE number = $1`, number).Scan(&owner); err != nil {
		return err
	}
	if owner == userID {
		return domain.ErrOrderOwnedByUser
	}
	return domain.ErrOrderOwnedByAnother
}

// ListByUser returns the user's orders newest first.
func (r *OrderRepo) ListByUser(ctx context.Context, userID int64) ([]domain.Order, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT number, user_id, status, COALESCE(accrual, 0)::float8, accrual IS NOT NULL, uploaded_at
		   FROM orders WHERE user_id = $1 ORDER BY uploaded_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Order
	for rows.Next() {
		var o domain.Order
		var accrual float64
		if err := rows.Scan(&o.Number, &o.UserID, &o.Status, &accrual, &o.HasAccrual, &o.UploadedAt); err != nil {
			return nil, err
		}
		o.Accrual = domain.NewMoneyFromFloat(accrual)
		out = append(out, o)
	}
	return out, rows.Err()
}

// ListPending returns up to limit orders still awaiting a final accrual result,
// oldest first.
func (r *OrderRepo) ListPending(ctx context.Context, limit int) ([]domain.Order, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT number, user_id, status FROM orders
		  WHERE status IN ('NEW', 'PROCESSING') ORDER BY uploaded_at LIMIT $1`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Order
	for rows.Next() {
		var o domain.Order
		if err := rows.Scan(&o.Number, &o.UserID, &o.Status); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// ApplyAccrual updates the order's status and, when accrual is present, credits
// the owner's balance in the same transaction.
func (r *OrderRepo) ApplyAccrual(ctx context.Context, number string, status domain.OrderStatus, accrual domain.Money, hasAccrual bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var userID int64
	err = tx.QueryRow(ctx, `SELECT user_id FROM orders WHERE number = $1 FOR UPDATE`, number).Scan(&userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	}

	if hasAccrual {
		_, err = tx.Exec(ctx, `UPDATE orders SET status = $2, accrual = $3 WHERE number = $1`,
			number, status, accrual.Float())
	} else {
		_, err = tx.Exec(ctx, `UPDATE orders SET status = $2, accrual = NULL WHERE number = $1`,
			number, status)
	}
	if err != nil {
		return err
	}

	if hasAccrual && accrual > 0 {
		if _, err = tx.Exec(ctx,
			`UPDATE balances SET current = current + $2 WHERE user_id = $1`, userID, accrual.Float(),
		); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
