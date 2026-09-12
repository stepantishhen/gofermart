// Package app is the composition root: it builds every component from the
// configuration and runs the HTTP server together with the accrual worker until
// the context is cancelled.
package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/stepantishhen/gofermart/internal/accrual"
	"github.com/stepantishhen/gofermart/internal/auth"
	"github.com/stepantishhen/gofermart/internal/config"
	"github.com/stepantishhen/gofermart/internal/httpserver"
	"github.com/stepantishhen/gofermart/internal/httpserver/handler"
	"github.com/stepantishhen/gofermart/internal/repository/postgres"
	"github.com/stepantishhen/gofermart/internal/service"
)

const (
	shutdownTimeout  = 5 * time.Second
	accrualBatchSize = 100
)

// Run starts the service and blocks until ctx is cancelled or a fatal error
// occurs.
func Run(ctx context.Context, cfg *config.Config, log *slog.Logger) error {
	if err := postgres.Migrate(cfg.DatabaseURI); err != nil {
		return err
	}
	pool, err := postgres.Connect(ctx, cfg.DatabaseURI)
	if err != nil {
		return err
	}
	defer pool.Close()
	storage := postgres.New(pool)

	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.TokenTTL)
	authSvc := service.NewAuth(storage.Users, tokens)
	orderSvc := service.NewOrder(storage.Orders)
	balanceSvc := service.NewBalance(storage.Balances, storage.Withdrawals)

	h := handler.New(authSvc, orderSvc, balanceSvc, cfg.TokenTTL)
	router := httpserver.NewRouter(h, tokens, log)

	worker := accrual.NewWorker(
		storage.Orders,
		accrual.NewClient(cfg.AccrualSystemAddress),
		log,
		cfg.AccrualPollInterval,
		accrualBatchSize,
	)

	srv := &http.Server{Addr: cfg.RunAddress, Handler: router}

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		log.Info("http server started", "addr", cfg.RunAddress)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	g.Go(func() error {
		worker.Run(gctx)
		return nil
	})

	g.Go(func() error {
		<-gctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		log.Info("shutting down")
		return srv.Shutdown(shutdownCtx)
	})

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
