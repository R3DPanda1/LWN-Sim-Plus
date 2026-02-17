package logging

import (
	"log/slog"
	"os"
)

type Config struct {
	Level string `json:"level"` // trace, debug, info, warn, error
	JSON  bool   `json:"json"`  // true for K8s, false for local dev
}

func Setup(cfg Config) {
	var level slog.Level
	switch cfg.Level {
	case "debug", "trace":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if cfg.JSON {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}
