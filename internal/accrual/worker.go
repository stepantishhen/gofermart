package accrual

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/stepantishhen/gofermart/internal/domain"
)

// PendingStore is the subset of the order repository the worker needs.
type PendingStore interface {
	ListPending(ctx context.Context, limit int) ([]domain.Order, error)
	ApplyAccrual(ctx context.Context, number string, status domain.OrderStatus, accrual domain.Money, hasAccrual bool) error
}

// Fetcher retrieves an accrual result for an order number.
type Fetcher interface {
	Get(ctx context.Context, number string) (Result, error)
}

// Worker periodically polls the accrual service for every pending order and
// writes final results back to the store.
type Worker struct {
	store     PendingStore
	fetcher   Fetcher
	log       *slog.Logger
	interval  time.Duration
	batchSize int
}

// NewWorker builds a Worker.
func NewWorker(store PendingStore, fetcher Fetcher, log *slog.Logger, interval time.Duration, batchSize int) *Worker {
	if batchSize <= 0 {
		batchSize = 100
	}
	return &Worker{store: store, fetcher: fetcher, log: log, interval: interval, batchSize: batchSize}
}

// Run polls until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.pollOnce(ctx)
		}
	}
}

func (w *Worker) pollOnce(ctx context.Context) {
	orders, err := w.store.ListPending(ctx, w.batchSize)
	if err != nil {
		w.log.Error("list pending orders", "error", err)
		return
	}

	for _, o := range orders {
		if ctx.Err() != nil {
			return
		}
		if err := w.process(ctx, o.Number); err != nil {
			var tmr *TooManyRequestsError
			if errors.As(err, &tmr) {
				w.log.Warn("accrual rate limited", "sleep", tmr.RetryAfter)
				select {
				case <-ctx.Done():
				case <-time.After(tmr.RetryAfter):
				}
				return
			}
			w.log.Error("process order", "number", o.Number, "error", err)
		}
	}
}

func (w *Worker) process(ctx context.Context, number string) error {
	res, err := w.fetcher.Get(ctx, number)
	if err != nil {
		if errors.Is(err, ErrOrderNotRegistered) {
			return nil
		}
		return err
	}

	status, final := mapStatus(res.Status)
	if !final && !res.HasAccrual {
		return w.store.ApplyAccrual(ctx, number, status, 0, false)
	}
	return w.store.ApplyAccrual(ctx, number, status, res.Accrual, res.HasAccrual)
}

func mapStatus(s Status) (status domain.OrderStatus, final bool) {
	switch s {
	case StatusInvalid:
		return domain.OrderStatusInvalid, true
	case StatusProcessed:
		return domain.OrderStatusProcessed, true
	default:
		return domain.OrderStatusProcessing, false
	}
}
