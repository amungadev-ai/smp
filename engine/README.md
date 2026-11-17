# Drevanta Engine — M01

The core server engine: a Go backend that discovers the VPS it runs on, checks
readiness, deploys approved Docker containers as background jobs, health-checks
them, routes domains through Caddy (automatic HTTPS), streams container logs
over Server-Sent Events and rolls back to the previous good release.

**M08C status:** Mock mode is the default. Live mode now requires `--live --ack-disposable-vps --caddy-admin http://127.0.0.1:2021` and a dedicated Caddy instance. Follow [M08C acceptance](docs/M08C-ACCEPTANCE.md); passing CI is not real VPS validation.

**M01 scope:** JSON file persistence (PostgreSQL arrives in milestone 05), a
small vanilla-JS console for lab testing, and mock modes for Docker/Caddy so
the entire workflow is testable without a VPS.

## Layout

```
cmd/drevanta-engine/     entrypoint (flags, wiring, graceful shutdown)
internal/model/          domain types (mirrored by the portal's TypeScript)
internal/store/          JSON state with atomic writes
internal/auth/           password login, HMAC tokens, rate limiting
internal/serverinfo/     local VPS discovery + readiness checklist
internal/runtime/        Docker Engine API client (unix socket) + mock mode
internal/proxy/          Caddy admin API client + Caddyfile generator
internal/healthcheck/    HTTP port probing
internal/jobs/           deploy / rollback / restart workers
internal/api/            REST + SSE endpoints
web/                     mini console (vanilla JS)
deploy/                  systemd unit + Caddyfile example
docs/INSTALL.md          VPS installation guide
```

## Quick start (lab, no Docker needed)

```bash
go build ./...
go test ./...        # mocked deploy → health → routing → rollback → SSE
go run ./cmd/drevanta-engine --addr 127.0.0.1:8080 --data ./data
# console: http://127.0.0.1:8080/  (password printed in the log / data/admin-password)
```

Mock mode is automatic when `/var/run/docker.sock` is absent (or forced with
`--mock`). Every API response reports the mode (`mock_mode`), and the SSE log
stream delivers a synthetic container trace so the UI is fully testable.

## API (v1)

| Method | Path | Purpose |
| --- | --- | --- |
| POST | /api/v1/auth/login | password → session token (rate limited) |
| GET | /api/v1/health | engine status + mode (no auth) |
| GET | /api/v1/server | discovered host facts |
| GET | /api/v1/server/readiness | M01 checklist |
| POST | /api/v1/projects/deploy | create project + queue deployment job |
| POST | /api/v1/projects/{id}/deploy | roll project forward to a new image |
| GET | /api/v1/projects[/{id}] | list / read projects |
| DELETE | /api/v1/projects/{id} | stop container, remove project, re-route |
| POST | /api/v1/projects/{id}/restart | restart in place |
| POST | /api/v1/projects/{id}/rollback | restore previous healthy release |
| GET | /api/v1/releases, /api/v1/jobs[/{id}] | history |
| GET/POST/DELETE | /api/v1/domains[/{id}] | Caddy routing |
| GET | /api/v1/projects/{id}/logs/stream | SSE container logs |

All routes except `health` and `login` require `Authorization: Bearer <token>`.
Log streams use authenticated fetch streaming, not tokens in URLs.

## Live mode (VPS with Docker + Caddy)

- Docker is driven through the Engine API socket: pull (streaming), create with
  ephemeral host-port mapping, start, restart, stop/rm, log follow with frame
  demultiplexing.
- Caddy is driven through the admin API (`POST /load` with a generated
  Caddyfile, mirrored to `<data>/Caddyfile`). Site blocks per domain give
  automatic HTTPS; Caddy obtains and renews certificates.
- Health checks poll the mapped host port until the app responds (20 tries,
  500 ms apart); result and latency are stored on the release.

## Tests

```bash
go test ./...
```

Covers: state persistence roundtrip, duplicate/atomic behavior, login and
rate limiting, token forgery, the full mocked deployment workflow (project
state, release history, health fields, routing), input validation, removal,
SSE frame delivery and the health endpoint shape.

## What M01 deliberately does not do

No PostgreSQL, no multi-host management, no build-from-source pipeline
(milestone 03), no storage quotas — those follow the roadmap in `docs/ROADMAP.md`
at the repository root.

## M08C validation

Run `go test ./...`, `go test -race ./...`, `go vet ./...` and `go build -o drevanta-engine ./cmd/drevanta-engine && ./scripts/mock-ci.sh`. The smoke test exercises the actual HTTP server. Live HTTPS testing requires a disposable VPS and explicitly opt-in configuration; see `docs/M08C-ACCEPTANCE.md`.
