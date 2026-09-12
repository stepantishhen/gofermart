package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/stepantishhen/gofermart/internal/domain"
	"github.com/stepantishhen/gofermart/internal/service"
)

type credentials struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// Register handles POST /api/user/register.
func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	token, err := h.auth.Register(r.Context(), c.Login, c.Password)
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		http.Error(w, "bad request", http.StatusBadRequest)
	case errors.Is(err, domain.ErrLoginTaken):
		http.Error(w, "login already taken", http.StatusConflict)
	case err != nil:
		http.Error(w, "internal error", http.StatusInternalServerError)
	default:
		h.setToken(w, token)
		w.WriteHeader(http.StatusOK)
	}
}

// Login handles POST /api/user/login.
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	token, err := h.auth.Login(r.Context(), c.Login, c.Password)
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		http.Error(w, "bad request", http.StatusBadRequest)
	case errors.Is(err, domain.ErrInvalidCredentials):
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
	case err != nil:
		http.Error(w, "internal error", http.StatusInternalServerError)
	default:
		h.setToken(w, token)
		w.WriteHeader(http.StatusOK)
	}
}
