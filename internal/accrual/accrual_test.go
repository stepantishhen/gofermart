package accrual

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stepantishhen/gofermart/internal/domain"
)

func TestClientGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/orders/processed":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"order":"processed","status":"PROCESSED","accrual":500}`))
		case "/api/orders/noaccrual":
			_, _ = w.Write([]byte(`{"order":"noaccrual","status":"PROCESSED"}`))
		case "/api/orders/missing":
			w.WriteHeader(http.StatusNoContent)
		case "/api/orders/limited":
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(http.StatusTooManyRequests)
		case "/api/orders/badjson":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"order":`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx := context.Background()

	res, err := c.Get(ctx, "processed")
	if err != nil || res.Status != StatusProcessed || !res.HasAccrual || res.Accrual != domain.NewMoneyFromFloat(500) {
		t.Fatalf("processed: res=%+v err=%v", res, err)
	}

	res, err = c.Get(ctx, "noaccrual")
	if err != nil || res.HasAccrual {
		t.Fatalf("noaccrual: res=%+v err=%v", res, err)
	}

	if _, err := c.Get(ctx, "missing"); !errors.Is(err, ErrOrderNotRegistered) {
		t.Fatalf("want ErrOrderNotRegistered, got %v", err)
	}

	_, err = c.Get(ctx, "limited")
	var tmr *TooManyRequestsError
	if !errors.As(err, &tmr) || tmr.RetryAfter != 3*time.Second {
		t.Fatalf("want TooManyRequestsError(3s), got %v", err)
	}
	if tmr.Error() == "" {
		t.Fatal("TooManyRequestsError.Error() is empty")
	}

	if _, err := c.Get(ctx, "boom"); err == nil {
		t.Fatal("want error on 500")
	}
	if _, err := c.Get(ctx, "badjson"); err == nil {
		t.Fatal("want error on malformed JSON body")
	}
}

func TestClientGetTransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	if _, err := NewClient(url).Get(context.Background(), "123"); err == nil {
		t.Fatal("want transport error against a closed server")
	}
}

func TestClientGetRequestBuildError(t *testing.T) {
	c := NewClient("http://127.0.0.1:0/\x7f\n")
	if _, err := c.Get(context.Background(), "123"); err == nil {
		t.Fatal("want request build error for an invalid URL")
	}
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"", time.Minute},
		{"60", 60 * time.Second},
		{"  30  ", 30 * time.Second},
		{"not-a-number", time.Minute},
	}
	for _, tt := range tests {
		if got := parseRetryAfter(tt.in); got != tt.want {
			t.Fatalf("parseRetryAfter(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// appliedRecord is what fakeStore.ApplyAccrual received for one order, so
// tests can assert not just the resulting status but whether a credit
// actually happened.
type appliedRecord struct {
	status     domain.OrderStatus
	accrual    domain.Money
	hasAccrual bool
}

type fakeStore struct {
	mu       sync.Mutex
	pending  []domain.Order
	applied  map[string]appliedRecord
	listErr  error
	applyErr error
}

func (f *fakeStore) ListPending(context.Context, int) ([]domain.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.pending, nil
}

func (f *fakeStore) ApplyAccrual(_ context.Context, number string, status domain.OrderStatus, accrual domain.Money, hasAccrual bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.applyErr != nil {
		return f.applyErr
	}
	if f.applied == nil {
		f.applied = map[string]appliedRecord{}
	}
	f.applied[number] = appliedRecord{status: status, accrual: accrual, hasAccrual: hasAccrual}
	return nil
}

func (f *fakeStore) appliedStatus(number string) domain.OrderStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.applied[number].status
}

func (f *fakeStore) wasApplied(number string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.applied[number]
	return ok
}

type limitCapturingStore struct {
	fakeStore
	seen chan int
}

func (s *limitCapturingStore) ListPending(_ context.Context, limit int) ([]domain.Order, error) {
	select {
	case s.seen <- limit:
	default:
	}
	return nil, nil
}

type fakeFetcher struct {
	results map[string]Result
	errs    map[string]error
}

func (f fakeFetcher) Get(_ context.Context, number string) (Result, error) {
	if err, ok := f.errs[number]; ok {
		return Result{}, err
	}
	r, ok := f.results[number]
	if !ok {
		return Result{}, ErrOrderNotRegistered
	}
	return r, nil
}

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func TestWorkerPollOnce(t *testing.T) {
	store := &fakeStore{pending: []domain.Order{{Number: "a"}, {Number: "b"}, {Number: "c"}}}
	fetcher := fakeFetcher{results: map[string]Result{
		"a": {Status: StatusProcessed, Accrual: domain.NewMoneyFromFloat(10), HasAccrual: true},
		"b": {Status: StatusInvalid},
		"c": {Status: StatusProcessing},
	}}
	w := NewWorker(store, fetcher, discardLogger(), time.Hour, 10)
	w.pollOnce(context.Background())

	if store.appliedStatus("a") != domain.OrderStatusProcessed {
		t.Fatalf("a = %v", store.appliedStatus("a"))
	}
	if store.appliedStatus("b") != domain.OrderStatusInvalid {
		t.Fatalf("b = %v", store.appliedStatus("b"))
	}
	if store.appliedStatus("c") != domain.OrderStatusProcessing {
		t.Fatalf("c = %v", store.appliedStatus("c"))
	}
}

func TestWorkerPollOnceListError(t *testing.T) {
	store := &fakeStore{listErr: errors.New("db down")}
	w := NewWorker(store, fakeFetcher{}, discardLogger(), time.Hour, 10)
	w.pollOnce(context.Background())
	if len(store.applied) != 0 {
		t.Fatalf("nothing should be applied, got %v", store.applied)
	}
}

func TestWorkerPollOnceRateLimitedStopsBatch(t *testing.T) {
	store := &fakeStore{pending: []domain.Order{{Number: "a"}, {Number: "b"}}}
	fetcher := fakeFetcher{errs: map[string]error{
		"a": &TooManyRequestsError{RetryAfter: time.Millisecond},
	}}
	w := NewWorker(store, fetcher, discardLogger(), time.Hour, 10)

	done := make(chan struct{})
	go func() { w.pollOnce(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pollOnce did not return after rate limit")
	}
	if store.wasApplied("b") {
		t.Fatal("batch should stop at the rate-limited order")
	}
}

func TestWorkerProcessFetchError(t *testing.T) {
	store := &fakeStore{pending: []domain.Order{{Number: "x"}}}
	fetcher := fakeFetcher{errs: map[string]error{"x": errors.New("boom")}}
	w := NewWorker(store, fetcher, discardLogger(), time.Hour, 10)
	w.pollOnce(context.Background())
	if len(store.applied) != 0 {
		t.Fatalf("fetch error must not apply anything, got %v", store.applied)
	}
}

func TestWorkerProcessNotRegistered(t *testing.T) {
	store := &fakeStore{}
	w := NewWorker(store, fakeFetcher{}, discardLogger(), time.Hour, 10)
	if err := w.process(context.Background(), domain.Order{Number: "unknown"}); err != nil {
		t.Fatalf("process = %v, want nil for unregistered order", err)
	}
	if len(store.applied) != 0 {
		t.Fatal("unregistered order must not be applied")
	}
}

func TestWorkerProcessApplyError(t *testing.T) {
	store := &fakeStore{applyErr: errors.New("write failed")}
	fetcher := fakeFetcher{results: map[string]Result{"x": {Status: StatusProcessing}}}
	w := NewWorker(store, fetcher, discardLogger(), time.Hour, 10)
	o := domain.Order{Number: "x", Status: domain.OrderStatusNew}
	if err := w.process(context.Background(), o); err == nil {
		t.Fatal("want the store error to propagate")
	}
}

func TestWorkerProcessSkipsWriteWhenStatusUnchanged(t *testing.T) {
	store := &fakeStore{}
	fetcher := fakeFetcher{results: map[string]Result{"x": {Status: StatusProcessing}}}
	w := NewWorker(store, fetcher, discardLogger(), time.Hour, 10)

	o := domain.Order{Number: "x", Status: domain.OrderStatusProcessing}
	if err := w.process(context.Background(), o); err != nil {
		t.Fatalf("process = %v", err)
	}
	if store.wasApplied("x") {
		t.Fatal("no write should happen when the accrual status has not changed")
	}
}

func TestWorkerProcessOnlyCreditsOnFinalProcessed(t *testing.T) {
	store := &fakeStore{}
	fetcher := fakeFetcher{results: map[string]Result{
		// A non-final status carrying an accrual should be written (status
		// changed) but never credited.
		"x": {Status: StatusProcessing, Accrual: domain.NewMoneyFromFloat(50), HasAccrual: true},
	}}
	w := NewWorker(store, fetcher, discardLogger(), time.Hour, 10)

	o := domain.Order{Number: "x", Status: domain.OrderStatusNew}
	if err := w.process(context.Background(), o); err != nil {
		t.Fatalf("process = %v", err)
	}
	rec, ok := store.applied["x"]
	if !ok {
		t.Fatal("the NEW -> PROCESSING transition must still be written")
	}
	if rec.hasAccrual {
		t.Fatal("accrual must not be credited before the order is PROCESSED")
	}
}

func TestWorkerRunStopsOnContextCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := &fakeStore{pending: []domain.Order{{Number: "a"}}}
		fetcher := fakeFetcher{results: map[string]Result{
			"a": {Status: StatusProcessed, Accrual: domain.NewMoneyFromFloat(5), HasAccrual: true},
		}}
		w := NewWorker(store, fetcher, discardLogger(), time.Second, 10)

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			w.Run(ctx)
			close(done)
		}()

		// The fake clock only advances once every goroutine is durably
		// blocked, so this fires the worker's first tick deterministically
		// instead of racing a real timer.
		time.Sleep(time.Second)
		synctest.Wait()

		if store.appliedStatus("a") != domain.OrderStatusProcessed {
			t.Fatal("worker never processed the order")
		}

		cancel()
		synctest.Wait()

		select {
		case <-done:
		default:
			t.Fatal("Run did not stop after context cancel")
		}
	})
}

func TestNewWorkerDefaultsBatchSize(t *testing.T) {
	store := &limitCapturingStore{seen: make(chan int, 1)}
	w := NewWorker(store, fakeFetcher{}, discardLogger(), time.Hour, 0)
	w.pollOnce(context.Background())
	select {
	case got := <-store.seen:
		if got != 100 {
			t.Fatalf("batch size = %d, want default 100", got)
		}
	default:
		t.Fatal("ListPending was not called")
	}
}
