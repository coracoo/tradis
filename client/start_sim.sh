#!/bin/sh
set -eu

trap 'kill $(jobs -p) 2>/dev/null || true; exit 0' TERM INT

mkdir -p /app/client/backend/data
BACKEND_PORT="${BACKEND_PORT:-8080}"
FRONTEND_PORT="${FRONTEND_PORT:-3000}"
export BACKEND_PORT
export FRONTEND_PORT
MODE="${VITE_MANAGEMENT_MODE:-CS}"
# 生成运行时环境配置文件（Nginx与Vite Preview均可读取）
mkdir -p /usr/share/nginx/html || true
mkdir -p /app/client/frontend/dist || true
cat > /usr/share/nginx/html/env.js <<EOF
window.__ENV__ = { MANAGEMENT_MODE: "${MODE}" };
EOF
cat > /app/client/frontend/dist/env.js <<EOF
window.__ENV__ = { MANAGEMENT_MODE: "${MODE}" };
EOF
echo "Runtime MANAGEMENT_MODE=${MODE}"

echo "Starting with BACKEND_PORT=$BACKEND_PORT, FRONTEND_PORT=$FRONTEND_PORT"

(
  while true; do
    echo "Starting backend..."
    (cd /app/client/backend && /app/backend/backend) || true
    sleep 2
  done
) &

(
  if command -v npx >/dev/null 2>&1 && [ -d "/app/client/frontend" ]; then
    cd /app/client/frontend
    while true; do
      npx vite preview --host 0.0.0.0 --port "${FRONTEND_PORT}" || true
      sleep 2
    done
  elif command -v nginx >/dev/null 2>&1; then
    [ -d /usr/share/nginx/html ] || mkdir -p /usr/share/nginx/html
    mkdir -p /etc/nginx/http.d
    cat > /etc/nginx/http.d/default.conf <<EOF
server {
    listen ${FRONTEND_PORT};
    server_name _;
    root /usr/share/nginx/html;
    index index.html;
    location / {
        try_files \$uri \$uri/ /index.html;
    }
    location /api {
        proxy_pass http://127.0.0.1:${BACKEND_PORT}/api;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_http_version 1.1;
    }
}
EOF
    nginx -g 'daemon off;'
  elif command -v busybox >/dev/null 2>&1; then
    ROOT="/usr/share/nginx/html"
    [ -d "$ROOT" ] || ROOT="/app/client/frontend/dist"
    busybox httpd -f -p "${FRONTEND_PORT}" -h "$ROOT"
  else
    if command -v python3 >/dev/null 2>&1; then
      cd /usr/share/nginx/html 2>/dev/null || cd /app/client/frontend/dist 2>/dev/null || cd /app
      python3 -m http.server "${FRONTEND_PORT}"
    else
      while true; do sleep 3600; done
    fi
  fi
) &

wait
