#!/bin/sh
set -eu

trap 'kill $(jobs -p) 2>/dev/null || true; exit 0' TERM INT

mkdir -p /app/client/backend/data
BACKEND_PORT="${BACKEND_PORT:-8080}"
FRONTEND_PORT="${FRONTEND_PORT:-3000}"

(
  while true; do
    (cd /app/client/backend && /app/backend/backend) || true
    sleep 2
  done
) &

(
  cd /app/client/frontend
  while true; do
    npx vite preview --host 0.0.0.0 --port "${FRONTEND_PORT}" || true
    sleep 2
  done
) &

wait
