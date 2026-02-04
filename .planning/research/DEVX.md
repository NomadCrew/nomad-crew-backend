# Developer Experience (DevX) Research

**Project:** NomadCrew Backend
**Domain:** Go 1.24 Backend DevX Tooling
**Researched:** 2026-02-04
**Overall Confidence:** HIGH

---

## Executive Summary

This research covers modern Go developer experience tooling for 2025/2026, specifically targeting a Windows-primary development environment with Go 1.24, Gin framework, and GitHub Actions CI/CD.

**Key Recommendations:**
1. **Task (Taskfile)** over Makefile for Windows compatibility and YAML readability
2. **golangci-lint v2** with explicit linter configuration (v2 format)
3. **pre-commit** with tekwizely/pre-commit-golang hooks
4. **Consolidated .env.example** with environment-specific comments
5. **.editorconfig + .vscode/settings.json** for consistent formatting

---

## 1. Build Tool: Task (Taskfile) vs Makefile vs Just

### Recommendation: **Task (Taskfile)**

| Criterion | Makefile | Task | Just |
|-----------|----------|------|------|
| Windows Native | Poor (needs Make installed) | Excellent (built-in utils) | Good |
| Syntax | Tab-sensitive, cryptic | YAML, readable | Make-like |
| Installation | System package | Single binary, npm, go install | Cargo |
| Cross-platform | Requires shims | First-class | Good |
| Learning curve | Medium-High | Low | Low |
| Go ecosystem fit | Historical | Native Go tool | Rust-based |

**Why Task over Makefile:**
1. **Windows-first support** - Task announced built-in core utilities for Windows in 2025, eliminating shell compatibility issues
2. **No tab/space issues** - YAML format prevents the common Makefile debugging nightmare
3. **Single binary** - No dependencies beyond the Go runtime
4. **Better DX** - Tab completion, colored output, task descriptions

**Why Task over Just:**
1. **Go-native** - Written in Go, natural fit for Go projects
2. **Wider adoption** - More examples, better documentation for Go workflows
3. **Built-in dependency checking** - Can skip tasks when sources unchanged

### Task Installation (Windows)

```powershell
# Via Scoop (recommended for Windows)
scoop install task

# Via Chocolatey
choco install go-task

# Via npm (cross-platform)
npm install -g @go-task/cli

# Via Go
go install github.com/go-task/task/v3/cmd/task@latest
```

### Recommended Taskfile.yml

```yaml
version: '3'

vars:
  BINARY_NAME: nomadcrew-backend
  BUILD_DIR: ./tmp
  GO_FILES:
    sh: find . -name '*.go' -not -path './vendor/*' | head -1

env:
  CGO_ENABLED: 0

tasks:
  default:
    desc: Show available tasks
    cmds:
      - task --list

  # Development
  dev:
    desc: Start development server with hot reload
    cmds:
      - air

  run:
    desc: Run the server directly
    cmds:
      - go run main.go

  # Build
  build:
    desc: Build the binary
    cmds:
      - go build -o {{.BUILD_DIR}}/{{.BINARY_NAME}} -ldflags '-X main.Version={{.VERSION}} -X main.Environment={{.ENV}}' .
    vars:
      VERSION:
        sh: git describe --tags --always --dirty 2>/dev/null || echo "dev"
      ENV: '{{.ENV | default "development"}}'
    sources:
      - ./**/*.go
      - go.mod
      - go.sum
    generates:
      - '{{.BUILD_DIR}}/{{.BINARY_NAME}}'

  build:prod:
    desc: Build production binary
    cmds:
      - task: build
        vars:
          ENV: production

  # Testing
  test:
    desc: Run all tests
    cmds:
      - go test -race ./...

  test:coverage:
    desc: Run tests with coverage report
    cmds:
      - go test -coverprofile=coverage.out ./...
      - go tool cover -html=coverage.out -o coverage.html
    generates:
      - coverage.out
      - coverage.html

  test:short:
    desc: Run short tests only
    cmds:
      - go test -short ./...

  test:integration:
    desc: Run integration tests
    cmds:
      - go test -tags=integration -v ./...

  # Linting
  lint:
    desc: Run golangci-lint
    cmds:
      - golangci-lint run --timeout=5m

  lint:fix:
    desc: Run golangci-lint with auto-fix
    cmds:
      - golangci-lint run --fix --timeout=5m

  fmt:
    desc: Format code with golangci-lint fmt
    cmds:
      - golangci-lint fmt

  # Dependencies
  deps:
    desc: Download dependencies
    cmds:
      - go mod download
      - go mod verify

  deps:tidy:
    desc: Tidy go.mod
    cmds:
      - go mod tidy

  deps:update:
    desc: Update all dependencies
    cmds:
      - go get -u ./...
      - go mod tidy

  # Documentation
  docs:
    desc: Generate Swagger documentation
    cmds:
      - swag init -g main.go -o ./static/docs/api

  docs:serve:
    desc: Serve documentation locally
    deps: [docs]
    cmds:
      - echo "Swagger UI available at http://localhost:8080/swagger/index.html"
      - task: run

  # Database
  db:migrate:
    desc: Run database migrations
    cmds:
      - psql -d {{.DB_NAME}} -f db/migrations/init.sql
    vars:
      DB_NAME: '{{.DB_NAME | default "nomadcrew"}}'

  # Docker
  docker:build:
    desc: Build Docker image
    cmds:
      - docker build -t nomadcrew-backend:{{.TAG}} .
    vars:
      TAG: '{{.TAG | default "latest"}}'

  docker:up:
    desc: Start Docker Compose services
    cmds:
      - docker-compose up -d

  docker:down:
    desc: Stop Docker Compose services
    cmds:
      - docker-compose down

  docker:logs:
    desc: Show Docker Compose logs
    cmds:
      - docker-compose logs -f

  # Utilities
  clean:
    desc: Clean build artifacts
    cmds:
      - rm -rf {{.BUILD_DIR}}
      - rm -f coverage.out coverage.html

  check:
    desc: Run all checks (format, lint, test)
    cmds:
      - task: fmt
      - task: lint
      - task: test

  # Pre-commit
  pre-commit:install:
    desc: Install pre-commit hooks
    cmds:
      - pre-commit install

  pre-commit:run:
    desc: Run pre-commit on all files
    cmds:
      - pre-commit run --all-files

  # Setup
  setup:
    desc: Initial project setup
    cmds:
      - task: deps
      - task: pre-commit:install
      - echo "Setup complete! Run 'task dev' to start development server"
```

**Estimated setup time:** 10-15 minutes

---

## 2. golangci-lint Configuration

### Current State Analysis

The project uses golangci-lint v1.64.2 in CI with only `--timeout=5m` flag. No `.golangci.yml` exists, meaning default linters only.

### Recommendation: Upgrade to v2 Format

golangci-lint v2 was released in March 2025 with significant configuration improvements:
- New `version: "2"` format
- Separate `formatters` section
- `linters.default` replaces `enable-all`/`disable-all`
- Built-in `golangci-lint fmt` command

### Recommended .golangci.yml (v2 Format)

```yaml
# golangci-lint configuration v2
# https://golangci-lint.run/docs/configuration/
version: "2"

run:
  timeout: 5m
  tests: true
  go: "1.24"
  # Skip vendor, third-party, and generated files
  skip-dirs:
    - vendor
    - static/docs
  skip-files:
    - ".*_gen\\.go$"
    - ".*_mock\\.go$"

# Formatters configuration (new in v2)
formatters:
  enable:
    - goimports
    - gofumpt
  settings:
    goimports:
      local-prefixes: github.com/NomadCrew/nomad-crew-backend
    gofumpt:
      extra-rules: true

# Linters configuration
linters:
  # Start with standard set, add specific linters
  default: standard

  enable:
    # Error handling
    - errcheck       # Check error returns
    - errorlint      # Find issues with error wrapping
    - nilerr         # Find nil error returns

    # Code quality
    - gocyclo        # Cyclomatic complexity
    - gocognit       # Cognitive complexity
    - nakedret       # Naked returns in large functions
    - funlen         # Function length
    - dupl           # Code duplication

    # Security
    - gosec          # Security issues

    # Performance
    - ineffassign    # Ineffective assignments
    - unconvert      # Unnecessary conversions
    - prealloc       # Slice preallocation

    # Best practices
    - govet          # Go vet checks
    - staticcheck    # Staticcheck suite
    - revive         # Replacement for golint
    - misspell       # Spelling mistakes
    - whitespace     # Whitespace issues
    - bodyclose      # HTTP body close
    - noctx          # HTTP requests without context

    # Go 1.24 specific
    - copyloopvar    # Loop variable copying (Go 1.22+)
    - intrange       # Integer range loops (Go 1.22+)

  disable:
    # Too strict for existing codebase
    - exhaustruct    # Require all struct fields
    - wrapcheck      # Require error wrapping
    - varnamelen     # Variable name length
    - nlreturn       # Newline before return
    - wsl            # Whitespace linter
    - godox          # TODO/FIXME comments
    - ireturn        # Interface returns

  settings:
    errcheck:
      check-type-assertions: true
      check-blank: true
      exclude-functions:
        - io.Close
        - (*os.File).Close

    errorlint:
      errorf: true
      asserts: true
      comparison: true

    gocyclo:
      min-complexity: 15

    gocognit:
      min-complexity: 20

    funlen:
      lines: 100
      statements: 50

    govet:
      enable-all: true
      disable:
        - fieldalignment  # Too noisy

    gosec:
      excludes:
        - G104  # Audit errors not checked (handled by errcheck)
        - G304  # File path provided as taint input (intentional in some cases)

    revive:
      rules:
        - name: blank-imports
        - name: context-as-argument
        - name: context-keys-type
        - name: dot-imports
        - name: error-return
        - name: error-strings
        - name: error-naming
        - name: exported
        - name: if-return
        - name: increment-decrement
        - name: var-naming
        - name: var-declaration
        - name: package-comments
        - name: range
        - name: receiver-naming
        - name: time-naming
        - name: unexported-return
        - name: indent-error-flow
        - name: errorf
        - name: empty-block
        - name: superfluous-else
        - name: unused-parameter
          disabled: true  # Too strict
        - name: unreachable-code

    misspell:
      locale: US

    nakedret:
      max-func-lines: 30

    dupl:
      threshold: 150

  # Exclusions
  exclusions:
    # Enable common exclusion presets
    presets:
      - comments
      - std-error-handling
      - common-false-positives

    # Exclude test files from certain linters
    rules:
      - path: _test\.go
        linters:
          - funlen
          - dupl
          - gocyclo
          - errcheck
          - gosec

      # Allow dot imports in tests
      - path: _test\.go
        text: "dot-imports"

      # Exclude handlers from some complexity checks (HTTP handlers are naturally complex)
      - path: handlers/
        linters:
          - funlen
        text: "function .* is too long"

# Output configuration
output:
  formats:
    - format: colored-line-number
  print-issued-lines: true
  print-linter-name: true
  sort-results: true

# Issue configuration
issues:
  max-issues-per-linter: 50
  max-same-issues: 10
  new: false
  fix: false

# Severity configuration
severity:
  default: warning
  rules:
    - linters:
        - gosec
      severity: error
    - linters:
        - errcheck
        - errorlint
      severity: error
```

### CI Workflow Update

Update `.github/workflows/golang-cilint.yml`:

```yaml
name: golangci-lint
on:
  push:
    branches: [ '**' ]
  pull_request:
    branches: [ '**' ]
    paths-ignore:
      - '**.md'
      - 'docs/**'

permissions:
  contents: read

jobs:
  golangci:
    name: lint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Get dependencies
        run: |
          go mod download
          go mod verify

      - name: golangci-lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: v2.1  # Upgrade to v2
          args: --timeout=5m
          working-directory: ./

    env:
      GOFLAGS: "-buildvcs=false"
```

**Estimated setup time:** 15-20 minutes

---

## 3. Pre-commit Hooks

### Recommendation: pre-commit with tekwizely/pre-commit-golang

### Installation (Windows)

```powershell
# Option 1: pip (requires Python)
pip install pre-commit

# Option 2: pipx (isolated)
pipx install pre-commit

# Option 3: Scoop
scoop install pre-commit

# Option 4: Chocolatey
choco install python
pip install pre-commit
```

### Recommended .pre-commit-config.yaml

```yaml
# Pre-commit hooks configuration
# https://pre-commit.com/
# Install: pre-commit install
# Run all: pre-commit run --all-files

repos:
  # General hooks
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.6.0
    hooks:
      - id: trailing-whitespace
        args: [--markdown-linebreak-ext=md]
      - id: end-of-file-fixer
      - id: check-yaml
        args: [--allow-multiple-documents]
      - id: check-json
      - id: check-added-large-files
        args: [--maxkb=1000]
      - id: check-merge-conflict
      - id: detect-private-key
      - id: mixed-line-ending
        args: [--fix=lf]

  # Go-specific hooks
  - repo: https://github.com/tekwizely/pre-commit-golang
    rev: v1.0.0-rc.4
    hooks:
      # Format
      - id: go-fmt-repo
        stages: [pre-commit]

      # Imports
      - id: go-imports-repo
        args: [-local, 'github.com/NomadCrew/nomad-crew-backend']
        stages: [pre-commit]

      # Mod tidy
      - id: go-mod-tidy-repo
        stages: [pre-commit]

      # Vet (fast, catches common issues)
      - id: go-vet-repo-mod
        stages: [pre-commit]

      # golangci-lint (comprehensive)
      - id: golangci-lint-repo-mod
        args: [--timeout=5m, --new-from-rev=HEAD~1]
        stages: [pre-commit]

  # Commit message linting (optional but recommended)
  - repo: https://github.com/commitizen-tools/commitizen
    rev: v3.29.0
    hooks:
      - id: commitizen
        stages: [commit-msg]

# CI configuration
ci:
  autofix_commit_msg: |
    [pre-commit.ci] auto fixes from pre-commit hooks
  autofix_prs: true
  autoupdate_branch: ''
  autoupdate_commit_msg: '[pre-commit.ci] pre-commit autoupdate'
  autoupdate_schedule: weekly
  skip: []
  submodules: false
```

### Setup Script

Create `scripts/setup-devx.ps1` (Windows):

```powershell
#!/usr/bin/env pwsh
# DevX Setup Script for Windows

Write-Host "Setting up Developer Experience tools..." -ForegroundColor Cyan

# Check for required tools
$tools = @{
    "go" = "Go is required. Install from https://go.dev/dl/"
    "git" = "Git is required. Install from https://git-scm.com/"
}

foreach ($tool in $tools.Keys) {
    if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) {
        Write-Host "ERROR: $($tools[$tool])" -ForegroundColor Red
        exit 1
    }
}

# Install Task
Write-Host "Installing Task..." -ForegroundColor Yellow
if (Get-Command task -ErrorAction SilentlyContinue) {
    Write-Host "Task already installed" -ForegroundColor Green
} else {
    go install github.com/go-task/task/v3/cmd/task@latest
}

# Install golangci-lint
Write-Host "Installing golangci-lint..." -ForegroundColor Yellow
if (Get-Command golangci-lint -ErrorAction SilentlyContinue) {
    Write-Host "golangci-lint already installed" -ForegroundColor Green
} else {
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
}

# Install air (hot reload)
Write-Host "Installing air..." -ForegroundColor Yellow
if (Get-Command air -ErrorAction SilentlyContinue) {
    Write-Host "air already installed" -ForegroundColor Green
} else {
    go install github.com/air-verse/air@latest
}

# Install pre-commit
Write-Host "Installing pre-commit..." -ForegroundColor Yellow
if (Get-Command pre-commit -ErrorAction SilentlyContinue) {
    Write-Host "pre-commit already installed" -ForegroundColor Green
} else {
    pip install pre-commit
}

# Setup pre-commit hooks
Write-Host "Setting up pre-commit hooks..." -ForegroundColor Yellow
pre-commit install
pre-commit install --hook-type commit-msg

# Download dependencies
Write-Host "Downloading Go dependencies..." -ForegroundColor Yellow
go mod download
go mod verify

Write-Host "`nSetup complete!" -ForegroundColor Green
Write-Host "Run 'task' to see available commands" -ForegroundColor Cyan
```

**Estimated setup time:** 5-10 minutes (initial), < 1 minute per developer

---

## 4. Environment Variable Management

### Current State Analysis

The project has 4 `.env.example*` files:
- `.env.example` - Generic (inconsistent format)
- `.env.example.local` - Local development
- `.env.example.docker` - Docker Compose
- `.env.example.production` - Production template

**Problem:** Confusion about which to use, inconsistent variable sets.

### Recommendation: Single .env.example with Environment Markers

### Consolidated .env.example

```bash
# =============================================================================
# NomadCrew Backend Environment Configuration
# =============================================================================
# Copy this file to .env and update values for your environment
#
# Environment types:
#   [DEV]  = Development (local or Docker)
#   [PROD] = Production only
#   [ALL]  = All environments
# =============================================================================

# -----------------------------------------------------------------------------
# Server Configuration [ALL]
# -----------------------------------------------------------------------------
PORT=8080
SERVER_ENVIRONMENT=development  # development | staging | production
LOG_LEVEL=debug                 # debug | info | warn | error

# -----------------------------------------------------------------------------
# Database Configuration [ALL]
# -----------------------------------------------------------------------------
# Local development (without Docker):
# DATABASE_URL=postgres://postgres:password@localhost:5432/nomadcrew?sslmode=disable
#
# Docker Compose:
# DATABASE_URL=postgres://postgres:admin123@postgres:5432/nomadcrew?sslmode=disable
#
# Production (Neon/Supabase):
# DATABASE_URL=postgres://user:pass@host:5432/db?sslmode=require
DATABASE_URL=postgres://postgres:password@localhost:5432/nomadcrew?sslmode=disable

# Legacy individual settings (optional, DATABASE_URL takes precedence)
# DB_HOST=localhost
# DB_PORT=5432
# DB_USER=postgres
# DB_PASSWORD=password
# DB_NAME=nomadcrew
# DB_SSL_MODE=disable

# -----------------------------------------------------------------------------
# Redis Configuration [ALL]
# -----------------------------------------------------------------------------
# Local: localhost:6379
# Docker: redis:6379
# Production: your-redis.upstash.io:6379
REDIS_ADDRESS=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
# REDIS_USE_TLS=false  # [PROD] Set to true for Upstash/cloud Redis

# -----------------------------------------------------------------------------
# Supabase Configuration [ALL]
# -----------------------------------------------------------------------------
SUPABASE_URL=https://your-project.supabase.co
SUPABASE_ANON_KEY=your-anon-key
SUPABASE_SERVICE_KEY=your-service-key
SUPABASE_JWT_SECRET=your-jwt-secret-minimum-32-characters
# SUPABASE_PROJECT_ID=your-project-id  # Optional

# -----------------------------------------------------------------------------
# Security [ALL]
# -----------------------------------------------------------------------------
JWT_SECRET_KEY=development-only-secret-key-minimum-32-characters
# [PROD] Use a cryptographically random 64+ character string

# -----------------------------------------------------------------------------
# CORS & Frontend [ALL]
# -----------------------------------------------------------------------------
# Development:
ALLOWED_ORIGINS=http://localhost:3000,http://localhost:5173
FRONTEND_URL=http://localhost:3000
# Production:
# ALLOWED_ORIGINS=https://nomadcrew.com,https://www.nomadcrew.com
# FRONTEND_URL=https://nomadcrew.com

# -----------------------------------------------------------------------------
# Email (Resend) [ALL]
# -----------------------------------------------------------------------------
EMAIL_FROM_ADDRESS=noreply@nomadcrew.com
EMAIL_FROM_NAME=NomadCrew
RESEND_API_KEY=re_xxxxxxxxxxxx

# -----------------------------------------------------------------------------
# External APIs [ALL]
# -----------------------------------------------------------------------------
GEOAPIFY_KEY=your-geoapify-key
PEXELS_API_KEY=your-pexels-key

# -----------------------------------------------------------------------------
# Rate Limiting [ALL]
# -----------------------------------------------------------------------------
RATE_LIMIT_AUTH_REQUESTS_PER_MINUTE=10
RATE_LIMIT_WINDOW_SECONDS=60

# -----------------------------------------------------------------------------
# Event Service [DEV]
# -----------------------------------------------------------------------------
# EVENT_SERVICE_PUBLISH_TIMEOUT_SECONDS=5
# EVENT_SERVICE_SUBSCRIBE_TIMEOUT_SECONDS=10
# EVENT_SERVICE_EVENT_BUFFER_SIZE=100

# -----------------------------------------------------------------------------
# Performance Tuning [PROD]
# -----------------------------------------------------------------------------
# DATABASE_MAX_OPEN_CONNS=25
# DATABASE_MAX_IDLE_CONNS=10
# DATABASE_CONN_MAX_LIFE=30m
# REDIS_POOL_SIZE=15
# REDIS_MIN_IDLE_CONNS=5

# -----------------------------------------------------------------------------
# AWS (if using AWS services) [PROD]
# -----------------------------------------------------------------------------
# AWS_REGION=us-east-1
# AWS_SECRETS_PATH=/nomadcrew/prod/
```

### Quick-start Environment Files

Provide minimal quick-start files:

**.env.docker** (copy from .env.example, Docker-ready):
```bash
# Quick start for Docker Compose
# Copy to .env: cp .env.docker .env

DATABASE_URL=postgres://postgres:admin123@postgres:5432/nomadcrew?sslmode=disable
REDIS_ADDRESS=redis:6379
REDIS_PASSWORD=redispass
PORT=8080
SERVER_ENVIRONMENT=development
LOG_LEVEL=debug
# ... add your API keys below
```

**Estimated refactoring time:** 30 minutes

---

## 5. IDE Integration

### .editorconfig

```ini
# EditorConfig for NomadCrew Backend
# https://editorconfig.org/

root = true

# Default for all files
[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true
indent_style = space
indent_size = 2

# Go files use tabs (Go standard)
[*.go]
indent_style = tab
indent_size = 4
tab_width = 4

# Makefiles require tabs
[Makefile]
indent_style = tab

# YAML files
[*.{yml,yaml}]
indent_style = space
indent_size = 2

# JSON files
[*.json]
indent_style = space
indent_size = 2

# Markdown
[*.md]
trim_trailing_whitespace = false
max_line_length = 120

# Shell scripts
[*.sh]
indent_style = space
indent_size = 2
end_of_line = lf

# PowerShell scripts
[*.ps1]
indent_style = space
indent_size = 4
end_of_line = crlf

# SQL files
[*.sql]
indent_style = space
indent_size = 2

# Docker files
[Dockerfile*]
indent_style = space
indent_size = 2

# Git config
[.git*]
indent_style = space
indent_size = 2

# Ignore generated files
[static/docs/**]
generated_code = true
```

### .vscode/settings.json

```json
{
  // Go settings
  "go.useLanguageServer": true,
  "go.lintTool": "golangci-lint",
  "go.lintFlags": ["--fast"],
  "go.formatTool": "gofumpt",

  // gopls settings
  "gopls": {
    "formatting.gofumpt": true,
    "ui.semanticTokens": true,
    "ui.completion.usePlaceholders": true,
    "ui.diagnostic.analyses": {
      "unusedparams": true,
      "shadow": true,
      "nilness": true,
      "unusedwrite": true
    },
    "build.directoryFilters": [
      "-node_modules",
      "-vendor",
      "-static/docs"
    ]
  },

  // Editor settings for Go
  "[go]": {
    "editor.formatOnSave": true,
    "editor.codeActionsOnSave": {
      "source.organizeImports": "explicit"
    },
    "editor.defaultFormatter": "golang.go",
    "editor.tabSize": 4,
    "editor.insertSpaces": false,
    "editor.rulers": [100, 120]
  },

  "[go.mod]": {
    "editor.formatOnSave": true
  },

  // General settings
  "editor.formatOnSave": true,
  "files.trimTrailingWhitespace": true,
  "files.insertFinalNewline": true,
  "files.trimFinalNewlines": true,

  // File associations
  "files.associations": {
    "Taskfile.yml": "yaml",
    ".golangci.yml": "yaml",
    ".air.toml": "toml",
    "*.env*": "properties"
  },

  // Exclude from search/watch
  "files.exclude": {
    "**/tmp": true,
    "**/vendor": true,
    "**/.git": true,
    "**/coverage.out": true,
    "**/coverage.html": true
  },

  // Search exclude
  "search.exclude": {
    "**/vendor": true,
    "**/static/docs": true,
    "**/tmp": true,
    "**/coverage.*": true
  },

  // Terminal settings for Windows
  "terminal.integrated.defaultProfile.windows": "PowerShell",

  // Task runner integration
  "task.autoDetect": "on"
}
```

### .vscode/extensions.json

```json
{
  "recommendations": [
    // Required
    "golang.go",
    "EditorConfig.EditorConfig",

    // Highly recommended
    "eamodio.gitlens",
    "tamasfe.even-better-toml",
    "redhat.vscode-yaml",

    // Optional but useful
    "streetsidesoftware.code-spell-checker",
    "yzhang.markdown-all-in-one",
    "ms-azuretools.vscode-docker",
    "task.vscode-task"
  ]
}
```

### .vscode/launch.json

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Launch Server",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}/main.go",
      "env": {
        "SERVER_ENVIRONMENT": "development",
        "LOG_LEVEL": "debug"
      },
      "envFile": "${workspaceFolder}/.env"
    },
    {
      "name": "Debug Current Test",
      "type": "go",
      "request": "launch",
      "mode": "test",
      "program": "${fileDirname}",
      "showLog": true
    },
    {
      "name": "Debug All Tests",
      "type": "go",
      "request": "launch",
      "mode": "test",
      "program": "${workspaceFolder}/...",
      "args": ["-v"]
    }
  ]
}
```

**Estimated setup time:** 10 minutes

---

## 6. Implementation Roadmap

### Phase 1: Foundation (Day 1) - 1 hour

1. Create `.editorconfig`
2. Create `.vscode/settings.json`, `extensions.json`, `launch.json`
3. Create `.golangci.yml` (v2 format)

### Phase 2: Build Tooling (Day 1) - 30 minutes

1. Install Task: `go install github.com/go-task/task/v3/cmd/task@latest`
2. Create `Taskfile.yml`
3. Test: `task --list`

### Phase 3: Pre-commit (Day 1) - 30 minutes

1. Install pre-commit
2. Create `.pre-commit-config.yaml`
3. Run `pre-commit install`
4. Test: `pre-commit run --all-files`

### Phase 4: Environment Cleanup (Day 2) - 1 hour

1. Consolidate to single `.env.example`
2. Create `.env.docker` quick-start
3. Update documentation
4. Remove redundant `.env.example.*` files

### Phase 5: CI Update (Day 2) - 30 minutes

1. Update golangci-lint action to v2
2. Test workflow

**Total estimated time:** 3.5 hours

---

## 7. Summary of Deliverables

| File | Purpose | Priority |
|------|---------|----------|
| `Taskfile.yml` | Task runner (replaces Makefile) | HIGH |
| `.golangci.yml` | Linter configuration (v2) | HIGH |
| `.pre-commit-config.yaml` | Pre-commit hooks | HIGH |
| `.editorconfig` | Editor formatting consistency | MEDIUM |
| `.vscode/settings.json` | VS Code Go settings | MEDIUM |
| `.vscode/extensions.json` | Recommended extensions | LOW |
| `.vscode/launch.json` | Debug configurations | LOW |
| `.env.example` (consolidated) | Environment template | MEDIUM |
| `scripts/setup-devx.ps1` | Windows setup script | LOW |

---

## Sources

### Build Tools
- [Just Make a Task (Make vs. Taskfile vs. Just) - Applied Go](https://appliedgo.net/spotlight/just-make-a-task/)
- [Task vs Make - DevCube](https://rnemet.dev/posts/tools/tasklist_final/)
- [Task Official Documentation](https://taskfile.dev/)
- [Task Windows Core Utils Announcement](https://taskfile.dev/blog/windows-core-utils)

### golangci-lint
- [golangci-lint Official Configuration](https://golangci-lint.run/docs/configuration/)
- [golangci-lint v2 Configuration File](https://golangci-lint.run/docs/configuration/file/)
- [Welcome to golangci-lint v2](https://ldez.github.io/blog/2025/03/23/golangci-lint-v2/)
- [Golden config for golangci-lint (maratori)](https://gist.github.com/maratori/47a4d00457a92aa426dbd48a18776322)
- [Migrating to GolangCI-Lint v2](https://www.khajaomer.com/blog/level-up-your-go-linting)

### Pre-commit
- [pre-commit Official](https://pre-commit.com/)
- [tekwizely/pre-commit-golang](https://github.com/tekwizely/pre-commit-golang)
- [golangci-lint Pre-commit Hooks](https://github.com/golangci/golangci-lint/blob/main/.pre-commit-hooks.yaml)

### IDE Integration
- [VS Code Go Extension Wiki](https://github.com/golang/vscode-go/wiki/settings)
- [EditorConfig](https://editorconfig.org/)
- [VS Code Setup for Golang 2025](https://dipjyotimetia.medium.com/vs-code-setup-for-golang-development-in-2025-57ba0a50881c)
