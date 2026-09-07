package handler_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stepantishhen/gofermart/internal/domain"
	"github.com/stepantishhen/gofermart/internal/httpserver"
	"github.com/stepantishhen/gofermart/internal/httpserver/handler"
	"github.com/stepantishhen/gofermart/internal/service"
	"log/slog"
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

func do(t *testing.T, srv http.Handler, method, path, body string, authed bool) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if authed {
		req.Header.Set("Authorization", "Bearer good")
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec.Result()
}

func TestRegister(t *testing.T) {
	srv := newServer(stubAuth{token: "tok"}, stubOrders{}, stubBalance{})

	resp := do(t, srv, http.MethodPost, "/api/user/register", `{"login":"a","password":"b"}`, false)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if resp.Header.Get("Authorization") == "" {
		t.Fatal("missing Authorization header")
	}

	resp = do(t, srv, http.MethodPost, "/api/user/register", `not json`, false)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad json status = %d", resp.StatusCode)
	}
}

func TestRegisterConflict(t *testing.T) {
	srv := newServer(stubAuth{err: domain.ErrLoginTaken}, stubOrders{}, stubBalance{})
	resp := do(t, srv, http.MethodPost, "/api/user/register", `{"login":"a","password":"b"}`, false)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestLoginUnauthorized(t *testing.T) {
	srv := newServer(stubAuth{err: domain.ErrInvalidCredentials}, stubOrders{}, stubBalance{})
	resp := do(t, srv, http.MethodPost, "/api/user/login", `{"login":"a","password":"b"}`, false)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", resp.StatusCode)
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
			if resp.StatusCode != c.want {
				t.Fatalf("status = %d, want %d", resp.StatusCode, c.want)
			}
		})
	}
}

func TestUploadOrderUnauthorized(t *testing.T) {
	srv := newServer(stubAuth{}, stubOrders{}, stubBalance{})
	resp := do(t, srv, http.MethodPost, "/api/user/orders", "12345678903", false)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestListOrders(t *testing.T) {
	srv := newServer(stubAuth{}, stubOrders{}, stubBalance{})
	resp := do(t, srv, http.MethodGet, "/api/user/orders", "", true)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("empty list status = %d", resp.StatusCode)
	}

	acc := domain.NewMoneyFromFloat(500)
	srv = newServer(stubAuth{}, stubOrders{list: []domain.Order{
		{Number: "9278923470", Status: domain.OrderStatusProcessed, Accrual: acc, HasAccrual: true, UploadedAt: time.Now()},
	}}, stubBalance{})
	resp = do(t, srv, http.MethodGet, "/api/user/orders", "", true)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"accrual":500`) || !strings.Contains(string(body), `"number":"9278923470"`) {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestGetBalance(t *testing.T) {
	srv := newServer(stubAuth{}, stubOrders{}, stubBalance{bal: domain.Balance{
		Current:   domain.NewMoneyFromFloat(500.5),
		Withdrawn: domain.NewMoneyFromFloat(42),
	}})
	resp := do(t, srv, http.MethodGet, "/api/user/balance", "", true)
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"current":500.5`) || !strings.Contains(string(body), `"withdrawn":42`) {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
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
			if resp.StatusCode != c.want {
				t.Fatalf("status = %d, want %d", resp.StatusCode, c.want)
			}
		})
	}
}

func TestListWithdrawals(t *testing.T) {
	srv := newServer(stubAuth{}, stubOrders{}, stubBalance{})
	resp := do(t, srv, http.MethodGet, "/api/user/withdrawals", "", true)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("empty status = %d", resp.StatusCode)
	}

	srv = newServer(stubAuth{}, stubOrders{}, stubBalance{withdrawals: []domain.Withdrawal{
		{OrderNumber: "2377225624", Sum: domain.NewMoneyFromFloat(500), ProcessedAt: time.Now()},
	}})
	resp = do(t, srv, http.MethodGet, "/api/user/withdrawals", "", true)
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"order":"2377225624"`) {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
}

func TestRegisterInvalidInputAndInternal(t *testing.T) {
	srv := newServer(stubAuth{err: service.ErrInvalidInput}, stubOrders{}, stubBalance{})
	if resp := do(t, srv, http.MethodPost, "/api/user/register", `{"login":"a","password":""}`, false); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid input status = %d", resp.StatusCode)
	}

	srv = newServer(stubAuth{err: errors.New("boom")}, stubOrders{}, stubBalance{})
	if resp := do(t, srv, http.MethodPost, "/api/user/register", `{"login":"a","password":"b"}`, false); resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("internal status = %d", resp.StatusCode)
	}
}

func TestLogin(t *testing.T) {
	srv := newServer(stubAuth{token: "tok"}, stubOrders{}, stubBalance{})
	resp := do(t, srv, http.MethodPost, "/api/user/login", `{"login":"a","password":"b"}`, false)
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Authorization") == "" {
		t.Fatalf("status=%d auth=%q", resp.StatusCode, resp.Header.Get("Authorization"))
	}

	if resp := do(t, srv, http.MethodPost, "/api/user/login", `not json`, false); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad json status = %d", resp.StatusCode)
	}

	srv = newServer(stubAuth{err: service.ErrInvalidInput}, stubOrders{}, stubBalance{})
	if resp := do(t, srv, http.MethodPost, "/api/user/login", `{"login":"","password":""}`, false); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid input status = %d", resp.StatusCode)
	}

	srv = newServer(stubAuth{err: errors.New("boom")}, stubOrders{}, stubBalance{})
	if resp := do(t, srv, http.MethodPost, "/api/user/login", `{"login":"a","password":"b"}`, false); resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("internal status = %d", resp.StatusCode)
	}
}

func TestUploadOrderEmptyBody(t *testing.T) {
	srv := newServer(stubAuth{}, stubOrders{}, stubBalance{})
	resp := do(t, srv, http.MethodPost, "/api/user/orders", "", true)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty body status = %d", resp.StatusCode)
	}
}

func TestListOrdersServiceError(t *testing.T) {
	srv := newServer(stubAuth{}, stubOrders{listErr: errors.New("boom")}, stubBalance{})
	resp := do(t, srv, http.MethodGet, "/api/user/orders", "", true)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestGetBalanceServiceError(t *testing.T) {
	srv := newServer(stubAuth{}, stubOrders{}, stubBalance{getErr: errors.New("boom")})
	resp := do(t, srv, http.MethodGet, "/api/user/balance", "", true)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestWithdrawBadJSON(t *testing.T) {
	srv := newServer(stubAuth{}, stubOrders{}, stubBalance{})
	resp := do(t, srv, http.MethodPost, "/api/user/balance/withdraw", `not json`, true)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestListWithdrawalsServiceError(t *testing.T) {
	srv := newServer(stubAuth{}, stubOrders{}, stubBalance{withdrawalsErr: errors.New("boom")})
	resp := do(t, srv, http.MethodGet, "/api/user/withdrawals", "", true)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d", resp.StatusCode)
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
