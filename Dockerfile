# Single-service image (方案 1): build the Vue frontend, build the Go backend,
# then run one Go process that serves BOTH the static SPA and the /api routes.
# Works as-is on Render / Railway / Fly.io / Google Cloud Run.

# ---- 1. Build the frontend ----
FROM node:20-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build            # -> /app/frontend/dist

# ---- 2. Build the backend (static binary) ----
FROM golang:1.23-alpine AS backend
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /server ./cmd/server

# ---- 3. Minimal runtime ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
WORKDIR /app
COPY --from=backend --chown=app:app /server /app/server
COPY --from=frontend --chown=app:app /app/frontend/dist /app/web

ENV GIN_MODE=release \
    WEB_DIR=/app/web \
    PORT=8080
EXPOSE 8080
USER app

# Secrets (Apple .p8 via APPLE_PRIVATE_KEY, etc.) are injected at runtime as env
# vars by the hosting platform — never baked into the image.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
  CMD wget -qO- "http://127.0.0.1:${PORT:-8080}/api/health" >/dev/null 2>&1 || exit 1

CMD ["/app/server"]
