#!/bin/bash

# LWN-Sim-Plus Stop Script

BINARY_NAME="lwnsimulator"
PID_FILE="/tmp/lwnsimulator.pid"

echo "Stopping LWN-Sim-Plus..."

STOPPED=false

# Try to stop using PID file
if [ -f "$PID_FILE" ]; then
    PID=$(cat "$PID_FILE")
    if kill -0 "$PID" 2>/dev/null; then
        echo "Stopping process with PID $PID..."
        kill "$PID" 2>/dev/null || true
        sleep 2

        # Force kill if still running
        if kill -0 "$PID" 2>/dev/null; then
            echo "Process still running, force killing..."
            kill -9 "$PID" 2>/dev/null || true
        fi
        STOPPED=true
    fi
    rm -f "$PID_FILE"
fi

# Fallback: kill by process name
PIDS=$(pgrep -f "$BINARY_NAME" || true)
if [ -n "$PIDS" ]; then
    echo "Found additional processes: $PIDS"
    pkill -f "$BINARY_NAME" || true
    sleep 2

    # Force kill if still running
    pkill -9 -f "$BINARY_NAME" 2>/dev/null || true
    STOPPED=true
fi

if [ "$STOPPED" = true ]; then
    echo "✓ Simulator stopped"
else
    echo "✓ No running simulator found"
fi
