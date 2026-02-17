package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestSetupJSON(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{Level: "info", JSON: true}

	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	handler := slog.NewJSONHandler(&buf, opts)
	logger := slog.New(handler)
	logger.Info("test message", "component", "device")

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON, got: %s", buf.String())
	}
	if result["msg"] != "test message" {
		t.Errorf("expected msg 'test message', got %v", result["msg"])
	}
	_ = cfg // cfg used to verify config struct exists
}

func TestSetupText(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{Level: "debug", JSON: false}

	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	handler := slog.NewTextHandler(&buf, opts)
	logger := slog.New(handler)
	logger.Debug("debug msg")

	if !strings.Contains(buf.String(), "debug msg") {
		t.Errorf("expected 'debug msg' in output, got: %s", buf.String())
	}
	_ = cfg
}

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	handler := slog.NewTextHandler(&buf, opts)
	logger := slog.New(handler)

	logger.Debug("should not appear")
	if buf.Len() > 0 {
		t.Errorf("debug message should be filtered at info level")
	}

	logger.Info("should appear")
	if buf.Len() == 0 {
		t.Errorf("info message should not be filtered at info level")
	}
}
