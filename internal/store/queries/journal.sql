-- name: CreateJournalEntry :one
INSERT INTO journal_entries (trip_id, author_id, entry_date, title, body, lat, lon)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListJournalEntries :many
SELECT * FROM journal_entries
WHERE trip_id = $1
ORDER BY entry_date, created_at;

-- name: JournalEntryByID :one
SELECT * FROM journal_entries WHERE id = $2 AND trip_id = $1;

-- name: UpdateJournalEntry :one
UPDATE journal_entries
SET entry_date = $3, title = $4, body = $5, lat = $6, lon = $7, updated_at = now()
WHERE id = $2 AND trip_id = $1
RETURNING *;

-- name: DeleteJournalEntry :execrows
DELETE FROM journal_entries WHERE id = $2 AND trip_id = $1;

-- name: CreateJournalPhoto :one
-- The ID is supplied by the caller because the file on disk is named after it.
INSERT INTO journal_photos (id, entry_id, file_path, content_type, size_bytes, taken_at, lat, lon, caption)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListJournalPhotosForTrip :many
-- Photos for all of a trip's entries, for embedding into the journal listing.
SELECT p.* FROM journal_photos p
JOIN journal_entries e ON e.id = p.entry_id
WHERE e.trip_id = $1
ORDER BY p.taken_at NULLS LAST, p.created_at;

-- name: ListJournalPhotosForEntry :many
SELECT * FROM journal_photos WHERE entry_id = $1 ORDER BY taken_at NULLS LAST, created_at;

-- name: JournalPhotoWithTrip :one
-- Resolves a photo to its file plus its trip, for role-checked serving.
SELECT p.*, e.trip_id AS photo_trip_id FROM journal_photos p
JOIN journal_entries e ON e.id = p.entry_id
WHERE p.id = $1;

-- name: DeleteJournalPhoto :one
-- Returns the deleted row so the caller can remove the file from disk.
DELETE FROM journal_photos p
USING journal_entries e
WHERE p.id = $2 AND p.entry_id = e.id AND e.trip_id = $1
RETURNING p.*;
