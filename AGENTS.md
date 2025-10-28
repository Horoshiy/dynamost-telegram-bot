# Repository Guidelines

## Project Structure & Module Organization
Core bot code is organized under `cmd/bot` (entrypoint) and `internal/` packages grouped by responsibility: `config` for env loading, `telegram` for updates, `service` for business logic, `repository/pg` for PostgreSQL access, and `session` for admin wizards. Database migrations live in `migrations/`, assets and specs in `docs/`. Keep tests alongside source files using `_test.go` suffix. Add new packages under `internal/` rather than creating additional top-level folders.

## Build, Test, and Development Commands
Use Go 1.22+. Bootstrap dependencies with `go mod tidy`. Run the bot locally via `make run` (equivalent to `go run ./cmd/bot`). Apply migrations against your database using `make migrate-up`; roll back with `make migrate-down`. Execute the full unit-test suite with `go test ./...`, and run targeted packages as needed (e.g., `go test ./internal/telegram`). Include `golangci-lint run` before submitting changes if you add the linter configuration.

## Coding Style & Naming Conventions
Follow standard Go conventions enforced by `gofmt`. Use tabs for indentation and keep lines under 120 characters. Exported identifiers require doc comments; prefer descriptive names like `MatchesService` over abbreviations. Private helpers should use lowerCamelCase. Group Telegram callback keys as `verb|k=v` per the existing docs. Place SQL in migrations or repository files; avoid inline SQL in handlers.

## Testing Guidelines
Write table-driven tests with `_test.go` files mirroring package structure. Target critical flows: inline command routing, wizard state transitions, and repository interactions (using pgx test containers or fakes). When adding new behavior, cover happy path and error handling. Aim to maintain or improve coverage; treat failing tests as release blockers. Run `go test ./...` before every push.

## Commit & Pull Request Guidelines
Adopt Conventional Commits (`feat:`, `fix:`, `chore:`) to convey intent. Keep commits focused and include migration files and docs updates in the same change when they are related. Pull requests should describe the motivation, summarize implementation details, list test evidence (`go test`, manual bot checks), and link any relevant tasks. Add screenshots or terminal captures when altering bot UX.

## Environment & Security Notes
Store secrets in `.env` (see `.env.example`). Never commit real tokens or production DSNs. For local work, use dedicated bot tokens and database roles. Review SQL for parameter binding to avoid injection, and ensure new handlers validate `ADMIN_TELEGRAM_IDS` before mutating data.
