# Waypoint — Roadmap

Milestones are sized to ship independently; each ends with something usable.

## M0 — Skeleton *(this repo state)*
- [x] Repo docs, architecture and data-model design
- [x] Go server skeleton: config, pgx pool, embedded goose migrations, `/healthz`
- [x] React + TS + Vite scaffold in `web/`
- [x] docker-compose (app + Postgres 16), multi-stage Dockerfile, Makefile
- [x] CI: go vet/test + frontend typecheck/build

## M1 — Auth & users
- [ ] `users` + `sessions` migrations
- [ ] Google OIDC flow (`/auth/google` → callback → session cookie)
- [ ] Local email/password auth behind `WAYPOINT_LOCAL_AUTH` (dev/testing)
- [ ] First-user-is-admin; `WAYPOINT_ALLOWED_EMAILS` allowlist
- [ ] Frontend: login page, session-aware shell, sign-out
- [ ] Seed command for a local test user (`make seed`)

## M2 — Trips & itinerary (the planner)
- [ ] Trips CRUD (`trips`, `stops`, `itinerary_items`)
- [ ] Adopt sqlc for the store layer
- [ ] Trip list + trip detail pages; create/edit forms
- [ ] Day-by-day itinerary view with drag-to-reorder
- [ ] Place search for stops (Nominatim geocoding, configurable endpoint)

## M3 — Maps
- [ ] MapLibre GL map on trip detail: stop markers + route line
- [ ] Configurable tile server (`WAYPOINT_TILE_URL`, OSM default)
- [ ] Pick-on-map for stop coordinates

## M4 — Journal (the logger)
- [ ] `journal_entries` + `journal_photos`; photo upload to `WAYPOINT_DATA_DIR`
- [ ] EXIF extraction (timestamp + GPS) on upload
- [ ] Trip timeline view mixing itinerary and journal entries
- [ ] Markdown editor for entries

## M5 — Location tracking (the tracker)
- [ ] `devices` + `track_points`; per-device bearer tokens
- [ ] OwnTracks-compatible ingestion endpoint
- [ ] Track rendering on the trip map; simple playback slider
- [ ] Auto-associate track points with the active trip

## M6 — Sharing & polish
- [ ] `trip_members` (viewer/editor roles), invite by email
- [ ] Public read-only share links for completed trips
- [ ] Export: trip → GPX/GeoJSON, journal → Markdown bundle
- [ ] Stats page (countries visited, distance traveled)

## Deliberately out of scope (for now)
- Mobile apps (OwnTracks covers tracking; the SPA is responsive)
- Multi-tenancy beyond simple user accounts on one instance
- Flight/hotel booking API integrations
- PostGIS — plain lat/lon columns until a feature needs spatial queries
