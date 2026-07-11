# Mastodon

Single-user Mastodon instance exposed publicly via Cloudflare Tunnel for federation, with authentication restricted to users on the VPN through Authentik SSO.

## Architecture

```
Internet ──► Cloudflare Tunnel ──┐
                                 ├──► nginx (:3000) ──┬──► Puma (:3001)
VPN/LAN  ──► Gateway HTTPRoute ──┘                    └──► mastodon-streaming (:4000)
                                                            (WebSocket /api/v1/streaming)
                                 ┌──────────────────────────────┐
                                 ▼                              ▼
                          mastodon-sidekiq                 database-rw
                           (background jobs)               (CNPG PostgreSQL)
                                 │                              │
                                 └────────► redis (Valkey :6379) ◄┘
```

The nginx sidecar in the web pod routes `/api/v1/streaming` (WebSocket) to the streaming service and all other traffic to Puma. This is necessary because the Cloudflare Tunnel delivers all traffic to a single origin (`localhost:3000`), but Mastodon requires WebSocket connections to reach the separate Node.js streaming server.

### Networking & Access

- **Public access**: The instance is reachable from the internet via a Cloudflare Tunnel (`cloudflared` sidecar in the web deployment). Federation, public timelines, and profiles work without VPN.
- **LAN/VPN access**: Also reachable via the internal gateway HTTPRoute at `mastodon.klimlive.de` (resolves to the gateway IP via local DNS wildcard).
- **Login/Registration**: Disabled locally (`OMNIAUTH_ONLY=true`). All authentication goes through Authentik at `auth.klimlive.de`, which is only reachable from the VPN. This means only VPN users can sign in or create accounts, while the instance remains federated and publicly readable.

### Components

| Deployment | Image | Purpose |
|---|---|---|
| `mastodon-web` | `ghcr.io/mastodon/mastodon` | Rails app (Puma :3001) serving API + web UI, with `nginx` reverse proxy (:3000) and `cloudflared` sidecar |
| `mastodon-streaming` | `ghcr.io/mastodon/mastodon-streaming` | Node.js real-time streaming API |
| `mastodon-sidekiq` | `ghcr.io/mastodon/mastodon` | Background job processor (all queues) |
| `redis` | `valkey/valkey:9` | Cache and job queue backend |

### Database

PostgreSQL via CloudNativePG operator — 1 instance, 10Gi on `openebs-crucial`. Service endpoint: `database-rw`. The CNPG instance manager requires egress to the Kubernetes API server; this is allowed via a port-based egress rule in the `database-egress` NetworkPolicy.

### Shared Volume

The `mastodon-data` PVC (`openebs-crucial`, RWO) is mounted by both `mastodon-web` and `mastodon-sidekiq`. Because it is RWO, both deployments use `strategy: Recreate` and sidekiq has a `podAffinity` rule to co-locate with the web pod on the same node.

### Observability

- **Tracing (web + sidekiq)**: Native OTel export (Mastodon >=4.3.0), configured via `OTEL_EXPORTER_OTLP_ENDPOINT` in the ConfigMap. Service names: `mastodon/web`, `mastodon/sidekiq`.
- **Tracing (streaming)**: OTel Operator Node.js auto-instrumentation via `inject-nodejs` annotation.
- **Postgres metrics**: Dedicated `OpenTelemetryCollector` deployment scraping `database-rw:5432`.
- **Redis metrics**: `OpenTelemetryCollector` sidecar injected into the Redis pod.
- **Tunnel metrics**: Prometheus `ServiceMonitor` scraping cloudflared at `:2000/metrics`.

All telemetry flows to the central backend collector in the `observability` namespace and into SigNoz.

## Setup

### 1. Generate secrets

```bash
# SECRET_KEY_BASE and OTP_SECRET (run twice, one for each)
docker run --rm ghcr.io/mastodon/mastodon:v4.3.6 bundle exec rails secret

# VAPID keys
docker run --rm ghcr.io/mastodon/mastodon:v4.3.6 bundle exec rails mastodon:webpush:generate_vapid_key

# ActiveRecord encryption keys
docker run --rm ghcr.io/mastodon/mastodon:v4.3.6 bundle exec rails db:encryption:init
```

### 2. Configure Authentik

1. Create a new **OAuth2/OpenID Connect** provider with slug `mastodon`.
2. Set redirect URI to `https://mastodon.klimlive.de/auth/auth/openid_connect/callback`.
3. Scopes: `openid`, `profile`, `email`.
4. Create an application linked to the provider.
5. Copy the client ID and secret into `secret.yaml`.

### 3. Create Cloudflare Tunnel

1. In the Cloudflare dashboard, create a tunnel for `mastodon.klimlive.de`.
2. Route the hostname to `http://localhost:3000` (nginx reverse proxy in the same pod).
3. Copy the tunnel token into `secret-tunnel.yaml`.

### 4. Encrypt secrets

```bash
sops --encrypt --age age1nl4pnuny2pjg3ejfk9vrx0y4ssmna36xlw3wqmzv55ku38psdylsp2t2yw \
  --encrypted-regex '^(stringData)$' -i secret.yaml

sops --encrypt --age age1nl4pnuny2pjg3ejfk9vrx0y4ssmna36xlw3wqmzv55ku38psdylsp2t2yw \
  --encrypted-regex '^(stringData)$' -i secret-tunnel.yaml
```

### 5. Deploy

Apply the namespace, secrets, configmap, database, and redis first. Then run the migration job before starting the application deployments:

```bash
kubectl apply -k .
# Wait for the database to be ready
kubectl -n mastodon wait --for=condition=Ready cluster/database --timeout=120s
# The mastodon-db-migrate Job runs automatically — check its status
kubectl -n mastodon logs job/mastodon-db-migrate -f
```

### 6. Create admin account

After the first successful login via Authentik, promote the user to admin:

```bash
kubectl -n mastodon exec -it deploy/mastodon-web -c web -- \
  bundle exec tootctl accounts modify <username> --role Owner
```
