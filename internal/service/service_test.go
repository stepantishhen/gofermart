package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stepantishhen/gofermart/internal/domain"
)

type fakeUsers struct {
	byLogin   map[string]domain.User
	nextID    int64
	createErr error
}

func (f *fakeUsers) Create(_ context.Context, login, hash string) (domain.User, error) {
	if f.createErr != nil {
		return domain.User{}, f.createErr
	}
	if _, ok := f.byLogin[login]; ok {
		return domain.User{}, domain.ErrLoginTaken
	}
	f.nextID++
	u := domain.User{ID: f.nextID, Login: login, PasswordHash: hash}
	if f.byLogin == nil {
		f.byLogin = map[string]domain.User{}
	}
	f.byLogin[login] = u
	return u, nil
}

func (f *fakeUsers) GetByLogin(_ context.Context, login string) (domain.User, error) {
	u, ok := f.byLogin[login]
	if !ok {
		return domain.User{}, domain.ErrUserNotFound
	}
	return u, nil
}

type fakeTokens struct{}

func (fakeTokens) Issue(userID int64) (string, error) { return "token-for-user", nil }

func TestAuthRegister(t *testing.T) {
	a := NewAuth(&fakeUsers{}, fakeTokens{})

	tok, err := a.Register(context.Background(), "bob", "pw")
	if err != nil || tok == "" {
		t.Fatalf("register: tok=%q err=%v", tok, err)
	}

	if _, err := a.Register(context.Background(), "bob", "pw"); !errors.Is(err, domain.ErrLoginTaken) {
		t.Fatalf("want ErrLoginTaken, got %v", err)
	}
	if _, err := a.Register(context.Background(), "", "pw"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestAuthLogin(t *testing.T) {
	users := &fakeUsers{}
	a := NewAuth(users, fakeTokens{})
	if _, err := a.Register(context.Background(), "bob", "pw"); err != nil {
		t.Fatal(err)
	}

	if _, err := a.Login(context.Background(), "bob", "pw"); err != nil {
		t.Fatalf("valid login failed: %v", err)
	}
	if _, err := a.Login(context.Background(), "bob", "wrong"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
	if _, err := a.Login(context.Background(), "nobody", "pw"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials for missing user, got %v", err)
	}
}

type fakeOrders struct {
	created map[string]int64
	err     error
}

func (f *fakeOrders) Create(_ context.Context, number string, userID int64) error {
	if f.err != nil {
		return f.err
	}
	if owner, ok := f.created[number]; ok {
		if owner == userID {
			return domain.ErrOrderOwnedByUser
		}
		return domain.ErrOrderOwnedByAnother
	}
	if f.created == nil {
		f.created = map[string]int64{}
	}
	f.created[number] = userID
	return nil
}

func (f *fakeOrders) ListByUser(_ context.Context, userID int64) ([]domain.Order, error) {
	var out []domain.Order
	for n, o := range f.created {
		if o == userID {
			out = append(out, domain.Order{Number: n, UserID: o, Status: domain.OrderStatusNew})
		}
	}
	return out, nil
}

func TestOrderUpload(t *testing.T) {
	o := NewOrder(&fakeOrders{})

	if err := o.Upload(context.Background(), 1, "12345678903"); err != nil {
		t.Fatalf("valid upload failed: %v", err)
	}
	if err := o.Upload(context.Background(), 1, "12345678903"); !errors.Is(err, domain.ErrOrderOwnedByUser) {
		t.Fatalf("want ErrOrderOwnedByUser, got %v", err)
	}
	if err := o.Upload(context.Background(), 2, "12345678903"); !errors.Is(err, domain.ErrOrderOwnedByAnother) {
		t.Fatalf("want ErrOrderOwnedByAnother, got %v", err)
	}
	if err := o.Upload(context.Background(), 1, "12345678901"); !errors.Is(err, domain.ErrInvalidOrderNumber) {
		t.Fatalf("want ErrInvalidOrderNumber, got %v", err)
	}
}

type fakeBalances struct {
	bal         domain.Balance
	withdrawErr error
}

func (f *fakeBalances) Get(_ context.Context, _ int64) (domain.Balance, error) {
	return f.bal, nil
}

func (f *fakeBalances) Withdraw(_ context.Context, _ int64, _ string, sum domain.Money) error {
	if f.withdrawErr != nil {
		return f.withdrawErr
	}
	if f.bal.Current < sum {
		return domain.ErrInsufficientBalance
	}
	f.bal.Current -= sum
	f.bal.Withdrawn += sum
	return nil
}

type fakeWithdrawals struct{ items []domain.Withdrawal }

func (f *fakeWithdrawals) ListByUser(_ context.Context, _ int64) ([]domain.Withdrawal, error) {
	return f.items, nil
}

func TestBalanceWithdraw(t *testing.T) {
	bals := &fakeBalances{bal: domain.Balance{Current: domain.NewMoneyFromFloat(100)}}
	b := NewBalance(bals, &fakeWithdrawals{})

	if err := b.Withdraw(context.Background(), 1, "2377225624", domain.NewMoneyFromFloat(40)); err != nil {
		t.Fatalf("valid withdraw failed: %v", err)
	}
	if bals.bal.Current != domain.NewMoneyFromFloat(60) {
		t.Fatalf("current = %v, want 60", bals.bal.Current.Float())
	}
	if err := b.Withdraw(context.Background(), 1, "2377225624", domain.NewMoneyFromFloat(1000)); !errors.Is(err, domain.ErrInsufficientBalance) {
		t.Fatalf("want ErrInsufficientBalance, got %v", err)
	}
	if err := b.Withdraw(context.Background(), 1, "123", domain.NewMoneyFromFloat(1)); !errors.Is(err, domain.ErrInvalidOrderNumber) {
		t.Fatalf("want ErrInvalidOrderNumber, got %v", err)
	}
}
