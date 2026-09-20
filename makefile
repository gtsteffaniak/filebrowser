# Use bash on all platforms
# Windows users: Requires Git Bash (comes with Git for Windows: https://git-scm.com/download/win)
# Add C:\Program Files\Git\bin to your PATH, or run make from Git Bash terminal
ifeq ($(OS),Windows_NT)
    SHELL := C:/Program Files/Git/bin/bash.exe
    .SHELLFLAGS := -ec
else
    SHELL := /bin/bash
endif

PLAYWRIGHT_TEST ?= "settings"
# Path to the dev-tools module, relative to backend/ (see backend/build/go.mod).
BACKEND_BUILD := build
# Resolves a dev-tool binary from backend/build (e.g. $(call backend_dev_tool,air)).
backend_dev_tool = $$(cd $(BACKEND_BUILD) && go tool -n $(1))

# git checkout remote branch PR
# git fetch origin pull/####/head:pr-####

.SILENT:

.PHONY: setup update build build-docker build-backend build-frontend dev run generate-docs
.PHONY: lint-frontend lint-backend lint test test-backend test-frontend check-all
.PHONY: check-translations sync-translations test-playwright test-playwright-performance playwright-base playwright-perf-base perf-check perf-smoke perf-baseline perf-dashboard run-proxy screenshots
.PHONY: check-icons generate-icons sync-icons setup-gofitz-cgo

setup:
	echo "creating ./backend/test_config.yaml for local testing..."
	if [ ! -f backend/test_config.yaml ]; then \
		cp backend/config.yaml backend/test_config.yaml; \
	fi
	$(MAKE) setup-gofitz-cgo
	echo "installing backend dev tooling..."
	cd backend/build && go get tool
	cd backend/internal/web && mkdir -p embed dist && touch embed/.gitignore
	echo "installing npm requirements for frontend..."
	cd frontend && npm i

setup-gofitz-cgo:
	echo "linking go-fitz MuPDF headers for preview CGO..."
	cd backend && go run ./scripts/setup-gofitz-cgo

update:
	cd backend && go get -u ./... && go mod tidy
	cd backend/build && go get -u tool && go mod tidy
	cd frontend && npm update

build: build-frontend build-backend

build-docker:
	docker build --build-arg="VERSION=testing" --build-arg="REVISION=n/a" -t gtstef/filebrowser -f _docker/Dockerfile .

build-docker-slim:
	docker build --build-arg="VERSION=testing" --build-arg="REVISION=n/a" -t gtstef/filebrowser -f _docker/Dockerfile.slim .

build-backend:
	@echo "Building backend..."
	cd backend && go build -o filebrowser --ldflags="-w -s -X 'github.com/gtsteffaniak/filebrowser/backend/internal/version.CommitSHA=testingCommit' -X 'github.com/gtsteffaniak/filebrowser/backend/internal/version.Version=testing'"
	@echo "✓ Backend built successfully"

# New dev target with hot-reloading for frontend and backend
dev: generate-docs generate-icons setup-gofitz-cgo
	@echo "Starting dev servers... Press Ctrl+C to stop."
	pkill -f '[t]est_config.yaml' || true
	pkill -f '[a]ir -c .air' || true
	@cd frontend && DEV_BUILD=true npm run watch & \
	FRONTEND_PID=$$!; \
	cd backend && export FILEBROWSER_DEVMODE=true && $(call backend_dev_tool,air) $$([ "$(OS)" = "Windows_NT" ] && echo "-c .air.windows.toml" || echo "") & \
	BACKEND_PID=$$!; \
	trap 'echo "Stopping..."; kill $$FRONTEND_PID $$BACKEND_PID 2>/dev/null; sleep 1; kill -9 $$FRONTEND_PID $$BACKEND_PID 2>/dev/null; exit 0' INT TERM; \
	wait $$FRONTEND_PID $$BACKEND_PID 2>/dev/null || true

run: build-frontend generate-docs setup-gofitz-cgo
	cd backend && $(call backend_dev_tool,swag) init --output swagger/docs
	@if [ "$$(uname)" = "Darwin" ]; then \
		sed -i '' '/func init/,+3d' backend/swagger/docs/docs.go; \
	else \
		sed -i '/func init/,+3d' backend/swagger/docs/docs.go; \
	fi
	cd backend && CGO_ENABLED=1 FILEBROWSER_DEVMODE=true go run --tags=mupdf \
	--ldflags="-w -s -X 'github.com/gtsteffaniak/filebrowser/backend/internal/version.CommitSHA=testingCommit' -X 'github.com/gtsteffaniak/filebrowser/backend/internal/version.Version=testing'" . -c test_config.yaml

generate-docs:
	@echo "NOTE: Run 'make setup' if you haven't already."
	@echo "Generating swagger docs..."
	cd backend && $(call backend_dev_tool,swag) init --output swagger/docs
	@if [ "$$(uname)" = "Darwin" ]; then \
		sed -i '' '/func init/,+3d' backend/swagger/docs/docs.go; \
	else \
		sed -i '/func init/,+3d' backend/swagger/docs/docs.go; \
	fi
	@echo "Generating frontend config..."
	cd backend && FILEBROWSER_GENERATE_CONFIG=true go run .

build-frontend:
	@echo "Building frontend..."
	cd frontend && npm run build
	@echo "✓ Frontend built successfully"

lint-frontend:
	cd frontend && npm run lint

lint-backend:
	cd backend && GOLANGCI_LINT="$$(cd $(BACKEND_BUILD) && go tool -n golangci-lint)" && "$$GOLANGCI_LINT" run --path-prefix=backend

lint: lint-backend lint-frontend

test: test-backend test-frontend

check-all: lint test check-translations check-icons

check-icons:
	cd frontend && npm run icons:check

sync-icons:
	cd frontend && npm run icons:sync

generate-icons:
	cd frontend && npm run icons:sync

check-translations:
	cd frontend && npm run i18n:check

sync-translations:
	cd frontend && npm run i18n:sync

reorder-translations:
	cd frontend && npm run i18n:enforce-order

cleanup-translations:
	cd frontend && npm run i18n:cleanup

test-backend:
	cd backend && go test -race -timeout=30s ./...

test-frontend:
	cd frontend && npm run test

test-playwright: build-frontend
	cd backend && GOOS=linux go build -o filebrowser .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-sharing .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-settings .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-noauth .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-general .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-jwt .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-proxy .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-previews .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-oidc .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-no-config .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-screenshots .

# --- Listing performance (optional; does not affect test-playwright / CI playwright matrix) ---

# Same image as CI (chromium only). Matches _docker/Dockerfile.playwright-base on main.
playwright-base:
	DOCKER_BUILDKIT=1 docker build -t filebrowser-playwright-base \
		-f _docker/Dockerfile.playwright-base .

# Optional multi-browser base for local perf-check only (not used by test-playwright / GHCR publish).
PERF_PLAYWRIGHT_BROWSERS ?= chromium,firefox,webkit

playwright-perf-base:
	DOCKER_BUILDKIT=1 docker build -t filebrowser-playwright-perf-base \
		--build-arg PLAYWRIGHT_BROWSERS="$(PERF_PLAYWRIGHT_BROWSERS)" \
		-f _docker/Dockerfile.playwright-perf-base .

test-playwright-performance: build-frontend
	cd backend && GOOS=linux go build -o filebrowser .
	DOCKER_BUILDKIT=1 docker build --target ci -t filebrowser-playwright-performance \
		-f _docker/Dockerfile.playwright-performance .

PERF_GREP ?=
PERF_SKIP_BUILD ?=
PERF_SKIP_PLAYWRIGHT_BASE ?=
# Local runs repeat each scenario 3x and take the median; CI uses a single pass.
PERF_REPEATS ?= 3

# Full local sweep: all three browsers, for cross-browser comparison.
# Only chromium feeds the CI baseline; the others are advisory insight.
perf-check:
ifndef PERF_SKIP_BUILD
	$(MAKE) build-frontend
endif
	cd backend && GOOS=linux go build -o filebrowser .
ifndef PERF_SKIP_PLAYWRIGHT_BASE
	$(MAKE) playwright-perf-base PLAYWRIGHT_BROWSERS="$(PERF_PLAYWRIGHT_BROWSERS)"
endif
	@echo "Running listing performance tests (docker, all browsers)..."
	mkdir -p frontend/test-results/listing-performance-perf frontend/test-results/playwright-performance
	cd _docker && DOCKER_BUILDKIT=1 PERF_GREP="$(PERF_GREP)" docker compose run --rm --build local-playwright-performance
	@echo "--- Dashboard: make perf-dashboard  →  http://127.0.0.1:9323/dashboard/report.html ---"

# Fast local smoke test: small scales, chromium only, one pass. Seconds, not minutes.
perf-smoke:
	cd frontend && PERF_SCALES=100,1000 PERF_BROWSERS=chromium PERF_REPEATS=1 \
		npx playwright test -c playwright.performance.config.ts

# Regenerate the committed chromium baseline. Run this in the SAME environment
# as CI so timings are comparable: the baseline records worker count, browser
# version and Playwright version, and comparisons warn when they differ.
perf-baseline:
	@echo "Regenerating chromium baseline (PERF_UPDATE_BASELINE=1)..."
	cd frontend && PERF_UPDATE_BASELINE=1 PERF_BROWSERS=chromium \
		npx playwright test -c playwright.performance.config.ts
	@echo "Baseline written to frontend/tests/playwright/performance/perf-baseline.json"

perf-dashboard:
	cd frontend && node ./scripts/serve-perf-dashboard.mjs

# get version from environment variable, for example
# cd frontend && npm i @playwright/test && npx playwright install --with-deps chromium
# make PLAYWRIGHT_TEST=settings test-playwright-ui 
test-playwright-ui: build-frontend
	docker stop local-playwright-tests || true
	docker rm local-playwright-tests || true
	rm -rf _docker/src/tmp/ || true && mkdir -p _docker/src/tmp/
	cp -r _docker/src/$(PLAYWRIGHT_TEST)/backend/* _docker/src/tmp/
	cp -r backend/reduce-rounded-corners.css _docker/src/tmp/no-rounded.css
	cp -r _docker/src/$(PLAYWRIGHT_TEST)/frontend/playwright.config.ts $(shell pwd)/frontend/playwright.config.ts
	cd backend && GOOS=linux go build -o ../_docker/src/tmp/filebrowser .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-local .
	docker run -d -p 80:80 --name local-playwright-tests -t filebrowser-playwright-tests .
	cd frontend && npx playwright test --ui

run-proxy: build-frontend
	cd _docker && docker compose up -d --build nginx-proxy-auth filebrowser

run-jwt: build-frontend
	cd _docker && docker compose up -d --build nginx-proxy-jwt filebrowser-jwt

# optional: install playwright locally
# once local playwright server is running, you can also watch the tests interactively with:
# cd frontend && npx playwright test --project dark-screenshots --ui
screenshots: build-frontend
	cd backend && GOOS=linux go build -o filebrowser --ldflags="-w -s -X 'github.com/gtsteffaniak/filebrowser/backend/internal/version.Version=latest'" .
	@echo "Running screenshots..."
	cd _docker && docker compose down && docker compose up --build local-playwright-screenshots
	@if [ -d ../filebrowserDocs ]; then \
		rm -rf ../filebrowserDocs/static/images/generated/; \
		cp -r ./frontend/generated ../filebrowserDocs/static/images/; \
		echo "Copied screenshots to ../filebrowserDocs/static/images/generated/"; \
	fi
