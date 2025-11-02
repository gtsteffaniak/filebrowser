# Contributing to FileBrowser Quantum

Thank you for your interest in contributing to FileBrowser Quantum! This guide will help you get started with development.

## Prerequisites

- **Go 1.25+** (see `backend/go.mod`)
- **Node.js 20.0.0+** with npm 9.0.0+ (see `frontend/package.json`)
- **Docker** (optional, for containerized development)
- **Git** especially improtant on windows -- needed for bash support.

### Optional Tools

- **ffmpeg**: For media features (subtitles, thumbnails, heic)
  - Ubuntu/Debian: `sudo apt-get install ffmpeg`
  - macOS: `brew install ffmpeg`
  - Windows: Download from [ffmpeg.org](https://ffmpeg.org/download.html)

- **mupdf-tools**: For PDF preview generation
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

# 3. Run in development mode with hot-reloading
make dev
```

`make setup` installs all dependencies and creates a test configuration file.
`make run` starts the development server. Note: changes require ctrl+c and re-run.

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
- **i18n**: 25+ languages with English as main
- **Components**: Feature-based organization

## Development Commands

### Essential Commands
```bash
make dev          # Start development server with hot-reloading
make test         # Run all tests
make lint         # Check code quality
make check-all    # Lint + tests

make build-frontend  # Build frontend only
make build-backend   # Build backend only
make build          # Build Docker image
```

### Frontend Development

Note: consider using make commands above instead.
```bash
cd frontend
npm run lint:fix  # Auto-fix linting issues
npm run i18n:sync # Sync translations changes
```

## Testing

### Running Tests
```bash
make test              # All tests
make test-backend      # Go tests with race detection
make test-frontend     # Frontend unit tests
make test-playwright   # E2E tests in Docker
```

E2E tests run with three authentication modes: standard auth, no auth, and proxy auth.

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
make build       # Full image with ffmpeg and muPDF
```

### Configuration
- **Test Config**: `backend/test_config.yaml` Auto-generated after running `make setup`

## Contributing

### Pull Request Process

1. Fork and create a feature branch
2. Make your changes following the code standards
3. Run `make dev` to build and run with your changes. Supports hot-reloading frontend and backend changes.
4. When ready, run `make check-all` to verify tests and linting
5. Submit PR with a clear description

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
```

**Authentication Issues:**

Always first enable debug logging in your config file if you have issues.

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
   - Verify cookie domain settings
   - Try incognito/private browsing mode

### Getting Help

- **Wiki**: [Project Wiki](https://github.com/gtsteffaniak/filebrowser/wiki)
- **Issues**: [GitHub Issues](https://github.com/gtsteffaniak/filebrowser/issues)
- **PR Template**: See `.github/PULL_REQUEST_TEMPLATE.md`
