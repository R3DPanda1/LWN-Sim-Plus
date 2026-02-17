package shared

import "log/slog"

// Verbose flag
var Verbose bool = false

// Version of the simulator
const Version = "1.0.3"

func DebugPrint(msg string) {
	if Verbose {
		slog.Debug(msg)
	}
}
