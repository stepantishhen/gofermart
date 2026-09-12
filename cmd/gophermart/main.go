// Command gophermart runs the "Гофермарт" cumulative loyalty system HTTP API.
//
// Configuration is taken from flags (-a, -d, -r) and the environment variables
// RUN_ADDRESS, DATABASE_URI and ACCRUAL_SYSTEM_ADDRESS, with the environment
// taking precedence.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/stepantishhen/gofermart/internal/app"
	"github.com/stepantishhen/gofermart/internal/config"
	"github.com/stepantishhen/gofermart/internal/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(2)
	}

	log := logger.New(os.Getenv("LOG_LEVEL"))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, cfg, log); err != nil {
		log.Error("service stopped with error", "error", err)
		os.Exit(1)
	}
}
