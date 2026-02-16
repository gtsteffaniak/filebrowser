# Use bash on all platforms
# Windows users: Requires Git Bash (comes with Git for Windows: https://git-scm.com/download/win)
# Add C:\Program Files\Git\bin to your PATH, or run make from Git Bash terminal
ifeq ($(OS),Windows_NT)
    SHELL := C:/Program Files/Git/bin/bash.exe
    .SHELLFLAGS := -ec
else
    SHELL := /bin/bash
endif

# git checkout remote branch PR
# git fetch origin pull/####/head:pr-####

.SILENT:

.PHONY: setup update build build-docker build-backend build-frontend dev run generate-docs
.PHONY: lint-frontend lint-backend lint test test-backend test-frontend check-all
.PHONY: check-translations sync-translations test-playwright run-proxy screenshots

setup:
	echo "creating ./backend/test_config.yaml for local testing..."
	if [ ! -f backend/test_config.yaml ]; then \
		cp backend/config.yaml backend/test_config.yaml; \
	fi
	echo "installing backend tooling..."
	cd backend && go get tool
	cd backend/http && mkdir -p embed && touch embed/.gitignore
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
	cd backend && go build -o filebrowser --ldflags="-w -s -X 'github.com/gtsteffaniak/filebrowser/backend/common/version.CommitSHA=testingCommit' -X 'github.com/gtsteffaniak/filebrowser/backend/common/version.Version=testing'"
	@echo "✓ Backend built successfully"

# New dev target with hot-reloading for frontend and backend
dev: generate-docs
	@echo "Starting dev servers... Press Ctrl+C to stop."
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
	--ldflags="-w -s -X 'github.com/gtsteffaniak/filebrowser/backend/common/version.CommitSHA=testingCommit' -X 'github.com/gtsteffaniak/filebrowser/backend/common/version.Version=testing'" . -c test_config.yaml

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

check-all: lint test check-translations

check-translations:
	cd frontend && npm run i18n:check

sync-translations:
	cd frontend && npm run i18n:sync

test-backend:
	cd backend && go test -race -timeout=10s ./...

test-frontend:
	cd frontend && npm run test

test-playwright: build-frontend
	cd backend && GOOS=linux go build -o filebrowser .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-previews .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-noauth .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-settings .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-oidc .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-sharing .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-general .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-no-config .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-proxy .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-screenshots .

run-proxy: build-frontend
	cd _docker && docker compose up -d --build

# once local playwright server is running, you can also watch the tests interactively with:
# cd frontend && npx playwright test --project dark-screenshots --ui
screenshots: build-frontend
	cd backend && GOOS=linux go build -o filebrowser .
	@echo "Running screenshots..."
	cd _docker && docker compose down && docker compose up -d --build local-playwright
	@echo "Installing playwright dependencies..."
	cd frontend && npx playwright install --with-deps firefox
	echo "Generating dark screenshots...";
	cd frontend && npx playwright test --project dark-screenshots
	echo "Running light screenshots...";
	cd frontend && npx playwright test --project light-screenshots

profile:
	@echo "Note: start the backend server with 'make dev' first"
	@echo "Results will be in ./backend/debug/ directory"
	@mkdir -p backend/debug
	@echo "Downloading heap profile..."
	@curl -s http://localhost:6060/debug/pprof/heap > backend/debug/heap.pb.gz || (echo "Error: Could not download heap profile. Is the server running?" && exit 1)
	@echo "Generating heap profile (SVG)..."
	cd backend && go tool pprof -svg -output debug/heap.svg debug/heap.pb.gz
	@echo "Generating heap profile (text)..."
	cd backend && go tool pprof -text debug/heap.pb.gz > debug/heap.txt 2>&1
	@echo "Downloading CPU profile..."
	@curl -s "http://localhost:6060/debug/pprof/profile?seconds=30" > backend/debug/cpu.pb.gz || (echo "Error: Could not download CPU profile. Is the server running?" && exit 1)
	@echo "Generating CPU profile (SVG)..."
	cd backend && go tool pprof -svg -output debug/cpu.svg debug/cpu.pb.gz
	@echo "Generating CPU profile (text)..."
	cd backend && go tool pprof -text debug/cpu.pb.gz > debug/cpu.txt 2>&1
	@echo "✓ Generated debug files: heap.pb.gz, heap.svg, heap.txt, cpu.pb.gz, cpu.svg, cpu.txt"

memory:
	@echo "Fetching memory stats from running server..."
	@echo "Note: start the backend server with 'make dev' first"
	@echo "Usage: make memory [PORT=8080]"
	@PORT=$${PORT:-8080}; \
	curl -s http://localhost:$$PORT/api/memory | python3 -m json.tool || \
	(echo "Error: Could not fetch memory stats. Is the server running on port $$PORT?" && exit 1)
