#!/bin/bash

# LWN-Sim-Plus Start Script

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

BINARY_NAME="lwnsimulator"
PID_FILE="/tmp/lwnsimulator.pid"
LOG_FILE="simulator.log"

echo "Starting LWN-Sim-Plus..."

# Check if already running
if [ -f "$PID_FILE" ]; then
    PID=$(cat "$PID_FILE")
    if kill -0 "$PID" 2>/dev/null; then
        echo "✗ Simulator is already running with PID $PID"
        echo "  Use ./stop.sh to stop it first, or ./restart.sh to restart"
        exit 1
    else
        rm -f "$PID_FILE"
    fi
fi

# Check if binary exists
if [ ! -f "bin/$BINARY_NAME" ]; then
    echo "Binary not found. Building..."
    export PATH=$PATH:~/go/bin
    make build
fi

# Start the simulator
nohup "./bin/$BINARY_NAME" > "$LOG_FILE" 2>&1 &
PID=$!
echo $PID > "$PID_FILE"

sleep 2
if kill -0 "$PID" 2>/dev/null; then
    echo "✓ Simulator started with PID $PID"
    echo ""
    echo "Logs: tail -f $LOG_FILE"
    echo "Stop: ./stop.sh"
    echo ""
    echo "First few lines of output:"
    echo "---"
    head -20 "$LOG_FILE" || true
else
    echo "✗ Failed to start simulator"
    echo "Check logs at $LOG_FILE"
    rm -f "$PID_FILE"
    exit 1
fi
