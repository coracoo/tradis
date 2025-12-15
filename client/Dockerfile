FROM golang:1.22-alpine AS backend-builder
ARG http_proxy
ARG https_proxy
ENV HTTP_PROXY=$http_proxy
ENV http_proxy=$http_proxy
ENV HTTPS_PROXY=$https_proxy
ENV https_proxy=$https_proxy
RUN apk add --no-cache build-base sqlite-dev
ENV CGO_ENABLED=1
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend ./
RUN go build -o /app/bin/backend ./cmd/main.go

FROM node:20-alpine
ARG http_proxy
ARG https_proxy
ENV HTTP_PROXY=$http_proxy
ENV http_proxy=$http_proxy
ENV HTTPS_PROXY=$https_proxy
ENV https_proxy=$https_proxy
ENV BACKEND_PORT=8080 FRONTEND_PORT=3000
RUN apk add --no-cache tini netcat-openbsd sqlite-libs docker-cli docker-cli-compose
WORKDIR /app
COPY --from=backend-builder /app/bin/backend /app/backend/backend
COPY frontend/package*.json /app/client/frontend/
WORKDIR /app/client/frontend
RUN npm ci
COPY frontend /app/client/frontend
RUN npm run build
WORKDIR /app
COPY start.sh /app/start.sh
RUN chmod +x /app/start.sh
VOLUME ["/app/client/backend/data"]
VOLUME ["/var/run/docker.sock"]
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 CMD sh -c 'nc -z 127.0.0.1 ${BACKEND_PORT:-8080} && nc -z 127.0.0.1 ${FRONTEND_PORT:-3000}'
ENTRYPOINT ["/sbin/tini","-g","--"]
CMD ["/bin/sh","/app/start.sh"]
