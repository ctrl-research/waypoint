# CLAUDE.md

## Purpose

Waypoint — a self-hosted travel planner, logger, and tracker. Go + Postgres
backend, React SPA frontend, Google OIDC auth. Single-binary deployment with
the web UI embedded.

## Tech stack

- **Go 1.24+** — stdlib `net/http` (1.22+ ServeMux patterns), no framework
- **PostgreSQL 16** — pgx/v5 pool, goose migrations embedded and run at startup
- **React + TypeScript + Vite** in `web/` — Tailwind, TanStack Query/Router, MapLibre GL
- **Auth** — Google OIDC + optional local users; server-side sessions in Postgres, cookie-based

## Structure

```
cmd/server/        entrypoint (config → db → migrate → serve)
internal/config/   WAYPOINT_* env configuration
internal/server/   router, middleware, handlers
migrations/        goose SQL migrations (go:embed)
web/               React SPA (Vite)
docs/              ARCHITECTURE.md, DATA_MODEL.md, ROADMAP.md
```

## Commands

```
make db       start postgres (docker compose)
make run      run Go server on :8080
make web      vite dev server on :5173 (proxies /api → :8080)
make test     go vet + go test ./...
make build    build web + embed into server binary
```

## Conventions

- Read `docs/ARCHITECTURE.md` before adding backend packages; `docs/DATA_MODEL.md`
  before writing migrations. Migrations are append-only, numbered `NNNNN_name.sql`,
  goose format.
- Configuration only via `WAYPOINT_*` env vars — never config files.
- API: JSON under `/api/v1`, errors as `{"error":{"code","message"}}`.
- Store-layer tests run against real Postgres, not mocks.
- Work is tracked as GitHub issues under milestones M1–M6 (`gh issue list`);
  reference the issue in the PR (`Closes #N`). docs/ROADMAP.md maps items to issues.
- Branch protection: never push directly to `main`; all changes via PR with review.
- Conventional commits (see CONTRIBUTING.md); branch names `feat|bug|hotfix|release|chore/short-description`.
