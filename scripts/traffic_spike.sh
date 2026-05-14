#!/usr/bin/env bash
set -euo pipefail

# Purpose: run a staged Locust spike to demonstrate scaling behavior during the review.

TARGET_URL="${TARGET_URL:-http://localhost:8080}"
RUN_TIME="${RUN_TIME:-90s}"
SPAWN_RATE="${SPAWN_RATE:-10}"

stages=(
  "10"
  "25"
  "50"
  "75"
)

for users in "${stages[@]}"; do
  echo "==> Running spike stage with ${users} users against ${TARGET_URL}"
  locust -f loadtesting/locustfile.py \
    --headless \
    --host "${TARGET_URL}" \
    --users "${users}" \
    --spawn-rate "${SPAWN_RATE}" \
    --run-time "${RUN_TIME}" \
    --stop-timeout 10
done
