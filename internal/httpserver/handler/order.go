package handler

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/stepantishhen/gofermart/internal/domain"
)

type orderView struct {
	Number     string             `json:"number"`
	Status     domain.OrderStatus `json:"status"`
	Accrual    *domain.Money      `json:"accrual,omitempty"`
	UploadedAt string             `json:"uploaded_at"`
}

// UploadOrder handles POST /api/user/orders.
func (h *Handlers) UploadOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	number := string(body)
	if number == "" {
		http.Error(w, "empty order number", http.StatusBadRequest)
		return
	}

	err = h.orders.Upload(r.Context(), userID, number)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusAccepted)
	case errors.Is(err, domain.ErrOrderOwnedByUser):
		w.WriteHeader(http.StatusOK)
	case errors.Is(err, domain.ErrOrderOwnedByAnother):
		http.Error(w, "order belongs to another user", http.StatusConflict)
	case errors.Is(err, domain.ErrInvalidOrderNumber):
		http.Error(w, "invalid order number", http.StatusUnprocessableEntity)
	default:
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// ListOrders handles GET /api/user/orders.
func (h *Handlers) ListOrders(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	orders, err := h.orders.List(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(orders) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	views := make([]orderView, 0, len(orders))
	for _, o := range orders {
		v := orderView{
			Number:     o.Number,
			Status:     o.Status,
			UploadedAt: o.UploadedAt.Format(time.RFC3339),
		}
		if o.HasAccrual {
			accrual := o.Accrual
			v.Accrual = &accrual
		}
		views = append(views, v)
	}
	writeJSON(w, http.StatusOK, views)
}
