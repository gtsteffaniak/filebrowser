SHELL := /bin/bash

.SILENT:
setup:
	echo "creating ./backend/test_config.yaml for local testing..."
	if [ ! -f backend/test_config.yaml ]; then \
		cp backend/config.yaml backend/test_config.yaml; \
	fi
	echo "installing backend tooling..."
	cd backend && go get tool
	cd backend/http && mkdir embed || true && touch embed/.gitignore
	echo "installing npm requirements for frontend..."
	cd frontend && npm i

update:
	cd backend && go get -u ./... && go mod tidy
	cd frontend && npm update

build:
	docker build --build-arg="VERSION=testing" --build-arg="REVISION=n/a" -t gtstef/filebrowser -f _docker/Dockerfile .

build-backend:
	cd backend && go build -o filebrowser --ldflags="-w -s -X 'github.com/gtsteffaniak/filebrowser/backend/common/version.CommitSHA=testingCommit' -X 'github.com/gtsteffaniak/filebrowser/backend/common/version.Version=testing'"

AIR_CONFIG ?= .air.toml
# New dev target with hot-reloading for frontend and backend
dev:
	@echo "NOTE: Run 'make setup' if you haven't already."
	@echo "Generating swagger docs..."
	cd backend && go tool swag init --output swagger/docs && \
	if [ "$(shell uname)" = "Darwin" ]; then \
		sed -i '' '/func init/,+3d' ./swagger/docs/docs.go; \
	else \
		sed -i '/func init/,+3d' ./swagger/docs/docs.go; \
	fi
	@echo "Generating frontend config..."
	cd backend && FILEBROWSER_GENERATE_CONFIG=true go run --tags=mupdf .
	cd frontend && npm run build
	@echo "Starting dev servers... Press Ctrl+C to stop."
	@trap 'echo "Stopping servers..."; kill -TERM 0' INT TERM
	cd frontend && DEV_BUILD=true npm run watch & \
	cd backend && FILEBROWSER_DEVMODE=true go tool air -c $(AIR_CONFIG) & \
	wait

dev-windows:
	FILEBROWSER_CONFIG=http/dist/config.generated.yaml AIR_CONFIG=.air.windows.toml make dev

run: build-frontend generate-config
	cd backend && go tool swag init --output swagger/docs && \
	if [ "$(shell uname)" = "Darwin" ]; then \
		sed -i '' '/func init/,+3d' ./swagger/docs/docs.go; \
	else \
		sed -i '/func init/,+3d' ./swagger/docs/docs.go; \
	fi && \
	CGO_ENABLED=1 FILEBROWSER_DEVMODE=true go run --tags=mupdf \
	--ldflags="-w -s -X 'github.com/gtsteffaniak/filebrowser/backend/common/version.CommitSHA=testingCommit' -X 'github.com/gtsteffaniak/filebrowser/backend/common/version.Version=testing'" . -c test_config.yaml

generate-config:
	cd backend && FILEBROWSER_GENERATE_CONFIG=true go run .

build-frontend:
	if [ "$(OS)" = "Windows_NT" ]; then \
		cd frontend && npm run build-windows; \
	else \
		cd frontend && npm run build; \
	fi

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
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-noauth .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-no-config .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-settings .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-general .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-sharing .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-proxy .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-oidc .

run-proxy: build-frontend
	cd _docker && docker compose up -d --build

screenshots: build-frontend build-backend
	# copy the playwright-files directory so you don't edit the original
	cd frontend && rm -rf playwright-files || true && cp -r tests/playwright-files .
	# Kill any existing backend processes
	@echo "Killing any existing backend processes..."
	@pkill -f "go run ." || true
	@pkill -f "filebrowser" || true
	@pkill -f "backend" || true
	@lsof -ti:8080 | xargs kill -9 2>/dev/null || true
	@echo "Starting backend server..."
	@trap 'echo "Stopping backend server..."; pkill -f "go run ." || true; pkill -f "filebrowser" || true; pkill -f "backend" || true; lsof -ti:8080 | xargs kill -9 2>/dev/null || true; exit 0' INT TERM
	rm -rf backend/playwright-files.db || true
	cd backend && go run . -c playwright-config.yaml &
	BACKEND_PID=$$!; \
	sleep 2; \
	echo "Running dark screenshots..."; \
	cd frontend && npx playwright test --project dark-screenshots; \
	echo "Running light screenshots..."; \
	npx playwright test --project light-screenshots; \
	echo "Cleaning up..."; \
	kill $$BACKEND_PID 2>/dev/null || true; \
	pkill -f "go run ." || true; \
	pkill -f "filebrowser" || true; \
	lsof -ti:8080 | xargs kill -9 2>/dev/null || true