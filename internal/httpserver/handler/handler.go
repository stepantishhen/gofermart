// Package handler implements the HTTP handlers for the loyalty system API.
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/stepantishhen/gofermart/internal/domain"
	"github.com/stepantishhen/gofermart/internal/httpserver/middleware"
)

// AuthService registers and authenticates users, returning a token.
type AuthService interface {
	Register(ctx context.Context, login, password string) (string, error)
	Login(ctx context.Context, login, password string) (string, error)
}

// OrderService handles order uploads and listing.
type OrderService interface {
	Upload(ctx context.Context, userID int64, number string) error
	List(ctx context.Context, userID int64) ([]domain.Order, error)
}

// BalanceService handles balance queries, withdrawals and withdrawal listing.
type BalanceService interface {
	Get(ctx context.Context, userID int64) (domain.Balance, error)
	Withdraw(ctx context.Context, userID int64, orderNumber string, sum domain.Money) error
	Withdrawals(ctx context.Context, userID int64) ([]domain.Withdrawal, error)
}

// Handlers bundles the API handlers and their dependencies.
type Handlers struct {
	auth       AuthService
	orders     OrderService
	balance    BalanceService
	tokenTTL   time.Duration
	cookieName string
}

// New builds Handlers. tokenTTL controls the lifetime of the auth cookie the
// server sets alongside the Authorization header.
func New(auth AuthService, orders OrderService, balance BalanceService, tokenTTL time.Duration) *Handlers {
	return &Handlers{
		auth:       auth,
		orders:     orders,
		balance:    balance,
		tokenTTL:   tokenTTL,
		cookieName: middleware.AuthCookieName,
	}
}

func (h *Handlers) userID(r *http.Request) (int64, bool) {
	return middleware.UserID(r.Context())
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Handlers) setToken(w http.ResponseWriter, token string) {
	w.Header().Set("Authorization", "Bearer "+token)
	http.SetCookie(w, &http.Cookie{
		Name:     h.cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Now().Add(h.tokenTTL),
	})
}
