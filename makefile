setup:
	cd frontend && npm i

build:
	docker build -t gtstef/filebrwoser .

dev:
	# Kill processes matching exe/filebrowser, ignore errors if process does not exist
	-pkill -f "exe/filebrowser" || true
	# Start backend and frontend concurrently
	cd backend && go run . & BACKEND_PID=$$!; \
	cd frontend && npm run watch & FRONTEND_PID=$$!; \
	wait $$BACKEND_PID $$FRONTEND_PID