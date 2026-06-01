.PHONY: help dev-backend dev-frontend build test backend-test frontend-build install

help:
	@echo "Targets:"
	@echo "  install        install frontend deps (go deps fetch on build)"
	@echo "  dev-backend    run the Go API on :8080"
	@echo "  dev-frontend   run the Vite dev server on :5173"
	@echo "  test           run all tests (backend) + type-check (frontend)"
	@echo "  build          build backend binary + frontend dist"

install:
	cd frontend && npm install

dev-backend:
	cd backend && go run ./cmd/server

dev-frontend:
	cd frontend && npm run dev

backend-test:
	cd backend && go vet ./... && go test ./...

frontend-build:
	cd frontend && npm run type-check && npm run build

test: backend-test
	cd frontend && npm run type-check

build:
	cd backend && go build -o bin/server ./cmd/server
	cd frontend && npm run build
