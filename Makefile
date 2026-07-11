.PHONY: db run web seed generate test test-db build docker clean

# Start postgres only (for local development)
db:
	docker compose up -d postgres

# Regenerate the sqlc store code from internal/store/queries/*.sql
generate:
	go tool sqlc generate

# Run the Go server against the compose postgres
run:
	WAYPOINT_DATABASE_URL=$${WAYPOINT_DATABASE_URL:-postgres://waypoint:waypoint@localhost:5432/waypoint?sslmode=disable} \
		go run ./cmd/server

# Create/reset the local dev user (dev@waypoint.local / waypoint-dev).
# Sign in with it by running the server with WAYPOINT_LOCAL_AUTH=true.
seed:
	WAYPOINT_DATABASE_URL=$${WAYPOINT_DATABASE_URL:-postgres://waypoint:waypoint@localhost:5432/waypoint?sslmode=disable} \
		go run ./cmd/server seed

# Vite dev server on :5173, proxying /api and /auth to :8080
web:
	cd web && npm run dev

test:
	go vet ./...
	go test ./...

# Like test, but includes postgres-backed store tests (needs `make db` running;
# creates the waypoint_test database automatically)
test-db:
	go vet ./...
	WAYPOINT_TEST_DATABASE_URL=$${WAYPOINT_TEST_DATABASE_URL:-postgres://waypoint:waypoint@localhost:5432/waypoint_test?sslmode=disable} \
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
