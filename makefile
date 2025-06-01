SHELL := /bin/bash

.SILENT:
setup:
	echo "creating ./backend/test_config.yaml for local testing..."
	if [ ! -f backend/test_config.yaml ]; then \
		cp backend/config.yaml backend/test_config.yaml; \
	fi
	echo "installing backend tooling..."
	cd backend && go get tool
	echo "installing npm requirements for frontend..."
	cd frontend && npm i

update:
	cd backend && go get -u ./... && go mod tidy
	cd frontend && npm update

build:
	docker build --build-arg="VERSION=testing" --build-arg="REVISION=n/a" -t gtstef/filebrowser -f _docker/Dockerfile .

build-backend:
	cd backend && go build -o filebrowser --ldflags="-w -s -X 'github.com/gtsteffaniak/filebrowser/backend/version.CommitSHA=testingCommit' -X 'github.com/gtsteffaniak/filebrowser/backend/version.Version=testing'"

run: build-frontend
	cd backend && go tool swag init --output swagger/docs && \
	if [ "$(shell uname)" = "Darwin" ]; then \
		sed -i '' '/func init/,+3d' ./swagger/docs/docs.go; \
	else \
		sed -i '/func init/,+3d' ./swagger/docs/docs.go; \
	fi && \
	FILEBROWSER_NO_EMBEDED=true go run -tags "pdf" \
	--ldflags="-w -s -X 'github.com/gtsteffaniak/filebrowser/backend/version.CommitSHA=testingCommit' -X 'github.com/gtsteffaniak/filebrowser/backend/version.Version=testing'" . -c test_config.yaml

build-frontend:
	cd backend && rm -rf http/dist http/embed/* && \
	FILEBROWSER_GENERATE_CONFIG=true go run . && cp generated.yaml ../frontend/public/config.generated.yaml
	cd backend/http/ && ln -s ../../frontend/dist
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

check-all: lint test

test-backend:
	cd backend && go test -race -timeout=10s ./...

test-frontend:
	cd frontend && npm run test

test-playwright: build-frontend
	cd backend && GOOS=linux go build -o filebrowser .
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-general .
	docker run --rm --name filebrowser-playwright-tests filebrowser-playwright-tests
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-noauth .
	docker run --rm --name filebrowser-playwright-tests filebrowser-playwright-tests
	docker build -t filebrowser-playwright-tests -f _docker/Dockerfile.playwright-proxy .
	docker run --rm --name filebrowser-playwright-tests filebrowser-playwright-tests

run-proxy: build-frontend
	cd _docker && docker compose up -d --build