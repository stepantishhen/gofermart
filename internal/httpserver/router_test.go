package httpserver_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stepantishhen/gofermart/internal/domain"
	"github.com/stepantishhen/gofermart/internal/httpserver"
	"github.com/stepantishhen/gofermart/internal/httpserver/handler"
)

type svc struct{}

func (svc) Register(context.Context, string, string) (string, error) { return "t", nil }
func (svc) Login(context.Context, string, string) (string, error)    { return "t", nil }
func (svc) Upload(context.Context, int64, string) error              { return nil }
func (svc) List(context.Context, int64) ([]domain.Order, error)      { return nil, nil }
func (svc) Get(context.Context, int64) (domain.Balance, error)       { return domain.Balance{}, nil }
func (svc) Withdraw(context.Context, int64, string, domain.Money) error {
	return nil
}
func (svc) Withdrawals(context.Context, int64) ([]domain.Withdrawal, error) { return nil, nil }

type denyParser struct{}

func (denyParser) Parse(string) (int64, error) { return 0, errors.New("no") }

func TestRouterRoutesAndAuthGate(t *testing.T) {
	h := handler.New(svc{}, svc{}, svc{}, time.Hour)
	r := httpserver.NewRouter(h, denyParser{}, slog.New(slog.DiscardHandler))

	// public route reachable
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/user/login", nil))
	if rec.Code == http.StatusNotFound {
		t.Fatal("login route not mounted")
	}

	// protected route blocked without a valid token
	for _, p := range []string{"/api/user/orders", "/api/user/balance", "/api/user/withdrawals"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s: code = %d, want 401", p, rec.Code)
		}
	}
}
