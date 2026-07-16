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

# git checkout remote branch PR
# git fetch origin pull/####/head:pr-####

.SILENT:

.PHONY: setup update build build-docker build-backend build-frontend dev run generate-docs
.PHONY: lint-frontend lint-backend lint test test-backend test-frontend check-all
.PHONY: check-translations sync-translations test-playwright run-proxy screenshots
.PHONY: check-icons generate-icons sync-icons

setup:
	echo "creating ./backend/test_config.yaml for local testing..."
	if [ ! -f backend/test_config.yaml ]; then \
		cp backend/config.yaml backend/test_config.yaml; \
	fi
	echo "installing backend tooling..."
	cd backend && go get tool
	cd backend/internal/web && mkdir -p embed dist && touch embed/.gitignore
	echo "installing npm requirements for frontend..."
	cd frontend && npm i

update:
	cd backend && go get -u ./... && go mod tidy
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
dev: generate-docs generate-icons
	@echo "Starting dev servers... Press Ctrl+C to stop."
	pkill -f '[t]est_config.yaml' || true
	pkill -f '[g]o tool air' || true
	@cd frontend && DEV_BUILD=true npm run watch & \
	FRONTEND_PID=$$!; \
	cd backend && export FILEBROWSER_DEVMODE=true && go tool air $$([ "$(OS)" = "Windows_NT" ] && echo "-c .air.windows.toml" || echo "") & \
	BACKEND_PID=$$!; \
	trap 'echo "Stopping..."; kill $$FRONTEND_PID $$BACKEND_PID 2>/dev/null; sleep 1; kill -9 $$FRONTEND_PID $$BACKEND_PID 2>/dev/null; exit 0' INT TERM; \
	wait $$FRONTEND_PID $$BACKEND_PID 2>/dev/null || true

run: build-frontend generate-docs
	cd backend && go tool swag init --output swagger/docs
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
	cd backend && go tool swag init --output swagger/docs
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
	cd backend && go tool golangci-lint run --path-prefix=backend

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
