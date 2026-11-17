# Installing the Drevanta Engine on a VPS

Target: a fresh Ubuntu 22.04/24.04 or Debian 12 box with root SSH. The engine
manages this host — no add-server step.

## 1. Prerequisites

```bash
apt-get update
apt-get install -y golang-go docker.io caddy git   # Go 1.22+ required
systemctl enable --now docker caddy
```

(RHEL-family: `dnf install -y go docker-ce caddy` per upstream docs.)

## 2. Build and install

```bash
git clone <your-repo> /opt/drevanta-src && cd /opt/drevanta-src/engine
go build -o /usr/local/bin/drevanta-engine ./cmd/drevanta-engine
mkdir -p /var/lib/drevanta-engine
```

## 3. systemd service

```bash
cp deploy/drevanta-engine.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now drevanta-engine
journalctl -u drevanta-engine -f     # the admin password is printed here
```

The password is generated on first boot and stored in
`/var/lib/drevanta-engine/admin-password` (0600). Override with
`DRVT_ADMIN_PASSWORD` in the unit's Environment line if you prefer.

## 4. Caddy

The engine routes domains through Caddy's admin API on 127.0.0.1:2019. Make
sure Caddy is running with its admin endpoint enabled (the stock Caddyfile
already has `admin 127.0.0.1:2019`). Bootstrap alternative if you prefer
file-based config first:

```bash
cp deploy/Caddyfile.example /etc/caddy/Caddyfile
systemctl reload caddy
```

After the first deploy the engine mirrors its generated configuration to
`/var/lib/drevanta-engine/Caddyfile` for reference.

## 5. Verify

```bash
curl http://127.0.0.1:8080/api/v1/health
# {"ok":true,"version":"0.1.0","mock_mode":false}
```

Open the console on `http://SERVER_IP:8080/` (keep 8080 private; put the
portal + Caddy in front for public access) or connect the portal:

```bash
cd ../portal && npm install && npm run build && PORT=3000 node build
```

## 6. First deployment

1. Sign in, check **readiness** (Docker, Caddy, ports 80/443, state dir).
2. Deploy an approved image, e.g. `nginx:alpine` on port 80 with domain
   `app.example.com` (A record → this server first).
3. Watch the job turn succeeded, the release turn healthy, then open
   `https://app.example.com` — certificate is automatic.
4. Stream logs, restart, and roll back from the project page.

## Troubleshooting

| Symptom | Fix |
| --- | --- |
| engine in mock mode | Docker socket missing — check `docker ps` as root and the unit runs as root |
| caddy mode "mock" | admin API not reachable: `curl 127.0.0.1:2019/config/` |
| health check fails | wrong container port in the deploy form; app must answer `/` |
| certificate not issued | domain DNS must resolve to this server before the route is created |
| state corrupted | `data/state.json.tmp` is the good copy — replace state.json with it |
