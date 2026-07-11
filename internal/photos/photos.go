// Package photos stores uploaded journal photos on disk under
// WAYPOINT_DATA_DIR (never in Postgres) and extracts EXIF metadata.
package photos

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/rwcarlsen/goexif/exif"
)

// allowedTypes maps accepted image content types to file extensions.
var allowedTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

var ErrUnsupportedType = errors.New("unsupported image type (use JPEG, PNG, WebP, or GIF)")

type Store struct {
	dataDir string
}

func NewStore(dataDir string) *Store {
	return &Store{dataDir: dataDir}
}

// Save writes an uploaded photo under photos/<tripID>/<photoID><ext> and
// returns its repo-relative path and size. The path is stored in the DB; the
// tripID grouping lets trip deletion remove the whole directory.
func (s *Store) Save(tripID, photoID uuid.UUID, contentType string, r io.Reader) (string, int64, error) {
	ext, ok := allowedTypes[contentType]
	if !ok {
		return "", 0, ErrUnsupportedType
	}

	rel := filepath.Join("photos", tripID.String(), photoID.String()+ext)
	abs := filepath.Join(s.dataDir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", 0, err
	}

	f, err := os.Create(abs)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	size, err := io.Copy(f, r)
	if err != nil {
		os.Remove(abs)
		return "", 0, err
	}
	return rel, size, nil
}

// Open opens a stored photo by its repo-relative path.
func (s *Store) Open(relPath string) (*os.File, error) {
	return os.Open(filepath.Join(s.dataDir, relPath))
}

// Remove deletes a stored photo file. Missing files are not an error.
func (s *Store) Remove(relPath string) error {
	err := os.Remove(filepath.Join(s.dataDir, relPath))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// RemoveTrip deletes every photo stored for a trip.
func (s *Store) RemoveTrip(tripID uuid.UUID) error {
	return os.RemoveAll(filepath.Join(s.dataDir, "photos", tripID.String()))
}

// Meta is EXIF metadata extracted from an uploaded photo.
type Meta struct {
	TakenAt *time.Time
	Lat     *float64
	Lon     *float64
}

// ExtractMeta reads EXIF from a JPEG (the only format goexif supports).
// Missing or unparseable EXIF is not an error — it just yields empty Meta.
func ExtractMeta(r io.Reader) Meta {
	var meta Meta
	x, err := exif.Decode(r)
	if err != nil {
		return meta
	}
	if t, err := x.DateTime(); err == nil {
		meta.TakenAt = &t
	}
	if lat, lon, err := x.LatLong(); err == nil {
		meta.Lat, meta.Lon = &lat, &lon
	}
	return meta
}

// SniffContentType returns the detected image content type of the first
// bytes of the upload, normalizing what browsers report.
func SniffContentType(head []byte) (string, error) {
	ct := detect(head)
	if _, ok := allowedTypes[ct]; !ok {
		return "", fmt.Errorf("%w: detected %s", ErrUnsupportedType, ct)
	}
	return ct, nil
}

func detect(head []byte) string {
	switch {
	case len(head) >= 3 && head[0] == 0xFF && head[1] == 0xD8 && head[2] == 0xFF:
		return "image/jpeg"
	case len(head) >= 8 && string(head[:8]) == "\x89PNG\r\n\x1a\n":
		return "image/png"
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP":
		return "image/webp"
	case len(head) >= 6 && (string(head[:6]) == "GIF87a" || string(head[:6]) == "GIF89a"):
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}
