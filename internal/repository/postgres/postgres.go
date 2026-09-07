// Package postgres implements the loyalty system's persistence layer on top of
// PostgreSQL using a pgx connection pool. It also embeds and applies the schema
// migrations.
package postgres

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// DB is the subset of *pgxpool.Pool the repositories use. It is satisfied by the
// pool in production and by a mock in tests.
type DB interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Storage groups the concrete repositories backed by a single connection source.
type Storage struct {
	Users       *UserRepo
	Orders      *OrderRepo
	Balances    *BalanceRepo
	Withdrawals *WithdrawalRepo
}

// New assembles the repositories over db. Connection setup and migrations are
// the caller's responsibility (see Connect and Migrate).
func New(db DB) *Storage {
	return &Storage{
		Users:       &UserRepo{pool: db},
		Orders:      &OrderRepo{pool: db},
		Balances:    &BalanceRepo{pool: db},
		Withdrawals: &WithdrawalRepo{pool: db},
	}
}

// Connect opens a pgx pool for dsn and verifies it with a ping. The caller owns
// the returned pool and must Close it.
func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

// Migrate applies all pending embedded migrations to the database at dsn.
func Migrate(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
