package config

import (
	"testing"
	"time"
)

func envFrom(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

// requiredArgs supplies the flags parse() requires (-d and -r) so tests can
// focus on the setting under test.
var requiredArgs = []string{"-d", "postgres://x", "-r", "http://accrual:8081"}

func TestParseDefaults(t *testing.T) {
	cfg, err := parse(requiredArgs, envFrom(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RunAddress != ":8080" {
		t.Fatalf("RunAddress = %q", cfg.RunAddress)
	}
	if cfg.TokenTTL != 24*time.Hour {
		t.Fatalf("TokenTTL = %v", cfg.TokenTTL)
	}
}

func TestParseFlags(t *testing.T) {
	args := []string{"-a", "127.0.0.1:9000", "-d", "postgres://x", "-r", "http://accrual:8081"}
	cfg, err := parse(args, envFrom(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RunAddress != "127.0.0.1:9000" || cfg.DatabaseURI != "postgres://x" || cfg.AccrualSystemAddress != "http://accrual:8081" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

func TestEnvOverridesFlags(t *testing.T) {
	args := []string{"-a", "127.0.0.1:9000", "-d", "flagdb", "-r", "flagaccrual"}
	env := map[string]string{
		"RUN_ADDRESS":            ":7777",
		"DATABASE_URI":           "envdb",
		"ACCRUAL_SYSTEM_ADDRESS": "envaccrual",
	}
	cfg, err := parse(args, envFrom(env))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RunAddress != ":7777" || cfg.DatabaseURI != "envdb" || cfg.AccrualSystemAddress != "envaccrual" {
		t.Fatalf("env did not override flags: %+v", cfg)
	}
}

func TestEmptyEnvDoesNotOverrideFlags(t *testing.T) {
	args := []string{"-a", "127.0.0.1:9000", "-d", "flagdb", "-r", "flagaccrual"}
	env := map[string]string{"RUN_ADDRESS": "", "DATABASE_URI": "", "ACCRUAL_SYSTEM_ADDRESS": ""}
	cfg, err := parse(args, envFrom(env))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RunAddress != "127.0.0.1:9000" || cfg.DatabaseURI != "flagdb" || cfg.AccrualSystemAddress != "flagaccrual" {
		t.Fatalf("empty env var wiped a set flag: %+v", cfg)
	}
}

func TestParseRequiresDatabaseURI(t *testing.T) {
	args := []string{"-r", "http://accrual:8081"}
	if _, err := parse(args, envFrom(nil)); err == nil {
		t.Fatal("want an error when DATABASE_URI/-d is missing")
	}
}

func TestParseRequiresAccrualAddress(t *testing.T) {
	args := []string{"-d", "postgres://x"}
	if _, err := parse(args, envFrom(nil)); err == nil {
		t.Fatal("want an error when ACCRUAL_SYSTEM_ADDRESS/-r is missing")
	}
}
