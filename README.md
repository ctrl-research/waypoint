# Waypoint

Self-hosted travel planner, logger, and tracker.

- **Plan** trips: destinations, day-by-day itineraries, costs — laid out on a map
- **Log** the journey: journal entries and photos tied to places and dates
- **Track** where you've been: GPS ingestion (OwnTracks-compatible) rendered as routes

Waypoint ships as a single Docker image (Go binary with the web UI embedded)
plus PostgreSQL. Sign in with Google, or local accounts for development.

> **Status:** early development — M0 skeleton. See [docs/ROADMAP.md](docs/ROADMAP.md).

## Stack

| Layer     | Choice                                                      |
|-----------|-------------------------------------------------------------|
| Backend   | Go (stdlib `net/http`), pgx, goose migrations               |
| Database  | PostgreSQL 16                                               |
| Frontend  | React + TypeScript + Vite, Tailwind, MapLibre GL            |
| Auth      | Google OIDC (primary), local users for dev/testing          |
| Deploy    | Docker Compose (app + postgres)                             |

Design details: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) ·
[docs/DATA_MODEL.md](docs/DATA_MODEL.md) · [docs/ROADMAP.md](docs/ROADMAP.md)

## Development

Requirements: Go 1.24+, Node 22+, Docker.

```sh
cp .env.example .env      # then fill in values
make db                   # start postgres via docker compose
make run                  # run the Go server (applies migrations on boot)
make web                  # vite dev server on :5173, proxies /api to :8080
```

Other targets:

```sh
make test                 # go vet + go test ./...
make build                # build web UI + server binary with UI embedded
make docker               # build the full docker image
```

## Self-hosting

```sh
cp .env.example .env      # set WAYPOINT_SESSION_SECRET, Google OAuth creds, etc.
docker compose up -d
```

The app listens on `:8080`. Put it behind your reverse proxy with TLS.
Google sign-in requires an OAuth client (redirect URI:
`https://your-host/auth/google/callback`); local accounts can be enabled with
`WAYPOINT_LOCAL_AUTH=true` instead.

Prebuilt multi-arch images (amd64/arm64) are on GHCR:

```sh
docker pull ghcr.io/ctrl-research/waypoint:latest   # or a specific X.Y.Z
```

## Releases

Every merge to `main` bumps the patch version, tags the repo, and pushes
`ghcr.io/ctrl-research/waypoint:X.Y.Z` (+ `latest`). Put `[minor]` or
`[major]` in the PR title to bump higher. To cut a curated release — a
GitHub Release with notes, optionally an extra image tag like `stable` —
run the **Release** workflow manually from the Actions tab and pick the
bump and tag.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). All changes go through PRs against
`main`.
