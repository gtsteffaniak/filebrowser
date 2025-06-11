# Contributing to FileBrowser Quantum

Thank you for your interest in contributing to FileBrowser Quantum! This guide will help you get started with development.

## Prerequisites

- **Go 1.24.2+** (see `backend/go.mod`)
- **Node.js 18.0.0+** with npm 7.0.0+ (see `frontend/package.json`)
- **Docker** (optional, for containerized development)
- **Git**

### Required Tools

- **ffmpeg**: Required for video thumbnail generation
  - Ubuntu/Debian: `sudo apt-get install ffmpeg`
  - macOS: `brew install ffmpeg`
  - Windows: Download from [ffmpeg.org](https://ffmpeg.org/download.html)
  
- **mupdf-tools**: Required for PDF preview generation
  - Ubuntu/Debian: `sudo apt-get install mupdf-tools`
  - macOS: `brew install mupdf-tools`
  - Windows: Download from [mupdf.com](https://mupdf.com/downloads/)

## Quick Start

```bash
# 1. Clone the repository
git clone https://github.com/gtsteffaniak/filebrowser.git
cd filebrowser

# 2. Initial setup - installs dependencies and creates test config
make setup

# 3. Run in development mode
make run
```

`make setup` installs all dependencies and creates a test configuration file.
`make run` starts the development server with hot reload.

## Project Architecture

### Backend (Go)
- **Entry Point**: `backend/main.go` → `backend/cmd/`
- **HTTP Server**: `backend/http/` - API routes, middleware, auth
- **Storage**: BoltDB via `backend/database/storage/`
- **Authentication**: Multiple providers in `backend/auth/`
- **Indexing**: Real-time search in `backend/indexing/`
- **Previews**: Image/video/document generation in `backend/preview/`

### Frontend (Vue.js + TypeScript)
- **Framework**: Vue 3 + Vite + TypeScript
- **State**: Custom store in `frontend/src/store/`
- **API Client**: Axios-based in `frontend/src/api/`
- **i18n**: 25+ languages with English as master
- **Components**: Feature-based organization

## Development Commands

### Essential Commands
```bash
make run          # Start development server
make test         # Run all tests
make lint         # Check code quality
make check-all    # Lint + tests

make build-frontend  # Build frontend only
make build-backend   # Build backend only
make build          # Build Docker image
```

### Frontend Development
```bash
cd frontend
npm run dev       # Dev server with hot reload
npm run lint:fix  # Auto-fix linting issues
npm run typecheck # TypeScript validation
npm run i18n:sync # Sync translations
```

## Testing

### Running Tests
```bash
make test              # All tests
make test-backend      # Go tests with race detection
make test-frontend     # Frontend unit tests
make test-playwright   # E2E tests in Docker
```

### Coverage & Performance
```bash
cd backend
./run_check_coverage.sh  # Coverage report with HTML output
./run_benchmark.sh       # Benchmarks
```

**Code Coverage:**
- View report: Open `backend/coverage.html` after running coverage script
- CI enforces coverage for critical packages
- Use `go test -cover` for quick package coverage

E2E tests run with three authentication modes: standard auth, no auth, and proxy auth.

## Code Standards

### Backend (Go)
- **Linting**: `backend/.golangci.yml` with 30+ checks
- **Format**: Use `gofmt` (automated in CI)
- **Testing**: Maintain 80%+ coverage
- **Errors**: Handle all errors explicitly

### Frontend (Vue.js)
- **Linting**: ESLint with Vue 3 + TypeScript rules
- **i18n**: English is master locale, all text must use `$t('key')`
- **Types**: Use TypeScript everywhere
- **Fix**: Run `npm run lint:fix` before committing

## Build & Deployment

### Single Binary Build
The project builds into a single binary with embedded frontend:

```bash
make build-frontend  # Build Vue.js app
make build-backend   # Build Go binary with embedded assets
```

### Docker
```bash
make build       # Full image with ffmpeg (~200MB)
make run-proxy   # Docker Compose with nginx proxy
```

### Configuration
- **Config**: `backend/config.yaml` (YAML format)
- **Test Config**: Auto-generated `backend/test_config.yaml`
- **Sections**: server, sources, userDefaults, auth

### Environment Variables

Development environment variables:

```bash
# Backend
FILEBROWSER_NO_EMBEDDED=true    # Disable embedded frontend for development
FILEBROWSER_BASE_URL=/custom    # Custom base URL path
FILEBROWSER_LOG_LEVEL=debug     # Log levels: debug, info, warn, error

# Authentication
FILEBROWSER_AUTH_METHOD=json    # Auth types: none, json, proxy, oidc
FILEBROWSER_AUTH_HEADER=X-Auth  # Proxy auth header name

# Database
FILEBROWSER_DATABASE_PATH=./filebrowser.db  # BoltDB location

# Server
FILEBROWSER_PORT=8080           # Server port
FILEBROWSER_ADDRESS=0.0.0.0    # Listen address
```


## Contributing

### Pull Request Process

1. Fork and create a feature branch
2. Make your changes following the code standards
3. Run `make check-all` to verify tests and linting
4. Submit PR with clear description

### PR Requirements
- Clear description of changes
- All tests must pass
- Follow existing code patterns
- Update documentation if needed

### Commit Format
```
type(scope): description

Types: feat, fix, docs, refactor, test, chore
```

## Roadmap

Check the [Project Roadmap](https://github.com/users/gtsteffaniak/projects/4/views/2) to see issues sorted by priority. This helps you understand what features are planned and where you can contribute most effectively.

## Troubleshooting

### Common Issues

**Build failures:**
```bash
# Frontend
cd frontend && rm -rf node_modules && npm install

# Backend
cd backend && go mod tidy && go clean -modcache

# Docker
docker system prune -a
```

**Debugging:**
```bash
# Backend with debug logs
cd backend
FILEBROWSER_NO_EMBEDDED=true go run -tags mupdf . -c test_config.yaml --log-level debug

# API documentation
# Visit /swagger endpoint when running
```

**Authentication Issues:**

1. **OIDC Login Fails:**
   - Check redirect URLs match your config
   - Verify OIDC provider settings
   - Enable debug logs to see auth flow
   - Common issue: mismatched callback URLs

2. **Proxy Auth Not Working:**
   - Verify header names match config
   - Check nginx/reverse proxy passes headers
   - Test with: `curl -H "X-Auth: username" localhost:8080`
   - Enable `--log-level debug` to see headers

3. **TOTP Issues:**
   - Ensure server time is synchronized
   - Check QR code generation in logs
   - Test with authenticator app time settings
   - Database might have stale TOTP secrets

4. **Session/Cookie Problems:**
   - Clear browser cookies and localStorage
   - Check `FILEBROWSER_BASE_URL` if using subpath
   - Verify cookie domain settings
   - Try incognito/private browsing mode

### Getting Help

- **Wiki**: [Project Wiki](https://github.com/gtsteffaniak/filebrowser/wiki)
- **Issues**: [GitHub Issues](https://github.com/gtsteffaniak/filebrowser/issues)
- **PR Template**: See `.github/PULL_REQUEST_TEMPLATE.md`

For detailed architecture information, see [CLAUDE.md](CLAUDE.md).