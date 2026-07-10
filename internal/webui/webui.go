//go:build !embedwebui

// Package webui serves the built frontend. Without the embedwebui build tag
// (plain `go build`, used in development and CI) it serves a pointer to the
// Vite dev server instead; `make build` and the Dockerfile build with
// -tags embedwebui after copying web/dist into this package.
package webui

import "net/http"

func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "web UI not embedded in this build — run the Vite dev server (make web) or build with `make build`", http.StatusNotFound)
	})
}
