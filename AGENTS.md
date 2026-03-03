# Repository Guidelines

## Project Structure & Module Organization
This repository is a Go module (`go.mod`) for a WhatsApp bot built with `whatsmeow`.

- `cmd/bot/main.go`: application entrypoint (startup, auth flow, message handling, command routing).
- `README.md`: runtime usage and CLI flag examples.
- `AGENTS.md`: contributor guide (this file).

When adding features, keep executable entrypoints under `cmd/` and move reusable logic into internal packages (for example: `internal/config`, `internal/commands`, `internal/auth`) to avoid a growing `main.go`.

## Build, Test, and Development Commands
Run all commands from repository root:

- `go mod tidy`: resolve and clean dependencies.
- `go build ./...`: compile all packages to verify code correctness.
- `go test ./...`: run all tests.
- `go run ./cmd/bot --auth both --pair-phone 62812xxxxxx --owner 62812xxxxxx`: start bot locally.

Use `go run ./cmd/bot --auth qr` when testing QR-only login.

## Coding Style & Naming Conventions
- Follow standard Go formatting (`gofmt`) and idioms.
- Use tabs (Go default), short functions, and explicit error handling.
- Naming:
  - Exported identifiers: `PascalCase`
  - Unexported identifiers: `camelCase`
  - Constants: `camelCase` or `PascalCase` when exported
- Keep command names lowercase (`ping`, `setprefix`) and user-facing text concise.

Before opening a PR, run:
`gofmt -w . && go test ./... && go build ./...`

## Testing Guidelines
Use Go’s built-in `testing` package.

- Test files should end with `_test.go`.
- Prefer table-driven tests for parsers and command handlers.
- Minimum target areas:
  - prefix parsing/validation
  - owner authorization checks
  - config load/save behavior

Integration tests that require WhatsApp network auth should be optional/skipped by default.

## Commit & Pull Request Guidelines
Git history is not available yet in this workspace, so use this convention going forward:

- Commit format: `type(scope): summary`  
  Example: `feat(commands): add setowner authorization check`
- Common types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`.

PRs should include:
- clear change summary and reason
- test evidence (`go test ./...`, `go build ./...`)
- config or behavior impact notes (especially auth/prefix changes)
