// Package config resolves the service configuration from command-line flags and
// environment variables. Environment variables take precedence over flags, which
// take precedence over the built-in defaults.
package config

import (
	"flag"
	"fmt"
	"os"
	"time"
)

// Config holds every runtime setting of the loyalty service.
type Config struct {
	RunAddress           string
	DatabaseURI          string
	AccrualSystemAddress string

	AccrualPollInterval time.Duration
	JWTSecret           string
	TokenTTL            time.Duration
}

// Default values used when neither a flag nor an environment variable is set.
const (
	defaultRunAddress   = ":8080"
	defaultPollInterval = time.Second
	defaultJWTSecret    = "gophermart-dev-secret"
	defaultTokenTTL     = 24 * time.Hour
)

// Load parses os.Args and the environment and returns the resolved Config.
func Load() (*Config, error) {
	return parse(os.Args[1:], os.LookupEnv)
}

func parse(args []string, lookupEnv func(string) (string, bool)) (*Config, error) {
	cfg := &Config{
		RunAddress:          defaultRunAddress,
		AccrualPollInterval: defaultPollInterval,
		JWTSecret:           defaultJWTSecret,
		TokenTTL:            defaultTokenTTL,
	}

	fs := flag.NewFlagSet("gophermart", flag.ContinueOnError)
	fs.StringVar(&cfg.RunAddress, "a", cfg.RunAddress, "service run address host:port")
	fs.StringVar(&cfg.DatabaseURI, "d", cfg.DatabaseURI, "PostgreSQL connection URI")
	fs.StringVar(&cfg.AccrualSystemAddress, "r", cfg.AccrualSystemAddress, "accrual system base address")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if v, ok := lookupEnv("RUN_ADDRESS"); ok && v != "" {
		cfg.RunAddress = v
	}
	if v, ok := lookupEnv("DATABASE_URI"); ok && v != "" {
		cfg.DatabaseURI = v
	}
	if v, ok := lookupEnv("ACCRUAL_SYSTEM_ADDRESS"); ok && v != "" {
		cfg.AccrualSystemAddress = v
	}
	if v, ok := lookupEnv("JWT_SECRET"); ok && v != "" {
		cfg.JWTSecret = v
	}

	if cfg.DatabaseURI == "" {
		return nil, fmt.Errorf("database connection string is required: set -d or DATABASE_URI")
	}
	if cfg.AccrualSystemAddress == "" {
		return nil, fmt.Errorf("accrual system address is required: set -r or ACCRUAL_SYSTEM_ADDRESS")
	}

	return cfg, nil
}
