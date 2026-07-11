package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// tinyPNG is a valid 1x1 transparent PNG (no EXIF — metadata fields stay null).
var tinyPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00,
	0x0D, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x62, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49,
	0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
}

func uploadPhoto(t *testing.T, h http.Handler, cookie *http.Cookie, path string, data []byte, caption string) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("photo", "test.png")
	if err != nil {
		t.Fatal(err)
	}
	fw.Write(data)
	mw.WriteField("caption", caption)
	mw.Close()

	req := httptest.NewRequest("POST", path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var out map[string]any
	if rec.Body.Len() > 0 {
		json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec.Code, out
}

func TestJournal(t *testing.T) {
	h, alice, bob := setup(t)

	_, trip := call(t, h, alice, "POST", "/api/v1/trips", `{"title":"Journal trip"}`)
	tripPath := "/api/v1/trips/" + trip["id"].(string)

	// Create an entry.
	code, entry := call(t, h, alice, "POST", tripPath+"/journal",
		`{"entryDate":"2026-08-02","title":"Day one","body":"Landed and found **ramen**.","lat":35.0,"lon":135.0}`)
	if code != 201 {
		t.Fatalf("create entry: code = %d %v", code, entry)
	}
	entryID := entry["id"].(string)

	t.Run("validation", func(t *testing.T) {
		for name, body := range map[string]string{
			"missing date": `{"title":"no date"}`,
			"half coords":  `{"entryDate":"2026-08-02","lat":35.0}`,
			"bad date":     `{"entryDate":"aug 2"}`,
		} {
			if code, _ := call(t, h, alice, "POST", tripPath+"/journal", body); code != 400 {
				t.Fatalf("%s: code = %d, want 400", name, code)
			}
		}
	})

	t.Run("owner isolation", func(t *testing.T) {
		if code, _ := call(t, h, bob, "GET", tripPath+"/journal", ""); code != 404 {
			t.Fatalf("bob list journal: code = %d, want 404", code)
		}
	})

	// Upload a photo.
	code, photo := uploadPhoto(t, h, alice, tripPath+"/journal/"+entryID+"/photos", tinyPNG, "the arrival")
	if code != 201 {
		t.Fatalf("upload: code = %d %v", code, photo)
	}
	if photo["contentType"] != "image/png" || photo["caption"] != "the arrival" {
		t.Fatalf("photo = %v", photo)
	}
	if photo["takenAt"] != nil {
		t.Fatalf("takenAt should be null for EXIF-less PNG, got %v", photo["takenAt"])
	}
	photoURL := photo["url"].(string)

	t.Run("upload rejects non-images", func(t *testing.T) {
		code, resp := uploadPhoto(t, h, alice, tripPath+"/journal/"+entryID+"/photos", []byte("#!/bin/sh\necho hi"), "")
		if code != 400 {
			t.Fatalf("code = %d %v, want 400", code, resp)
		}
	})

	t.Run("photo serving is owner-scoped", func(t *testing.T) {
		req := httptest.NewRequest("GET", photoURL, nil)
		req.AddCookie(alice)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/png" || !bytes.Equal(rec.Body.Bytes(), tinyPNG) {
			t.Fatalf("owner fetch: code = %d type = %s len = %d", rec.Code, rec.Header().Get("Content-Type"), rec.Body.Len())
		}
		if code, _ := call(t, h, bob, "GET", photoURL, ""); code != 404 {
			t.Fatalf("bob fetch photo: code = %d, want 404", code)
		}
	})

	t.Run("listing embeds photos", func(t *testing.T) {
		_, listing := call(t, h, alice, "GET", tripPath+"/journal", "")
		entries := listing["entries"].([]any)
		if len(entries) != 1 {
			t.Fatalf("entries = %d, want 1", len(entries))
		}
		photos := entries[0].(map[string]any)["photos"].([]any)
		if len(photos) != 1 {
			t.Fatalf("photos = %d, want 1", len(photos))
		}
	})

	t.Run("patch merges", func(t *testing.T) {
		code, updated := call(t, h, alice, "PATCH", tripPath+"/journal/"+entryID, `{"body":"Rewritten.","clearLatLon":true}`)
		if code != 200 {
			t.Fatalf("patch: code = %d %v", code, updated)
		}
		if updated["title"] != "Day one" || updated["body"] != "Rewritten." || updated["lat"] != nil {
			t.Fatalf("patch result: %v", updated)
		}
	})

	t.Run("delete entry removes photos and files", func(t *testing.T) {
		if code, _ := call(t, h, alice, "DELETE", tripPath+"/journal/"+entryID, ""); code != 204 {
			t.Fatalf("delete entry: code = %d", code)
		}
		req := httptest.NewRequest("GET", photoURL, nil)
		req.AddCookie(alice)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 404 {
			t.Fatalf("photo after entry delete: code = %d, want 404", rec.Code)
		}
	})
}

func TestPhotoDelete(t *testing.T) {
	h, alice, _ := setup(t)

	_, trip := call(t, h, alice, "POST", "/api/v1/trips", `{"title":"Photo delete trip"}`)
	tripPath := "/api/v1/trips/" + trip["id"].(string)
	_, entry := call(t, h, alice, "POST", tripPath+"/journal", `{"entryDate":"2026-08-03"}`)
	entryID := entry["id"].(string)

	_, photo := uploadPhoto(t, h, alice, tripPath+"/journal/"+entryID+"/photos", tinyPNG, "")
	photoID := photo["id"].(string)

	if code, _ := call(t, h, alice, "DELETE", fmt.Sprintf("%s/photos/%s", tripPath, photoID), ""); code != 204 {
		t.Fatalf("delete photo: code = %d", code)
	}
	if code, _ := call(t, h, alice, "GET", "/api/v1/photos/"+photoID, ""); code != 404 {
		t.Fatalf("photo after delete: code = %d, want 404", code)
	}
}
