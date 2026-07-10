.PHONY: db run web test build docker clean

# Start postgres only (for local development)
db:
	docker compose up -d postgres

# Run the Go server against the compose postgres
run:
	WAYPOINT_DATABASE_URL=$${WAYPOINT_DATABASE_URL:-postgres://waypoint:waypoint@localhost:5432/waypoint?sslmode=disable} \
		go run ./cmd/server

# Vite dev server on :5173, proxying /api and /auth to :8080
web:
	cd web && npm run dev

test:
	go vet ./...
	go test ./...

# Build the web UI and the server binary with the UI embedded
build:
	cd web && npm run build
	rm -rf internal/webui/dist
	cp -R web/dist internal/webui/dist
	go build -tags embedwebui -o bin/waypoint ./cmd/server

docker:
	docker build -t waypoint .

clean:
	rm -rf bin internal/webui/dist web/dist
