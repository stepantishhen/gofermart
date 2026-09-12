package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"

	"github.com/stepantishhen/gofermart/internal/domain"
)

func newMock(t *testing.T) pgxmock.PgxPoolIface {
	t.Helper()
	m, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("new mock: %v", err)
	}
	t.Cleanup(func() {
		if err := m.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet expectations: %v", err)
		}
		m.Close()
	})
	return m
}

func TestNewAssemblesRepositories(t *testing.T) {
	m := newMock(t)
	s := New(m)
	if s.Users == nil || s.Orders == nil || s.Balances == nil || s.Withdrawals == nil {
		t.Fatalf("New returned incomplete Storage: %+v", s)
	}
}

func TestIsUniqueViolation(t *testing.T) {
	if !isUniqueViolation(&pgconn.PgError{Code: "23505"}) {
		t.Fatal("23505 should be a unique violation")
	}
	if isUniqueViolation(errors.New("plain")) || isUniqueViolation(&pgconn.PgError{Code: "23503"}) {
		t.Fatal("non-23505 must not be a unique violation")
	}
}

func TestUserRepoCreate(t *testing.T) {
	m := newMock(t)
	r := &UserRepo{pool: m}

	m.ExpectBegin()
	m.ExpectQuery("INSERT INTO users").
		WithArgs("bob", "hash").
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow(int64(1), time.Now()))
	m.ExpectExec("INSERT INTO balances").
		WithArgs(int64(1)).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	m.ExpectCommit()

	u, err := r.Create(context.Background(), "bob", "hash")
	if err != nil || u.ID != 1 {
		t.Fatalf("u=%+v err=%v", u, err)
	}
}

func TestUserRepoCreateBeginError(t *testing.T) {
	m := newMock(t)
	r := &UserRepo{pool: m}
	m.ExpectBegin().WillReturnError(errors.New("no conn"))

	if _, err := r.Create(context.Background(), "bob", "hash"); err == nil {
		t.Fatal("want begin error")
	}
}

func TestUserRepoCreateDuplicate(t *testing.T) {
	m := newMock(t)
	r := &UserRepo{pool: m}

	m.ExpectBegin()
	m.ExpectQuery("INSERT INTO users").
		WithArgs("bob", "hash").
		WillReturnError(&pgconn.PgError{Code: "23505"})
	m.ExpectRollback()

	if _, err := r.Create(context.Background(), "bob", "hash"); err != domain.ErrLoginTaken {
		t.Fatalf("err = %v, want ErrLoginTaken", err)
	}
}

func TestUserRepoCreateQueryError(t *testing.T) {
	m := newMock(t)
	r := &UserRepo{pool: m}

	m.ExpectBegin()
	m.ExpectQuery("INSERT INTO users").
		WithArgs("bob", "hash").
		WillReturnError(errors.New("boom"))
	m.ExpectRollback()

	if _, err := r.Create(context.Background(), "bob", "hash"); err == nil || err == domain.ErrLoginTaken {
		t.Fatalf("want generic error, got %v", err)
	}
}

func TestUserRepoCreateBalanceInsertError(t *testing.T) {
	m := newMock(t)
	r := &UserRepo{pool: m}

	m.ExpectBegin()
	m.ExpectQuery("INSERT INTO users").
		WithArgs("bob", "hash").
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow(int64(1), time.Now()))
	m.ExpectExec("INSERT INTO balances").
		WithArgs(int64(1)).
		WillReturnError(errors.New("balance boom"))
	m.ExpectRollback()

	if _, err := r.Create(context.Background(), "bob", "hash"); err == nil {
		t.Fatal("want balance insert error")
	}
}

func TestUserRepoCreateCommitError(t *testing.T) {
	m := newMock(t)
	r := &UserRepo{pool: m}

	m.ExpectBegin()
	m.ExpectQuery("INSERT INTO users").
		WithArgs("bob", "hash").
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow(int64(1), time.Now()))
	m.ExpectExec("INSERT INTO balances").
		WithArgs(int64(1)).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	m.ExpectCommit().WillReturnError(errors.New("commit boom"))

	if _, err := r.Create(context.Background(), "bob", "hash"); err == nil {
		t.Fatal("want commit error")
	}
}

func TestUserRepoGetByLogin(t *testing.T) {
	m := newMock(t)
	r := &UserRepo{pool: m}

	m.ExpectQuery("SELECT id, login, password_hash, created_at FROM users").
		WithArgs("bob").
		WillReturnRows(pgxmock.NewRows([]string{"id", "login", "password_hash", "created_at"}).
			AddRow(int64(5), "bob", "h", time.Now()))

	u, err := r.GetByLogin(context.Background(), "bob")
	if err != nil || u.ID != 5 || u.Login != "bob" {
		t.Fatalf("u=%+v err=%v", u, err)
	}
}

func TestUserRepoGetByLoginNotFound(t *testing.T) {
	m := newMock(t)
	r := &UserRepo{pool: m}

	m.ExpectQuery("SELECT id, login, password_hash, created_at FROM users").
		WithArgs("ghost").
		WillReturnError(pgx.ErrNoRows)

	if _, err := r.GetByLogin(context.Background(), "ghost"); err != domain.ErrUserNotFound {
		t.Fatalf("err = %v, want ErrUserNotFound", err)
	}
}

func TestUserRepoGetByLoginError(t *testing.T) {
	m := newMock(t)
	r := &UserRepo{pool: m}

	m.ExpectQuery("SELECT id, login, password_hash, created_at FROM users").
		WithArgs("bob").
		WillReturnError(errors.New("boom"))

	if _, err := r.GetByLogin(context.Background(), "bob"); err == nil || err == domain.ErrUserNotFound {
		t.Fatalf("want generic error, got %v", err)
	}
}

func TestOrderRepoCreateNew(t *testing.T) {
	m := newMock(t)
	r := &OrderRepo{pool: m}

	m.ExpectExec("INSERT INTO orders").
		WithArgs("12345678903", int64(1)).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	if err := r.Create(context.Background(), "12345678903", 1); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestOrderRepoCreateExecError(t *testing.T) {
	m := newMock(t)
	r := &OrderRepo{pool: m}

	m.ExpectExec("INSERT INTO orders").
		WithArgs("12345678903", int64(1)).
		WillReturnError(errors.New("boom"))

	if err := r.Create(context.Background(), "12345678903", 1); err == nil {
		t.Fatal("want exec error")
	}
}

func TestOrderRepoCreateConflictSameUser(t *testing.T) {
	m := newMock(t)
	r := &OrderRepo{pool: m}

	m.ExpectExec("INSERT INTO orders").
		WithArgs("12345678903", int64(1)).
		WillReturnResult(pgxmock.NewResult("INSERT", 0))
	m.ExpectQuery("SELECT user_id FROM orders WHERE number").
		WithArgs("12345678903").
		WillReturnRows(pgxmock.NewRows([]string{"user_id"}).AddRow(int64(1)))

	if err := r.Create(context.Background(), "12345678903", 1); err != domain.ErrOrderOwnedByUser {
		t.Fatalf("err = %v", err)
	}
}

func TestOrderRepoCreateConflictOtherUser(t *testing.T) {
	m := newMock(t)
	r := &OrderRepo{pool: m}

	m.ExpectExec("INSERT INTO orders").
		WithArgs("12345678903", int64(1)).
		WillReturnResult(pgxmock.NewResult("INSERT", 0))
	m.ExpectQuery("SELECT user_id FROM orders WHERE number").
		WithArgs("12345678903").
		WillReturnRows(pgxmock.NewRows([]string{"user_id"}).AddRow(int64(2)))

	if err := r.Create(context.Background(), "12345678903", 1); err != domain.ErrOrderOwnedByAnother {
		t.Fatalf("err = %v", err)
	}
}

func TestOrderRepoCreateConflictOwnerLookupError(t *testing.T) {
	m := newMock(t)
	r := &OrderRepo{pool: m}

	m.ExpectExec("INSERT INTO orders").
		WithArgs("12345678903", int64(1)).
		WillReturnResult(pgxmock.NewResult("INSERT", 0))
	m.ExpectQuery("SELECT user_id FROM orders WHERE number").
		WithArgs("12345678903").
		WillReturnError(errors.New("boom"))

	if err := r.Create(context.Background(), "12345678903", 1); err == nil {
		t.Fatal("want owner lookup error")
	}
}

func TestOrderRepoListByUser(t *testing.T) {
	m := newMock(t)
	r := &OrderRepo{pool: m}

	m.ExpectQuery("SELECT number, user_id, status").
		WithArgs(int64(1)).
		WillReturnRows(pgxmock.NewRows(
			[]string{"number", "user_id", "status", "accrual", "has", "uploaded_at"}).
			AddRow("9278923470", int64(1), "PROCESSED", 500.0, true, time.Now()).
			AddRow("346436439", int64(1), "INVALID", 0.0, false, time.Now()))

	got, err := r.ListByUser(context.Background(), 1)
	if err != nil || len(got) != 2 {
		t.Fatalf("got=%v err=%v", got, err)
	}
	if got[0].Accrual != domain.NewMoneyFromFloat(500) || !got[0].HasAccrual {
		t.Fatalf("row 0 = %+v", got[0])
	}
}

func TestOrderRepoListByUserQueryError(t *testing.T) {
	m := newMock(t)
	r := &OrderRepo{pool: m}

	m.ExpectQuery("SELECT number, user_id, status").
		WithArgs(int64(1)).
		WillReturnError(errors.New("boom"))

	if _, err := r.ListByUser(context.Background(), 1); err == nil {
		t.Fatal("want query error")
	}
}

func TestOrderRepoListByUserScanError(t *testing.T) {
	m := newMock(t)
	r := &OrderRepo{pool: m}

	m.ExpectQuery("SELECT number, user_id, status").
		WithArgs(int64(1)).
		WillReturnRows(pgxmock.NewRows([]string{"number"}).AddRow("only-one-column"))

	if _, err := r.ListByUser(context.Background(), 1); err == nil {
		t.Fatal("want scan error")
	}
}

func TestOrderRepoListPending(t *testing.T) {
	m := newMock(t)
	r := &OrderRepo{pool: m}

	m.ExpectQuery("SELECT number, user_id, status FROM orders").
		WithArgs(50).
		WillReturnRows(pgxmock.NewRows([]string{"number", "user_id", "status"}).
			AddRow("a", int64(1), "NEW").
			AddRow("b", int64(2), "PROCESSING"))

	got, err := r.ListPending(context.Background(), 50)
	if err != nil || len(got) != 2 || got[0].Number != "a" {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestOrderRepoListPendingQueryError(t *testing.T) {
	m := newMock(t)
	r := &OrderRepo{pool: m}

	m.ExpectQuery("SELECT number, user_id, status FROM orders").
		WithArgs(50).
		WillReturnError(errors.New("boom"))

	if _, err := r.ListPending(context.Background(), 50); err == nil {
		t.Fatal("want query error")
	}
}

func TestOrderRepoApplyAccrualCreditsBalance(t *testing.T) {
	m := newMock(t)
	r := &OrderRepo{pool: m}

	m.ExpectBegin()
	m.ExpectQuery("SELECT user_id FROM orders WHERE number").
		WithArgs("9278923470").
		WillReturnRows(pgxmock.NewRows([]string{"user_id"}).AddRow(int64(3)))
	m.ExpectExec("UPDATE orders SET status").
		WithArgs("9278923470", domain.OrderStatusProcessed, 500.0).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	m.ExpectExec("UPDATE balances SET current = current").
		WithArgs(int64(3), 500.0).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	m.ExpectCommit()

	err := r.ApplyAccrual(context.Background(), "9278923470", domain.OrderStatusProcessed, domain.NewMoneyFromFloat(500), true)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestOrderRepoApplyAccrualNoAccrual(t *testing.T) {
	m := newMock(t)
	r := &OrderRepo{pool: m}

	m.ExpectBegin()
	m.ExpectQuery("SELECT user_id FROM orders WHERE number").
		WithArgs("346436439").
		WillReturnRows(pgxmock.NewRows([]string{"user_id"}).AddRow(int64(3)))
	m.ExpectExec("UPDATE orders SET status = .+, accrual = NULL").
		WithArgs("346436439", domain.OrderStatusInvalid).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	m.ExpectCommit()

	err := r.ApplyAccrual(context.Background(), "346436439", domain.OrderStatusInvalid, 0, false)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestOrderRepoApplyAccrualOrderGone(t *testing.T) {
	m := newMock(t)
	r := &OrderRepo{pool: m}

	m.ExpectBegin()
	m.ExpectQuery("SELECT user_id FROM orders WHERE number").
		WithArgs("gone").
		WillReturnError(pgx.ErrNoRows)
	m.ExpectRollback()

	if err := r.ApplyAccrual(context.Background(), "gone", domain.OrderStatusProcessed, 0, false); err != nil {
		t.Fatalf("err = %v, want nil when the order disappeared", err)
	}
}

func TestOrderRepoApplyAccrualBeginError(t *testing.T) {
	m := newMock(t)
	r := &OrderRepo{pool: m}
	m.ExpectBegin().WillReturnError(errors.New("no conn"))

	if err := r.ApplyAccrual(context.Background(), "x", domain.OrderStatusProcessed, 0, false); err == nil {
		t.Fatal("want begin error")
	}
}

func TestOrderRepoApplyAccrualUpdateError(t *testing.T) {
	m := newMock(t)
	r := &OrderRepo{pool: m}

	m.ExpectBegin()
	m.ExpectQuery("SELECT user_id FROM orders WHERE number").
		WithArgs("x").
		WillReturnRows(pgxmock.NewRows([]string{"user_id"}).AddRow(int64(3)))
	m.ExpectExec("UPDATE orders SET status").
		WithArgs("x", domain.OrderStatusProcessed, 10.0).
		WillReturnError(errors.New("update boom"))
	m.ExpectRollback()

	if err := r.ApplyAccrual(context.Background(), "x", domain.OrderStatusProcessed, domain.NewMoneyFromFloat(10), true); err == nil {
		t.Fatal("want update error")
	}
}

func TestBalanceRepoGet(t *testing.T) {
	m := newMock(t)
	r := &BalanceRepo{pool: m}

	m.ExpectQuery("SELECT current::float8, withdrawn::float8 FROM balances").
		WithArgs(int64(1)).
		WillReturnRows(pgxmock.NewRows([]string{"current", "withdrawn"}).AddRow(500.5, 42.0))

	b, err := r.Get(context.Background(), 1)
	if err != nil || b.Current != domain.NewMoneyFromFloat(500.5) || b.Withdrawn != domain.NewMoneyFromFloat(42) {
		t.Fatalf("b=%+v err=%v", b, err)
	}
}

func TestBalanceRepoGetError(t *testing.T) {
	m := newMock(t)
	r := &BalanceRepo{pool: m}

	m.ExpectQuery("SELECT current::float8, withdrawn::float8 FROM balances").
		WithArgs(int64(1)).
		WillReturnError(errors.New("boom"))

	if _, err := r.Get(context.Background(), 1); err == nil {
		t.Fatal("want query error")
	}
}

func TestBalanceRepoWithdrawInsufficient(t *testing.T) {
	m := newMock(t)
	r := &BalanceRepo{pool: m}

	m.ExpectBegin()
	m.ExpectQuery("SELECT current::float8 FROM balances").
		WithArgs(int64(1)).
		WillReturnRows(pgxmock.NewRows([]string{"current"}).AddRow(10.0))
	m.ExpectRollback()

	err := r.Withdraw(context.Background(), 1, "2377225624", domain.NewMoneyFromFloat(50))
	if err != domain.ErrInsufficientBalance {
		t.Fatalf("err = %v", err)
	}
}

func TestBalanceRepoWithdrawOK(t *testing.T) {
	m := newMock(t)
	r := &BalanceRepo{pool: m}

	m.ExpectBegin()
	m.ExpectQuery("SELECT current::float8 FROM balances").
		WithArgs(int64(1)).
		WillReturnRows(pgxmock.NewRows([]string{"current"}).AddRow(100.0))
	m.ExpectExec("UPDATE balances SET current = current -").
		WithArgs(int64(1), 30.0).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	m.ExpectExec("INSERT INTO withdrawals").
		WithArgs(int64(1), "2377225624", 30.0).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	m.ExpectCommit()

	if err := r.Withdraw(context.Background(), 1, "2377225624", domain.NewMoneyFromFloat(30)); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestBalanceRepoWithdrawBeginError(t *testing.T) {
	m := newMock(t)
	r := &BalanceRepo{pool: m}
	m.ExpectBegin().WillReturnError(errors.New("no conn"))

	if err := r.Withdraw(context.Background(), 1, "2377225624", domain.NewMoneyFromFloat(1)); err == nil {
		t.Fatal("want begin error")
	}
}

func TestBalanceRepoWithdrawSelectError(t *testing.T) {
	m := newMock(t)
	r := &BalanceRepo{pool: m}

	m.ExpectBegin()
	m.ExpectQuery("SELECT current::float8 FROM balances").
		WithArgs(int64(1)).
		WillReturnError(errors.New("boom"))
	m.ExpectRollback()

	if err := r.Withdraw(context.Background(), 1, "2377225624", domain.NewMoneyFromFloat(1)); err == nil {
		t.Fatal("want select error")
	}
}

func TestBalanceRepoWithdrawUpdateError(t *testing.T) {
	m := newMock(t)
	r := &BalanceRepo{pool: m}

	m.ExpectBegin()
	m.ExpectQuery("SELECT current::float8 FROM balances").
		WithArgs(int64(1)).
		WillReturnRows(pgxmock.NewRows([]string{"current"}).AddRow(100.0))
	m.ExpectExec("UPDATE balances SET current = current -").
		WithArgs(int64(1), 30.0).
		WillReturnError(errors.New("boom"))
	m.ExpectRollback()

	if err := r.Withdraw(context.Background(), 1, "2377225624", domain.NewMoneyFromFloat(30)); err == nil {
		t.Fatal("want update error")
	}
}

func TestBalanceRepoWithdrawInsertError(t *testing.T) {
	m := newMock(t)
	r := &BalanceRepo{pool: m}

	m.ExpectBegin()
	m.ExpectQuery("SELECT current::float8 FROM balances").
		WithArgs(int64(1)).
		WillReturnRows(pgxmock.NewRows([]string{"current"}).AddRow(100.0))
	m.ExpectExec("UPDATE balances SET current = current -").
		WithArgs(int64(1), 30.0).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	m.ExpectExec("INSERT INTO withdrawals").
		WithArgs(int64(1), "2377225624", 30.0).
		WillReturnError(errors.New("boom"))
	m.ExpectRollback()

	if err := r.Withdraw(context.Background(), 1, "2377225624", domain.NewMoneyFromFloat(30)); err == nil {
		t.Fatal("want insert error")
	}
}

func TestWithdrawalRepoListByUser(t *testing.T) {
	m := newMock(t)
	r := &WithdrawalRepo{pool: m}

	m.ExpectQuery("SELECT order_number, user_id, sum::float8, processed_at FROM withdrawals").
		WithArgs(int64(1)).
		WillReturnRows(pgxmock.NewRows([]string{"order_number", "user_id", "sum", "processed_at"}).
			AddRow("2377225624", int64(1), 500.0, time.Now()))

	got, err := r.ListByUser(context.Background(), 1)
	if err != nil || len(got) != 1 || got[0].Sum != domain.NewMoneyFromFloat(500) {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestWithdrawalRepoListByUserQueryError(t *testing.T) {
	m := newMock(t)
	r := &WithdrawalRepo{pool: m}

	m.ExpectQuery("SELECT order_number, user_id, sum::float8, processed_at FROM withdrawals").
		WithArgs(int64(1)).
		WillReturnError(errors.New("boom"))

	if _, err := r.ListByUser(context.Background(), 1); err == nil {
		t.Fatal("want query error")
	}
}

func TestWithdrawalRepoListByUserScanError(t *testing.T) {
	m := newMock(t)
	r := &WithdrawalRepo{pool: m}

	m.ExpectQuery("SELECT order_number, user_id, sum::float8, processed_at FROM withdrawals").
		WithArgs(int64(1)).
		WillReturnRows(pgxmock.NewRows([]string{"order_number"}).AddRow("only-one"))

	if _, err := r.ListByUser(context.Background(), 1); err == nil {
		t.Fatal("want scan error")
	}
}
