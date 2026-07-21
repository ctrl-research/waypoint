# Configuration

Waypoint is configured entirely through `WAYPOINT_*` environment variables.
The compose file reads them from `.env` (see `.env.example` for a commented
template).

## Reference

| Variable | Default | Description |
|---|---|---|
| `WAYPOINT_DATABASE_URL` | — | **Required.** Postgres connection string. |
| `WAYPOINT_ADDR` | `:8080` | Listen address. |
| `WAYPOINT_BASE_URL` | `http://localhost:8080` | Public URL of the instance. OAuth redirect URIs derive from it; session cookies are `Secure` when it is `https`. No trailing slash. |
| `WAYPOINT_GOOGLE_CLIENT_ID` | — | Google OAuth client id. Set together with the secret to enable Sign in with Google. |
| `WAYPOINT_GOOGLE_CLIENT_SECRET` | — | Google OAuth client secret. |
| `WAYPOINT_OIDC_ISSUER_URL` | — | Generic OIDC issuer URL (Authentik, Keycloak, …), used for discovery. Must match the provider's discovery document **exactly, trailing slash included** — copy it from the IdP. Set together with the client id and secret. |
| `WAYPOINT_OIDC_CLIENT_ID` / `WAYPOINT_OIDC_CLIENT_SECRET` | — | Client credentials for the generic OIDC provider. |
| `WAYPOINT_OIDC_NAME` | `SSO` | Label for the generic provider's login button. |
| `WAYPOINT_LOCAL_AUTH` | `false` | Enable email/password accounts (intended for dev/testing; `make seed` creates one). |
| `WAYPOINT_ALLOWED_EMAILS` | — | Comma-separated emails allowed to **sign up** after the first user. Empty means the instance is closed to new accounts. |
| `WAYPOINT_DATA_DIR` | `data` | Directory for journal photo uploads. The container image uses `/data` — mount a volume there. |
| `WAYPOINT_TILE_URL` | OSM tiles | Raster tile URL template with `{z}/{x}/{y}` placeholders. |
| `WAYPOINT_MAP_STYLE_URL` | — | MapLibre vector style JSON URL. Takes precedence over raster tiles; map labels localize to `WAYPOINT_LANGUAGE`. |
| `WAYPOINT_NOMINATIM_URL` | `https://nominatim.openstreetmap.org` | Geocoding endpoint for place / station / airport search. |
| `WAYPOINT_LANGUAGE` | — | BCP 47 language (`en`, `fr`, …) for geocoded names and vector map labels. Unset keeps native names (東京都 instead of Tokyo). |
| `WAYPOINT_MCP` | `false` | Serve the MCP endpoint at `/mcp` for AI clients (see below). |

## Authentication

**Google sign-in.** Create an OAuth client in the
[Google Cloud Console](https://console.cloud.google.com/apis/credentials):

1. Configure the OAuth consent screen (type *External*; *Testing* mode is
   fine for a homelab — add the accounts that will sign in as test users).
2. Create an **OAuth client ID** of type *Web application* with the
   authorized redirect URI `$WAYPOINT_BASE_URL/auth/google/callback`.
3. Set both `WAYPOINT_GOOGLE_CLIENT_ID` and `WAYPOINT_GOOGLE_CLIENT_SECRET`
   and restart. The login page shows the Google button automatically.

Gotchas:

- The redirect URI must match `WAYPOINT_BASE_URL` exactly — scheme, host,
  and port. A mismatch surfaces as Google's `redirect_uri_mismatch` error.
- Google requires HTTPS for redirect URIs everywhere except `localhost`.
- **Sign in yourself first**: the first account on an instance becomes admin
  and bypasses the allowlist.

**Allowlist semantics.** `WAYPOINT_ALLOWED_EMAILS` gates *sign-up*, not
sign-in: after the first user, only listed addresses can create accounts
(case-insensitive, exact addresses — no domain wildcards), while existing
accounts always keep working. An empty allowlist closes the instance to new
accounts entirely.

**Generic OIDC (Authentik, Keycloak, …).** Register Waypoint in your IdP as a
confidential OAuth2/OIDC client with redirect URI
`$WAYPOINT_BASE_URL/auth/oidc/callback` and scopes `openid email profile`,
then set `WAYPOINT_OIDC_ISSUER_URL` (the issuer, e.g. Authentik's
`https://auth.example.com/application/o/waypoint/`), the client id/secret,
and optionally `WAYPOINT_OIDC_NAME` for the button label. Discovery,
PKCE, the allowlist, and email-based account linking all work exactly like
the Google flow; an id token that explicitly reports `email_verified: false`
is rejected.

**Local accounts.** `WAYPOINT_LOCAL_AUTH=true` enables email/password login,
meant for development. `make seed` creates `dev@waypoint.local` /
`waypoint-dev` as an admin.

## Maps and geocoding

Maps default to OpenStreetMap raster tiles. For crisp vector maps with
localized labels set `WAYPOINT_MAP_STYLE_URL` —
[OpenFreeMap](https://openfreemap.org)'s
`https://tiles.openfreemap.org/styles/liberty` is free with no API key.

Place search (areas, venues, train stations, airports) proxies a Nominatim
instance and is rate-limited to 1 req/s to respect the public server's usage
policy. Point `WAYPOINT_NOMINATIM_URL` at a self-hosted Nominatim if your
usage is heavy.

## AI access (MCP)

`WAYPOINT_MCP=true` serves a
[Model Context Protocol](https://modelcontextprotocol.io) endpoint at `/mcp`.
Each user mints a personal bearer token in **Settings → AI access** and
connects any MCP client:

```sh
claude mcp add --transport http waypoint \
  https://your-host/mcp \
  --header "Authorization: Bearer YOUR_TOKEN"
```

Tools run with exactly that user's access (trip roles apply). The endpoint
and its token-management routes are not registered at all when the flag is
off.
