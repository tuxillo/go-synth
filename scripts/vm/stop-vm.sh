#!/bin/bash
# Stop DragonFlyBSD VM
set -euo pipefail

# Load VM configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/config.sh"

PID_FILE="${VM_DIR}/vm.pid"

if [ ! -f "${PID_FILE}" ]; then
    echo "✓ VM not running"
    exit 0
fi

PID=$(cat "${PID_FILE}")
if ! kill -0 "${PID}" 2>/dev/null; then
    echo "✓ VM not running (stale PID file)"
    rm "${PID_FILE}"
    exit 0
fi

echo "🛑 Stopping VM (PID: ${PID})..."
kill "${PID}"
sleep 2

# Force kill if still running
if kill -0 "${PID}" 2>/dev/null; then
    echo "   Force stopping..."
    kill -9 "${PID}" 2>/dev/null || true
    sleep 1
fi

rm -f "${PID_FILE}"
echo "✓ VM stopped"
