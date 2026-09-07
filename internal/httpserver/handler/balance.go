package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/stepantishhen/gofermart/internal/domain"
	"github.com/stepantishhen/gofermart/internal/service"
)

type balanceView struct {
	Current   domain.Money `json:"current"`
	Withdrawn domain.Money `json:"withdrawn"`
}

type withdrawRequest struct {
	Order string       `json:"order"`
	Sum   domain.Money `json:"sum"`
}

type withdrawalView struct {
	Order       string       `json:"order"`
	Sum         domain.Money `json:"sum"`
	ProcessedAt string       `json:"processed_at"`
}

// GetBalance handles GET /api/user/balance.
func (h *Handlers) GetBalance(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	b, err := h.balance.Get(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, balanceView{Current: b.Current, Withdrawn: b.Withdrawn})
}

// Withdraw handles POST /api/user/balance/withdraw.
func (h *Handlers) Withdraw(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req withdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	err := h.balance.Withdraw(r.Context(), userID, req.Order, req.Sum)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusOK)
	case errors.Is(err, domain.ErrInvalidOrderNumber), errors.Is(err, service.ErrInvalidInput):
		http.Error(w, "invalid order number", http.StatusUnprocessableEntity)
	case errors.Is(err, domain.ErrInsufficientBalance):
		http.Error(w, "insufficient balance", http.StatusPaymentRequired)
	default:
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// ListWithdrawals handles GET /api/user/withdrawals.
func (h *Handlers) ListWithdrawals(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	items, err := h.balance.Withdrawals(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(items) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	views := make([]withdrawalView, 0, len(items))
	for _, it := range items {
		views = append(views, withdrawalView{
			Order:       it.OrderNumber,
			Sum:         it.Sum,
			ProcessedAt: it.ProcessedAt.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, views)
}
