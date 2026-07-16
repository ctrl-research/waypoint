# Deployment

Waypoint is a single container (Go binary with the web UI embedded) plus
PostgreSQL. Multi-arch images (linux/amd64, linux/arm64) are published on
every release:

```
ghcr.io/ctrl-research/waypoint:X.Y.Z   # or :latest
```

State lives in two places — the database, and `/data` for journal photo
uploads. Migrations run automatically at startup, so upgrades are just a new
image tag. See [CONFIGURATION.md](CONFIGURATION.md) for every environment
variable.

## Docker Compose

The repo's `docker-compose.yml` builds from source. For a homelab deploy off
the published image:

```yaml
services:
  waypoint:
    image: ghcr.io/ctrl-research/waypoint:latest
    ports:
      - "8080:8080"
    environment:
      WAYPOINT_DATABASE_URL: postgres://waypoint:${POSTGRES_PASSWORD}@postgres:5432/waypoint?sslmode=disable
      WAYPOINT_DATA_DIR: /data
      WAYPOINT_BASE_URL: https://waypoint.example.com
      WAYPOINT_GOOGLE_CLIENT_ID: ${WAYPOINT_GOOGLE_CLIENT_ID}
      WAYPOINT_GOOGLE_CLIENT_SECRET: ${WAYPOINT_GOOGLE_CLIENT_SECRET}
      WAYPOINT_ALLOWED_EMAILS: you@example.com
      WAYPOINT_MAP_STYLE_URL: https://tiles.openfreemap.org/styles/liberty
      WAYPOINT_LANGUAGE: en
    volumes:
      - waypoint-data:/data
    depends_on:
      postgres:
        condition: service_healthy
    restart: unless-stopped

  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: waypoint
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: waypoint
    volumes:
      - postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U waypoint"]
      interval: 5s
      timeout: 3s
      retries: 10
    restart: unless-stopped

volumes:
  waypoint-data:
  postgres-data:
```

Put a TLS-terminating reverse proxy (Caddy, Traefik, nginx) in front and set
`WAYPOINT_BASE_URL` to the public URL — Google OAuth requires HTTPS and the
redirect URI derives from it.

## Kubernetes

A minimal single-replica setup. Adjust storage classes, hosts, and resource
requests for your cluster; if you run an operator like CloudNativePG, use it
for Postgres instead of the StatefulSet below and just point
`WAYPOINT_DATABASE_URL` at it.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: waypoint-secrets
stringData:
  postgres-password: change-me
  google-client-id: ""       # optional; see docs/CONFIGURATION.md
  google-client-secret: ""
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: waypoint-data
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 5Gi
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: waypoint-postgres
spec:
  serviceName: waypoint-postgres
  replicas: 1
  selector:
    matchLabels: { app: waypoint-postgres }
  template:
    metadata:
      labels: { app: waypoint-postgres }
    spec:
      containers:
        - name: postgres
          image: postgres:16-alpine
          env:
            - name: POSTGRES_USER
              value: waypoint
            - name: POSTGRES_DB
              value: waypoint
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef: { name: waypoint-secrets, key: postgres-password }
          ports:
            - containerPort: 5432
          volumeMounts:
            - name: pgdata
              mountPath: /var/lib/postgresql/data
          readinessProbe:
            exec:
              command: ["pg_isready", "-U", "waypoint"]
            periodSeconds: 5
  volumeClaimTemplates:
    - metadata:
        name: pgdata
      spec:
        accessModes: [ReadWriteOnce]
        resources:
          requests:
            storage: 10Gi
---
apiVersion: v1
kind: Service
metadata:
  name: waypoint-postgres
spec:
  selector: { app: waypoint-postgres }
  ports:
    - port: 5432
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: waypoint
spec:
  replicas: 1
  strategy:
    type: Recreate        # /data is ReadWriteOnce
  selector:
    matchLabels: { app: waypoint }
  template:
    metadata:
      labels: { app: waypoint }
    spec:
      containers:
        - name: waypoint
          image: ghcr.io/ctrl-research/waypoint:latest   # pin X.Y.Z in production
          ports:
            - containerPort: 8080
          env:
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef: { name: waypoint-secrets, key: postgres-password }
            - name: WAYPOINT_DATABASE_URL
              value: postgres://waypoint:$(POSTGRES_PASSWORD)@waypoint-postgres:5432/waypoint?sslmode=disable
            - name: WAYPOINT_DATA_DIR
              value: /data
            - name: WAYPOINT_BASE_URL
              value: https://waypoint.example.com
            - name: WAYPOINT_MAP_STYLE_URL
              value: https://tiles.openfreemap.org/styles/liberty
            - name: WAYPOINT_GOOGLE_CLIENT_ID
              valueFrom:
                secretKeyRef: { name: waypoint-secrets, key: google-client-id, optional: true }
            - name: WAYPOINT_GOOGLE_CLIENT_SECRET
              valueFrom:
                secretKeyRef: { name: waypoint-secrets, key: google-client-secret, optional: true }
          volumeMounts:
            - name: data
              mountPath: /data
          readinessProbe:
            httpGet: { path: /healthz, port: 8080 }
            periodSeconds: 10
          livenessProbe:
            httpGet: { path: /healthz, port: 8080 }
            initialDelaySeconds: 10
            periodSeconds: 30
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: waypoint-data
---
apiVersion: v1
kind: Service
metadata:
  name: waypoint
spec:
  selector: { app: waypoint }
  ports:
    - port: 80
      targetPort: 8080
---
# Expose with your ingress of choice, e.g.:
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: waypoint
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt   # if you use cert-manager
spec:
  rules:
    - host: waypoint.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: waypoint
                port: { number: 80 }
  tls:
    - hosts: [waypoint.example.com]
      secretName: waypoint-tls
```

Notes:

- `/healthz` checks the database connection and is the right probe target.
- `strategy: Recreate` avoids two pods fighting over the ReadWriteOnce
  `/data` volume during rollouts. If you need rolling updates, put `/data`
  on ReadWriteMany storage.
- Pin the image to a version tag in production; `latest` moves on every
  merge. Releases are listed on
  [GitHub](https://github.com/ctrl-research/waypoint/releases).

## Backups

`pg_dump` of the database plus a copy of the `/data` volume is a complete
backup. Restores into a newer image version are fine — migrations run
forward automatically at startup.
