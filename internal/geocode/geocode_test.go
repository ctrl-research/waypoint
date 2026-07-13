package geocode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearch(t *testing.T) {
	var gotPath, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		gotUA = r.Header.Get("User-Agent")
		w.Write([]byte(`[
			{"display_name":"Kyoto, Japan","lat":"35.0116","lon":"135.7681"},
			{"display_name":"Broken","lat":"not-a-number","lon":"0"}
		]`))
	}))
	defer srv.Close()

	results, err := New(srv.URL, "").Search(context.Background(), "kyoto japan", 5, false)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1 (unparseable rows skipped)", len(results))
	}
	if results[0].Name != "Kyoto, Japan" || results[0].Lat != 35.0116 || results[0].Lon != 135.7681 {
		t.Fatalf("result = %+v", results[0])
	}
	if gotPath != "/search?format=jsonv2&limit=5&q=kyoto+japan" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotUA == "" || gotUA == "Go-http-client/1.1" {
		t.Fatalf("User-Agent = %q; OSM policy requires an identifying UA", gotUA)
	}
}

func TestSearchLanguage(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	if _, err := New(srv.URL, "fr").Search(context.Background(), "londres", 5, false); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !strings.Contains(gotQuery, "accept-language=fr") {
		t.Fatalf("query %q missing accept-language", gotQuery)
	}

	if _, err := New(srv.URL, "").Search(context.Background(), "londres", 5, false); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if strings.Contains(gotQuery, "accept-language") {
		t.Fatalf("query %q should not force a language when unset", gotQuery)
	}
}

func TestSearchUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	if _, err := New(srv.URL, "en").Search(context.Background(), "kyoto", 5, true); err == nil {
		t.Fatal("expected error on upstream 503")
	}
}
