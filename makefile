setup:
	cd frontend && npm i && npx playwright install
	if [ ! -f backend/test__config.yaml ]; then \
		cp backend/filebrowser.yaml backend/test_config.yaml; \
	fi

build:
	docker build --build-arg="VERSION=testing" --build-arg="REVISION=n/a"  -t gtstef/filebrowser .

run: run-frontend
	cd backend && FILEBROWSER_NO_EMBEDED=true go run \
	--ldflags="-w -s -X 'github.com/gtsteffaniak/filebrowser/version.CommitSHA=testingCommit' -X 'github.com/gtsteffaniak/filebrowser/version.Version=testing'" . -c test_config.yaml

run-frontend:
	cd backend/http && rm -rf dist && ln -s ../../frontend/dist && \
	cd ../../frontend && npm run build

lint-frontend:
	cd frontend && npm run lint

lint-backend:
	cd backend && golangci-lint run --path-prefix=backend

test-backend:
	cd backend && go test -race ./...

test-frontend:
	docker build -t gtstef/filebrowser-tests -f Dockerfile.playwright .
