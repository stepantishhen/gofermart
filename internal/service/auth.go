package service

import (
	"context"
	"errors"
	"strings"

	"github.com/stepantishhen/gofermart/internal/auth"
	"github.com/stepantishhen/gofermart/internal/domain"
)

// ErrInvalidInput signals a malformed request payload.
var ErrInvalidInput = errors.New("invalid input")

// Auth handles registration and login, returning a signed token on success.
type Auth struct {
	users  UserStore
	tokens TokenIssuer
}

// NewAuth builds an Auth service.
func NewAuth(users UserStore, tokens TokenIssuer) *Auth {
	return &Auth{users: users, tokens: tokens}
}

// Register creates a new account and returns an authentication token. It returns
// ErrInvalidInput for empty credentials and domain.ErrLoginTaken if the login
// exists.
func (a *Auth) Register(ctx context.Context, login, password string) (string, error) {
	if strings.TrimSpace(login) == "" || password == "" {
		return "", ErrInvalidInput
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return "", err
	}
	u, err := a.users.Create(ctx, login, hash)
	if err != nil {
		return "", err
	}
	return a.tokens.Issue(u.ID)
}

// Login verifies credentials and returns an authentication token, or
// domain.ErrInvalidCredentials on mismatch.
func (a *Auth) Login(ctx context.Context, login, password string) (string, error) {
	if strings.TrimSpace(login) == "" || password == "" {
		return "", ErrInvalidInput
	}
	u, err := a.users.GetByLogin(ctx, login)
	if errors.Is(err, domain.ErrUserNotFound) {
		return "", domain.ErrInvalidCredentials
	}
	if err != nil {
		return "", err
	}
	if !auth.CheckPassword(u.PasswordHash, password) {
		return "", domain.ErrInvalidCredentials
	}
	return a.tokens.Issue(u.ID)
}
