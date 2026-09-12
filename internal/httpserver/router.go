// Package httpserver wires the API handlers and middleware into an http.Handler.
package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/stepantishhen/gofermart/internal/httpserver/handler"
	"github.com/stepantishhen/gofermart/internal/httpserver/middleware"
)

// NewRouter builds the API router: public auth routes plus authenticated
// /api/user routes, with gzip and request logging applied to everything. All
// routes are static, so the stdlib ServeMux (Go 1.22+ method-and-path
// patterns) is enough and needs no third-party router.
func NewRouter(h *handler.Handlers, parser middleware.TokenParser, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/user/register", h.Register)
	mux.HandleFunc("POST /api/user/login", h.Login)

	auth := middleware.Auth(parser)
	mux.Handle("POST /api/user/orders", auth(http.HandlerFunc(h.UploadOrder)))
	mux.Handle("GET /api/user/orders", auth(http.HandlerFunc(h.ListOrders)))
	mux.Handle("GET /api/user/balance", auth(http.HandlerFunc(h.GetBalance)))
	mux.Handle("POST /api/user/balance/withdraw", auth(http.HandlerFunc(h.Withdraw)))
	mux.Handle("GET /api/user/withdrawals", auth(http.HandlerFunc(h.ListWithdrawals)))

	return middleware.Logging(log)(middleware.Gzip(mux))
}
