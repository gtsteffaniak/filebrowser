.SILENT:
setup:
	echo "creating ./backend/test_config.yaml for local testing..." && \
	if [ ! -f backend/test__config.yaml ]; then \
		cp backend/filebrowser.yaml backend/test_config.yaml; \
	fi
	echo "installing swagger needed to generate backend api docs..." && \
	go install github.com/swaggo/swag/cmd/swag@latest && \
	echo "installing npm requirements for frontend..." && \
	cd frontend && npm i


update:
	cd backend && go get -u ./... && go mod tidy && cd ../frontend && npm update

build:
	docker build --build-arg="VERSION=testing" --build-arg="REVISION=n/a"  -t gtstef/filebrowser .

run: run-frontend
	cd backend && swag init --output swagger/docs && \
	if [ "$(shell uname)" = "Darwin" ]; then \
		sed -i '' '/func init/,+3d' ./swagger/docs/docs.go; \
	else \
		sed -i '/func init/,+3d' ./swagger/docs/docs.go; \
	fi && \
	FILEBROWSER_NO_EMBEDED=true go run \
	--ldflags="-w -s -X 'github.com/gtsteffaniak/filebrowser/version.CommitSHA=testingCommit' -X 'github.com/gtsteffaniak/filebrowser/version.Version=testing'" . -c test_config.yaml

run-frontend:
	cd backend/http && rm -rf dist && ln -s ../../frontend/dist && \
	cd ../../frontend && npm run build

lint-frontend:
	cd frontend && npm run lint

lint-backend:
	cd backend && golangci-lint run --path-prefix=backend

lint: lint-backend lint-frontend

test: test-backend test-frontend

check-all: lint test

test-backend:
	cd backend && go test -race -timeout=10s ./...

test-frontend:
	cd frontend && npm run test

test-frontend-playwright:
	npx playwright install
	docker build -t gtstef/filebrowser-tests -f Dockerfile.playwright .
