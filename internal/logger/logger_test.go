package logger

import (
	"context"
	"log/slog"
	"testing"
)

func TestNewLevels(t *testing.T) {
	for _, lvl := range []string{"debug", "info", "warn", "error", "unknown", ""} {
		if New(lvl) == nil {
			t.Fatalf("New(%q) returned nil", lvl)
		}
	}
}

func TestNewDebugEnablesDebug(t *testing.T) {
	if !New("debug").Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("debug level not enabled")
	}
	if New("error").Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("info should be disabled at error level")
	}
}
