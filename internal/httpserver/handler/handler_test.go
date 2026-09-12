package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"log/slog"

	"github.com/stepantishhen/gofermart/internal/domain"
	"github.com/stepantishhen/gofermart/internal/httpserver"
	"github.com/stepantishhen/gofermart/internal/httpserver/handler"
	"github.com/stepantishhen/gofermart/internal/service"
)

type stubAuth struct {
	token string
	err   error
}

func (s stubAuth) Register(context.Context, string, string) (string, error) { return s.token, s.err }
func (s stubAuth) Login(context.Context, string, string) (string, error)    { return s.token, s.err }

type stubOrders struct {
	uploadErr error
	list      []domain.Order
	listErr   error
}

func (s stubOrders) Upload(context.Context, int64, string) error { return s.uploadErr }
func (s stubOrders) List(context.Context, int64) ([]domain.Order, error) {
	return s.list, s.listErr
}

type stubBalance struct {
	bal            domain.Balance
	getErr         error
	withdrawErr    error
	withdrawals    []domain.Withdrawal
	withdrawalsErr error
}

func (s stubBalance) Get(context.Context, int64) (domain.Balance, error) { return s.bal, s.getErr }
func (s stubBalance) Withdraw(context.Context, int64, string, domain.Money) error {
	return s.withdrawErr
}
func (s stubBalance) Withdrawals(context.Context, int64) ([]domain.Withdrawal, error) {
	return s.withdrawals, s.withdrawalsErr
}

type stubParser struct{}

func (stubParser) Parse(token string) (int64, error) {
	if token == "good" {
		return 7, nil
	}
	return 0, errors.New("bad token")
}

func newServer(a handler.AuthService, o handler.OrderService, b handler.BalanceService) http.Handler {
	h := handler.New(a, o, b, time.Hour)
	return httpserver.NewRouter(h, stubParser{}, slog.New(slog.DiscardHandler))
}

// do drives one request through srv and returns the recorder. Using the
// recorder directly (not rec.Result()) keeps the test free of an unclosed
// *http.Response body.
func do(t *testing.T, srv http.Handler, method, path, body string, authed bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if authed {
		req.Header.Set("Authorization", "Bearer good")
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func TestRegister(t *testing.T) {
	srv := newServer(stubAuth{token: "tok"}, stubOrders{}, stubBalance{})

	resp := do(t, srv, http.MethodPost, "/api/user/register", `{"login":"a","password":"b"}`, false)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d", resp.Code)
	}
	if resp.Header().Get("Authorization") == "" {
		t.Fatal("missing Authorization header")
	}

	resp = do(t, srv, http.MethodPost, "/api/user/register", `not json`, false)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("bad json status = %d", resp.Code)
	}
}

func TestRegisterConflict(t *testing.T) {
	srv := newServer(stubAuth{err: domain.ErrLoginTaken}, stubOrders{}, stubBalance{})
	resp := do(t, srv, http.MethodPost, "/api/user/register", `{"login":"a","password":"b"}`, false)
	if resp.Code != http.StatusConflict {
		t.Fatalf("status = %d", resp.Code)
	}
}

func TestLoginUnauthorized(t *testing.T) {
	srv := newServer(stubAuth{err: domain.ErrInvalidCredentials}, stubOrders{}, stubBalance{})
	resp := do(t, srv, http.MethodPost, "/api/user/login", `{"login":"a","password":"b"}`, false)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", resp.Code)
	}
}

func TestUploadOrder(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"accepted", nil, http.StatusAccepted},
		{"already by user", domain.ErrOrderOwnedByUser, http.StatusOK},
		{"by another", domain.ErrOrderOwnedByAnother, http.StatusConflict},
		{"invalid number", domain.ErrInvalidOrderNumber, http.StatusUnprocessableEntity},
		{"internal", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := newServer(stubAuth{}, stubOrders{uploadErr: c.err}, stubBalance{})
			resp := do(t, srv, http.MethodPost, "/api/user/orders", "12345678903", true)
			if resp.Code != c.want {
				t.Fatalf("status = %d, want %d", resp.Code, c.want)
			}
		})
	}
}

func TestUploadOrderUnauthorized(t *testing.T) {
	srv := newServer(stubAuth{}, stubOrders{}, stubBalance{})
	resp := do(t, srv, http.MethodPost, "/api/user/orders", "12345678903", false)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", resp.Code)
	}
}

func TestListOrders(t *testing.T) {
	srv := newServer(stubAuth{}, stubOrders{}, stubBalance{})
	resp := do(t, srv, http.MethodGet, "/api/user/orders", "", true)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("empty list status = %d", resp.Code)
	}

	acc := domain.NewMoneyFromFloat(500)
	srv = newServer(stubAuth{}, stubOrders{list: []domain.Order{
		{Number: "9278923470", Status: domain.OrderStatusProcessed, Accrual: acc, HasAccrual: true, UploadedAt: time.Now()},
	}}, stubBalance{})
	resp = do(t, srv, http.MethodGet, "/api/user/orders", "", true)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d", resp.Code)
	}
	body := resp.Body.String()
	if !strings.Contains(body, `"accrual":500`) || !strings.Contains(body, `"number":"9278923470"`) {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestGetBalance(t *testing.T) {
	srv := newServer(stubAuth{}, stubOrders{}, stubBalance{bal: domain.Balance{
		Current:   domain.NewMoneyFromFloat(500.5),
		Withdrawn: domain.NewMoneyFromFloat(42),
	}})
	resp := do(t, srv, http.MethodGet, "/api/user/balance", "", true)
	body := resp.Body.String()
	if resp.Code != http.StatusOK || !strings.Contains(body, `"current":500.5`) || !strings.Contains(body, `"withdrawn":42`) {
		t.Fatalf("status=%d body=%s", resp.Code, body)
	}
}

func TestWithdraw(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"ok", nil, http.StatusOK},
		{"insufficient", domain.ErrInsufficientBalance, http.StatusPaymentRequired},
		{"bad order", domain.ErrInvalidOrderNumber, http.StatusUnprocessableEntity},
		{"bad input", service.ErrInvalidInput, http.StatusUnprocessableEntity},
		{"internal", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := newServer(stubAuth{}, stubOrders{}, stubBalance{withdrawErr: c.err})
			resp := do(t, srv, http.MethodPost, "/api/user/balance/withdraw", `{"order":"2377225624","sum":10}`, true)
			if resp.Code != c.want {
				t.Fatalf("status = %d, want %d", resp.Code, c.want)
			}
		})
	}
}

func TestListWithdrawals(t *testing.T) {
	srv := newServer(stubAuth{}, stubOrders{}, stubBalance{})
	resp := do(t, srv, http.MethodGet, "/api/user/withdrawals", "", true)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("empty status = %d", resp.Code)
	}

	srv = newServer(stubAuth{}, stubOrders{}, stubBalance{withdrawals: []domain.Withdrawal{
		{OrderNumber: "2377225624", Sum: domain.NewMoneyFromFloat(500), ProcessedAt: time.Now()},
	}})
	resp = do(t, srv, http.MethodGet, "/api/user/withdrawals", "", true)
	body := resp.Body.String()
	if resp.Code != http.StatusOK || !strings.Contains(body, `"order":"2377225624"`) {
		t.Fatalf("status=%d body=%s", resp.Code, body)
	}
}

func TestRegisterInvalidInputAndInternal(t *testing.T) {
	srv := newServer(stubAuth{err: service.ErrInvalidInput}, stubOrders{}, stubBalance{})
	if resp := do(t, srv, http.MethodPost, "/api/user/register", `{"login":"a","password":""}`, false); resp.Code != http.StatusBadRequest {
		t.Fatalf("invalid input status = %d", resp.Code)
	}

	srv = newServer(stubAuth{err: errors.New("boom")}, stubOrders{}, stubBalance{})
	if resp := do(t, srv, http.MethodPost, "/api/user/register", `{"login":"a","password":"b"}`, false); resp.Code != http.StatusInternalServerError {
		t.Fatalf("internal status = %d", resp.Code)
	}
}

func TestLogin(t *testing.T) {
	srv := newServer(stubAuth{token: "tok"}, stubOrders{}, stubBalance{})
	resp := do(t, srv, http.MethodPost, "/api/user/login", `{"login":"a","password":"b"}`, false)
	if resp.Code != http.StatusOK || resp.Header().Get("Authorization") == "" {
		t.Fatalf("status=%d auth=%q", resp.Code, resp.Header().Get("Authorization"))
	}

	if resp := do(t, srv, http.MethodPost, "/api/user/login", `not json`, false); resp.Code != http.StatusBadRequest {
		t.Fatalf("bad json status = %d", resp.Code)
	}

	srv = newServer(stubAuth{err: service.ErrInvalidInput}, stubOrders{}, stubBalance{})
	if resp := do(t, srv, http.MethodPost, "/api/user/login", `{"login":"","password":""}`, false); resp.Code != http.StatusBadRequest {
		t.Fatalf("invalid input status = %d", resp.Code)
	}

	srv = newServer(stubAuth{err: errors.New("boom")}, stubOrders{}, stubBalance{})
	if resp := do(t, srv, http.MethodPost, "/api/user/login", `{"login":"a","password":"b"}`, false); resp.Code != http.StatusInternalServerError {
		t.Fatalf("internal status = %d", resp.Code)
	}
}

func TestUploadOrderEmptyBody(t *testing.T) {
	srv := newServer(stubAuth{}, stubOrders{}, stubBalance{})
	resp := do(t, srv, http.MethodPost, "/api/user/orders", "", true)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("empty body status = %d", resp.Code)
	}
}

func TestListOrdersServiceError(t *testing.T) {
	srv := newServer(stubAuth{}, stubOrders{listErr: errors.New("boom")}, stubBalance{})
	resp := do(t, srv, http.MethodGet, "/api/user/orders", "", true)
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", resp.Code)
	}
}

func TestGetBalanceServiceError(t *testing.T) {
	srv := newServer(stubAuth{}, stubOrders{}, stubBalance{getErr: errors.New("boom")})
	resp := do(t, srv, http.MethodGet, "/api/user/balance", "", true)
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", resp.Code)
	}
}

func TestWithdrawBadJSON(t *testing.T) {
	srv := newServer(stubAuth{}, stubOrders{}, stubBalance{})
	resp := do(t, srv, http.MethodPost, "/api/user/balance/withdraw", `not json`, true)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.Code)
	}
}

func TestListWithdrawalsServiceError(t *testing.T) {
	srv := newServer(stubAuth{}, stubOrders{}, stubBalance{withdrawalsErr: errors.New("boom")})
	resp := do(t, srv, http.MethodGet, "/api/user/withdrawals", "", true)
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", resp.Code)
	}
}

func TestHandlersRejectMissingUserContext(t *testing.T) {
	h := handler.New(stubAuth{}, stubOrders{}, stubBalance{}, time.Hour)
	calls := map[string]http.HandlerFunc{
		"UploadOrder":     h.UploadOrder,
		"ListOrders":      h.ListOrders,
		"GetBalance":      h.GetBalance,
		"Withdraw":        h.Withdraw,
		"ListWithdrawals": h.ListWithdrawals,
	}
	for name, fn := range calls {
		rec := httptest.NewRecorder()
		fn(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s without user in context: code = %d, want 401", name, rec.Code)
		}
	}
}
