// Package httpserver wires the API handlers and middleware into an http.Handler.
package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/stepantishhen/gofermart/internal/httpserver/handler"
	"github.com/stepantishhen/gofermart/internal/httpserver/middleware"
)

// NewRouter builds the API router: public auth routes plus authenticated
// /api/user routes, with gzip and request logging applied to everything.
func NewRouter(h *handler.Handlers, parser middleware.TokenParser, log *slog.Logger) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logging(log))
	r.Use(middleware.Gzip)

	r.Route("/api/user", func(r chi.Router) {
		r.Post("/register", h.Register)
		r.Post("/login", h.Login)

		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(parser))
			r.Post("/orders", h.UploadOrder)
			r.Get("/orders", h.ListOrders)
			r.Get("/balance", h.GetBalance)
			r.Post("/balance/withdraw", h.Withdraw)
			r.Get("/withdrawals", h.ListWithdrawals)
		})
	})

	return r
}
