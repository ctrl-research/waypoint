# Waypoint — Roadmap

Milestones are sized to ship independently; each ends with something usable.

Work is tracked as [GitHub issues](https://github.com/ctrl-research/waypoint/issues)
under [milestones](https://github.com/ctrl-research/waypoint/milestones) M1–M6 —
issue numbers below. The tracker is the source of truth for status; this file
is the narrative overview.

## M0 — Skeleton *(this repo state)*
- [x] Repo docs, architecture and data-model design
- [x] Go server skeleton: config, pgx pool, embedded goose migrations, `/healthz`
- [x] React + TS + Vite scaffold in `web/`
- [x] docker-compose (app + Postgres 16), multi-stage Dockerfile, Makefile
- [x] CI: go vet/test + frontend typecheck/build

## M1 — Auth & users
- [ ] `users` + `sessions` migrations (#1)
- [ ] Google OIDC flow (`/auth/google` → callback → session cookie) (#2)
- [ ] Local email/password auth behind `WAYPOINT_LOCAL_AUTH` (dev/testing) (#3)
- [ ] First-user-is-admin; `WAYPOINT_ALLOWED_EMAILS` allowlist (#4)
- [ ] Frontend: login page, session-aware shell, sign-out (#5)
- [ ] Seed command for a local test user (`make seed`) (#6)

## M2 — Trips & itinerary (the planner)
- [ ] Trips CRUD (`trips`, `stops`, `itinerary_items`) (#7)
- [ ] Adopt sqlc for the store layer (#8)
- [ ] Trip list + trip detail pages; create/edit forms (#9)
- [ ] Day-by-day itinerary view with drag-to-reorder (#10)
- [ ] Place search for stops (Nominatim geocoding, configurable endpoint) (#11)

## M3 — Maps
- [ ] MapLibre GL map on trip detail: stop markers + route line (#12)
- [ ] Configurable tile server (`WAYPOINT_TILE_URL`, OSM default) (#13)
- [ ] Pick-on-map for stop coordinates (#14)

## M4 — Journal (the logger)
- [ ] `journal_entries` + `journal_photos`; photo upload to `WAYPOINT_DATA_DIR` (#15)
- [ ] EXIF extraction (timestamp + GPS) on upload (#16)
- [ ] Trip timeline view mixing itinerary and journal entries (#17)
- [ ] Markdown editor for entries (#18)

## M5 — Location tracking (the tracker)
- [ ] `devices` + `track_points`; per-device bearer tokens (#19)
- [ ] OwnTracks-compatible ingestion endpoint (#20)
- [ ] Track rendering on the trip map; simple playback slider (#21)
- [ ] Auto-associate track points with the active trip (#22)

## M6 — Sharing & polish
- [ ] `trip_members` (viewer/editor roles), invite by email (#23)
- [ ] Public read-only share links for completed trips (#24)
- [ ] Export: trip → GPX/GeoJSON, journal → Markdown bundle (#25)
- [ ] Stats page (countries visited, distance traveled) (#26)

## Deliberately out of scope (for now)
- Mobile apps (OwnTracks covers tracking; the SPA is responsive)
- Multi-tenancy beyond simple user accounts on one instance
- Flight/hotel booking API integrations
- PostGIS — plain lat/lon columns until a feature needs spatial queries
