# Contributing to FileBrowser Quantum

Thank you for your interest in contributing to FileBrowser Quantum! This guide will help you get started with development.

## Prerequisites

- **Go 1.25+** (see `backend/go.mod`)
- **Node.js 22.0.0+** with npm 9.0.0+ (see `frontend/package.json`)
- **Docker** (optional, for containerized development)
- **Git** especially improtant on windows -- needed for bash support.

### Optional Tools

- **ffmpeg/ffprobe**: For media features (subtitles, thumbnails, heic)
  - Ubuntu/Debian: `sudo apt-get install ffmpeg`
  - macOS: `brew install ffmpeg`
  - Windows: Download from [ffmpeg.org](https://ffmpeg.org/download.html)

- **exiftool**: For photo metadata processing (RAW/HEIC previews, EXIF orientation)
  - Ubuntu/Debian: `sudo apt-get install libimage-exiftool-perl`
  - macOS: `brew install exiftool`
  - Windows: Download from [exiftool.org](https://exiftool.org/) (install `exiftool.exe` and add it to your PATH, or set `integrations.media.exiftoolPath` in config)

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
