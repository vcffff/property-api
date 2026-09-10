# CLAUDE.md

## Project

Go REST API (task manager) built with Gin + GORM (PostgreSQL), JWT auth, rate limiting.

## Commands

- Build: `go build ./...`
- Test: `go test ./...`
- Vet: `go vet ./...`
- Run: `go run ./cmd/api` (listens on :8080)
- Docker build: `docker build -t api-task-manager .`

## Environment

Requires `.env` in project root (not committed):

- `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`

## Architecture

Feature-first layout:

```
cmd/api/              — entrypoint, wiring (main.go)
configs/              — env config loading
internal/
  features/
    auth/             — auth feature: dto, handler, service, jwt
    user/             — user model
    property/         — property model + repository + service
  platform/
    database/         — DB connection (GORM/Postgres)
    middleware/       — auth middleware (JWT validation)
    middleware/limiter/ — rate limiter + tests
  routes/             — route registration
```

Rules:
- New features go into `internal/features/<name>/` — keep handler/service/dto/model together in one package.
- Cross-cutting infrastructure goes into `internal/platform/`.
- Routes are registered only in `internal/routes/routes.go`.

## API

- `POST /api/v1/register`
- `POST /api/v1/login`
- `GET /api/v1/users/me` (Bearer token required)
