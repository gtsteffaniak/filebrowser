setup:
	cd frontend && npm i
	if [ ! -f backend/test__config.yaml ]; then \
		cp backend/filebrowser.yaml backend/test_config.yaml; \
	fi

build:
	docker build -t gtstef/filebrwoser .

dev:
	# Kill processes matching exe/filebrowser, ignore errors if process does not exist
	-pkill -f "exe/filebrowser" || true
	# Start backend and frontend concurrently
	cd backend && go run . -c test_config.yaml & BACKEND_PID=$$!; \
	cd frontend && npm run watch & FRONTEND_PID=$$!; \
	wait $$BACKEND_PID $$FRONTEND_PID

make lint-frontend:
	cd frontend && npm run lint

make lint-backend:
	cd backend && golangci-lint run