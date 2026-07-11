package photos

import (
	"bytes"
	"math"
	"os"
	"testing"
	"time"
)

func TestExtractMeta(t *testing.T) {
	t.Run("jpeg with exif", func(t *testing.T) {
		// testdata/exif-gps.jpg carries DateTimeOriginal 2026:07:04 14:30:00
		// and GPS 35°30'N 139°45'E.
		f, err := os.Open("testdata/exif-gps.jpg")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		m := ExtractMeta(f)
		if m.TakenAt == nil {
			t.Fatal("TakenAt = nil, want 2026-07-04 14:30")
		}
		want := time.Date(2026, 7, 4, 14, 30, 0, 0, m.TakenAt.Location())
		if !m.TakenAt.Equal(want) {
			t.Fatalf("TakenAt = %v, want %v", m.TakenAt, want)
		}
		if m.Lat == nil || m.Lon == nil {
			t.Fatal("GPS coordinates missing")
		}
		if math.Abs(*m.Lat-35.5) > 1e-6 || math.Abs(*m.Lon-139.75) > 1e-6 {
			t.Fatalf("GPS = %v,%v, want 35.5,139.75", *m.Lat, *m.Lon)
		}
	})

	t.Run("no exif degrades gracefully", func(t *testing.T) {
		m := ExtractMeta(bytes.NewReader([]byte("definitely not a jpeg")))
		if m.TakenAt != nil || m.Lat != nil || m.Lon != nil {
			t.Fatalf("expected empty meta, got %+v", m)
		}
	})
}

func TestSniffContentType(t *testing.T) {
	for name, tc := range map[string]struct {
		head []byte
		want string
		err  bool
	}{
		"jpeg":   {[]byte{0xFF, 0xD8, 0xFF, 0xE0}, "image/jpeg", false},
		"png":    {[]byte("\x89PNG\r\n\x1a\n...."), "image/png", false},
		"gif":    {[]byte("GIF89a...."), "image/gif", false},
		"webp":   {[]byte("RIFF\x00\x00\x00\x00WEBP"), "image/webp", false},
		"script": {[]byte("#!/bin/sh"), "", true},
		"empty":  {nil, "", true},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := SniffContentType(tc.head)
			if tc.err != (err != nil) || got != tc.want {
				t.Fatalf("got %q err=%v, want %q err=%v", got, err, tc.want, tc.err)
			}
		})
	}
}
