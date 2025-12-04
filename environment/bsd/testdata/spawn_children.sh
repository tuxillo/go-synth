#!/bin/sh
# spawn_children.sh - Test helper that spawns background child processes
#
# This script is used by procctl integration tests to simulate a build
# process that spawns multiple child processes. It creates a process
# tree that should all be killed when procctl reaping is invoked.
#
# Usage: spawn_children.sh <num_children> <sleep_duration>
#
# Args:
#   num_children: Number of child processes to spawn (default: 3)
#   sleep_duration: How long children should sleep (default: 9999)
#
# The script:
#   1. Spawns N child processes in background
#   2. Each child sleeps for a long time
#   3. Parent waits for children (or exits if killed)
#
# Test scenario:
#   - Execute this script in chroot environment
#   - Kill parent with cleanup
#   - Verify ALL children are reaped (none survive)

NUM_CHILDREN="${1:-3}"
SLEEP_DURATION="${2:-9999}"

echo "[spawn_children] Starting with PID $$ (parent)"
echo "[spawn_children] Will spawn $NUM_CHILDREN children, each sleeping ${SLEEP_DURATION}s"

# Spawn child processes
for i in $(seq 1 "$NUM_CHILDREN"); do
    (
        echo "[spawn_children] Child $i started with PID $$"
        sleep "$SLEEP_DURATION"
        echo "[spawn_children] Child $i exiting (should not happen in test)"
    ) &
    CHILD_PID=$!
    echo "[spawn_children] Spawned child $i with PID $CHILD_PID"
done

echo "[spawn_children] Parent waiting for children..."
wait
echo "[spawn_children] Parent exiting (should not happen in test)"
