package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/stepantishhen/gofermart/internal/domain"
)

// UserRepo stores and retrieves user accounts.
type UserRepo struct {
	pool DB
}

// Create inserts a new user together with a zeroed balance row in a single
// transaction. It returns domain.ErrLoginTaken if the login already exists.
func (r *UserRepo) Create(ctx context.Context, login, passwordHash string) (domain.User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.User{}, err
	}
	defer tx.Rollback(ctx)

	var u domain.User
	u.Login = login
	u.PasswordHash = passwordHash
	err = tx.QueryRow(ctx,
		`INSERT INTO users (login, password_hash) VALUES ($1, $2) RETURNING id, created_at`,
		login, passwordHash,
	).Scan(&u.ID, &u.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.User{}, domain.ErrLoginTaken
		}
		return domain.User{}, err
	}

	if _, err = tx.Exec(ctx, `INSERT INTO balances (user_id) VALUES ($1)`, u.ID); err != nil {
		return domain.User{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.User{}, err
	}
	return u, nil
}

// GetByLogin returns the user with the given login or domain.ErrUserNotFound.
func (r *UserRepo) GetByLogin(ctx context.Context, login string) (domain.User, error) {
	var u domain.User
	err := r.pool.QueryRow(ctx,
		`SELECT id, login, password_hash, created_at FROM users WHERE login = $1`, login,
	).Scan(&u.ID, &u.Login, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrUserNotFound
	}
	if err != nil {
		return domain.User{}, err
	}
	return u, nil
}
