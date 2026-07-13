# Waypoint — Data Model

Postgres schema, evolved via goose migrations in `migrations/`. This document
is the design reference; migrations are the source of truth.

## Entity overview

```
users ─┬─ sessions
       ├─ trips ─┬─ stops ─── itinerary_items
       │         ├─ journal_entries ─── journal_photos
       │         ├─ track_points
       │         └─ trip_members (sharing, M6)
       └─ devices (tracking tokens, M5)
```

## Core tables

### users (M1)
| column        | type        | notes                                   |
|---------------|-------------|-----------------------------------------|
| id            | uuid PK     | `gen_random_uuid()`                     |
| email         | citext UQ   |                                         |
| display_name  | text        |                                         |
| avatar_url    | text NULL   | from Google profile                     |
| google_sub    | text UQ NULL| OIDC subject; NULL for local users      |
| password_hash | text NULL   | argon2id; NULL for Google users         |
| is_admin      | bool        | first user = true                       |
| created_at    | timestamptz |                                         |

### sessions (M1)
`token_hash (bytea PK, SHA-256 of the cookie token) · user_id FK · created_at ·
expires_at · last_seen_at`

Users must have at least one credential (`google_sub` or `password_hash`),
enforced by a CHECK constraint.

### trips (M2)
| column      | type                                        |
|-------------|---------------------------------------------|
| id          | uuid PK                                     |
| owner_id    | uuid FK → users                             |
| title       | text                                        |
| description | text                                        |
| status      | enum: `planning` `active` `completed`       |
| start_date  | date NULL                                   |
| end_date    | date NULL                                   |
| cover_photo | text NULL                                   |
| created_at / updated_at | timestamptz                     |

### stops (M2) — the places a trip visits, ordered
`id · trip_id FK · name · lat/lon (double precision, CHECK both-or-neither) ·
arrival_date NULL · departure_date NULL · position (int, ordering) · notes`

Geometry stays as plain lat/lon columns until we need spatial queries
(nearby search, route simplification) — then enable PostGIS. Don't add the
extension before something uses it.

### itinerary_items (M2) — what happens at a stop, day by day
`id · trip_id FK · stop_id FK NULL (ON DELETE SET NULL) ·
destination_stop_id FK NULL · origin_home_id / destination_home_id FK NULL
(flight/train legs travel stop-or-home → stop-or-home) · day (date) ·
start_time NULL · end_time NULL · title · category (enum: activity, food,
lodging, transport, flight, train, other) · notes · cost_cents NULL ·
currency (char(3)) · address · lat/lon NULL (venue, both-or-neither CHECK) ·
position`

### homes
`id · user_id FK · name · lat/lon (NOT NULL) · created_at`

Per-user saved locations ("(home) Toronto") usable as flight/train leg
endpoints; deleting one nulls the legs that referenced it.

cost_cents/currency are both-or-neither (CHECK). Position ordering is scoped
per day for items, per trip for stops.

### journal_entries (M4) — the "logger"
`id · trip_id FK · author_id FK · entry_date · title NULL · body (markdown) ·
lat/lon NULL · created_at / updated_at`

### journal_photos (M4)
`id · entry_id FK · file_path · taken_at NULL · lat/lon NULL (EXIF) · caption`
Binary data lives on disk under `WAYPOINT_DATA_DIR`, not in Postgres.

### track_points (M5) — the "tracker"
`id bigserial · trip_id FK · device_id FK · recorded_at timestamptz ·
lat/lon · altitude_m NULL · accuracy_m NULL · battery NULL`

Index `(trip_id, recorded_at)`. High-volume append-only; partition later if
real usage demands it, not before.

### devices (M5)
`id · user_id FK · name · token_hash UQ · created_at · last_seen_at`
Per-device bearer tokens for the ingestion endpoint (OwnTracks-compatible).

### trip_members (M6, sharing)
`trip_id FK · user_id FK · role (enum: viewer, editor) · PK(trip_id, user_id)`

## Conventions

- `uuid` PKs for user-facing entities; `bigserial` for high-volume append-only
  rows (`track_points`).
- All timestamps `timestamptz`, UTC in, local rendering in the UI.
- Soft deletes are not used; deletes cascade (`ON DELETE CASCADE` from trips
  down).
- Money as integer cents + ISO currency code.
