# Waypoint — Architecture

Waypoint is a self-hosted travel planner, logger, and tracker. It ships as a
single Docker image (Go binary with the built web UI embedded) plus a Postgres
database.

## High-level design

```
┌──────────────────────────────┐
│  Browser (React SPA)         │
│  Vite + TS + Tailwind        │
│  MapLibre GL for maps        │
└──────────────┬───────────────┘
               │ HTTPS (JSON API + static assets)
┌──────────────▼───────────────┐
│  Go server (single binary)   │
│  ├─ /api/v1/*  JSON API      │
│  ├─ /auth/*    OIDC + local  │
│  └─ /*         embedded SPA  │
└──────────────┬───────────────┘
               │ pgx pool
┌──────────────▼───────────────┐
│  PostgreSQL 16               │
└──────────────────────────────┘
```

## Backend (Go)

- **Go 1.24+**, standard-library `net/http` with the 1.22+ `ServeMux` method
  patterns. No web framework; middleware is plain `func(http.Handler) http.Handler`.
- **Database access:** [pgx/v5](https://github.com/jackc/pgx) connection pool.
  Queries start as hand-written SQL in store packages; we adopt
  [sqlc](https://sqlc.dev) once the query surface grows (see ROADMAP M2).
- **Migrations:** [goose](https://github.com/pressly/goose) with SQL files
  embedded in the binary (`migrations/`). Applied automatically at startup —
  a self-hosted app should never require a manual migration step.
- **Package layout:**

```
cmd/server/          entrypoint: config → db → migrate → serve
internal/
  config/            env-var configuration (WAYPOINT_* vars)
  server/            router, middleware, handler wiring
  auth/              Google OIDC, local login, session management   (M1)
  store/             one package per aggregate: users, trips, ...   (M1+)
migrations/          goose SQL migrations, embedded via go:embed
```

## Authentication

Primary: **Google Sign-In via OIDC** (`coreos/go-oidc` + `golang.org/x/oauth2`).
Standard authorization-code flow — no ID-token-in-JS, no Google SDK in the
frontend. The server redirects to Google, handles the callback, and creates a
session.

Secondary: **local users** (email + password, bcrypt/argon2id), primarily for
development and testing, enabled by `WAYPOINT_LOCAL_AUTH=true`. Disabled by
default in production guidance.

Sessions are server-side rows in Postgres referenced by an HttpOnly, Secure,
SameSite=Lax cookie. No JWTs — sessions are revocable and this is a
single-server app; stateless tokens buy nothing here.

First user to sign in becomes admin. Subsequent sign-ups can be restricted via
`WAYPOINT_ALLOWED_EMAILS` / open-registration toggle (self-hosted instances are
often single-user or family-scale).

## Frontend (web/)

- **React 19 + TypeScript + Vite** — SPA in `web/`, dev-proxied to the Go
  server, embedded into the binary (`go:embed`) for release builds.
- **Tailwind CSS + shadcn/ui** for styling and components.
- **TanStack Query** for server state; **TanStack Router** for routing.
- **MapLibre GL JS** for all map views — open-source, no API key, works with
  self-hostable tile servers. Default raster tiles from OpenStreetMap with a
  configurable tile URL so operators can point at their own tile server or a
  commercial provider.

Why a SPA instead of Go templates/HTMX: the core screens (itinerary board,
map with route lines, track playback) are interaction-heavy and map-centric;
MapLibre and drag-and-drop itinerary editing want a real client framework.

## API conventions

- JSON over `/api/v1/…`, standard REST resource shapes.
- Errors: `{"error": {"code": "...", "message": "..."}}` with appropriate
  HTTP status.
- Auth: session cookie; `401` when missing/expired. CSRF protected via
  SameSite=Lax + custom-header check on mutating requests.
- Tracking ingestion (M5) additionally accepts a per-device bearer token so
  phones can POST location without a browser session (OwnTracks-compatible
  endpoint is the goal).

## Deployment

- Single multi-stage `Dockerfile`: build web → build Go (embedding `web/dist`)
  → distroless/alpine runtime.
- `docker-compose.yml` runs `app` + `postgres` for both development and a
  reference self-hosted deployment.
- All configuration via `WAYPOINT_*` environment variables (see `.env.example`).
  No config files.

## Testing

- Go: table-driven unit tests; store-layer tests against a real Postgres
  (dockerized) rather than mocks.
- Frontend: Vitest + React Testing Library for logic-bearing components.
- CI (GitHub Actions): `go vet` + `go test ./...`, `tsc` + `vite build`.
