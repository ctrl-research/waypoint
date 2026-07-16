# Stage 1: build the web UI
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: build the Go server with the UI embedded
FROM golang:1.26-alpine AS server
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist internal/webui/dist
RUN CGO_ENABLED=0 go build -tags embedwebui -ldflags "-X main.version=${VERSION}" -o /waypoint ./cmd/server

# Stage 3: runtime
FROM alpine:3.21
RUN adduser -D -H waypoint && mkdir /data && chown waypoint /data
COPY --from=server /waypoint /usr/local/bin/waypoint
USER waypoint
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["waypoint"]
