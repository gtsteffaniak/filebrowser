setup:
	cd frontend && npm i && npx playwright install
	if [ ! -f backend/test__config.yaml ]; then \
		cp backend/filebrowser.yaml backend/test_config.yaml; \
	fi

build:
	docker build --build-arg="VERSION=testing" --build-arg="REVISION=n/a"  -t gtstef/filebrowser .

dev:
	# Kill processes matching exe/filebrowser, ignore errors if process does not exist
	-pkill -f "exe/filebrowser" || true
	# Start backend and frontend concurrently
	cd backend && FILEBROWSER_NO_EMBEDED=true go run \
	--ldflags="-w -s -X 'github.com/gtsteffaniak/filebrowser/version.CommitSHA=testingCommit' -X 'github.com/gtsteffaniak/filebrowser/version.Version=testing'" \
	. -c test_config.yaml & BACKEND_PID=$$!; \
	cd frontend && npm run watch & FRONTEND_PID=$$!; \
	wait $$BACKEND_PID $$FRONTEND_PID

lint-frontend:
	cd frontend && npm run lint

lint-backend:
	cd backend && golangci-lint run --path-prefix=backend

test-backend:
	cd backend && go test -race ./...

test-frontend:
	docker build -t gtstef/filebrowser-tests -f Dockerfile.playwright .
