# Drevanta — Server Management Platform

Drevanta is a self-hosted, GUI-first VPS management platform built with SvelteKit/TypeScript and a Go engine. The original v0.16 visual design is preserved.

## Current milestone: M09 — mock-only VPS installer

The native dashboard implements Projects, Websites, Domains, Databases, Storage, File Manager, deployment source inspection, Email, standalone Webmail and infrastructure previews. Most resource modules still use browser-local demo data. The **Engine** page communicates with the authenticated Go API.

The M09 installer configures **two sandboxed, local-only systemd services**:

- Go engine on `127.0.0.1:8080` forced into **mock mode**
- SvelteKit adapter-node portal on `127.0.0.1:3000`, proxying `/api/v1/*` to Go

It uses a separate unprivileged build account, keeps code root-owned after building, preserves a 0600 administrator password in persistent engine state, and supports versioned upgrades and rollback. An SSH tunnel provides access. **No live Docker, Caddy, HTTPS or production application deployment is enabled.**

See [M09 installation](docs/M09-INSTALL.md), [M08C acceptance](engine/docs/M08C-ACCEPTANCE.md) and the [installer CI workflow](.github/workflows/installer.yml).

## Developer workflow

```bash
cd engine
go test ./...
go test -race ./...
go vet ./...
go run ./cmd/drevanta-engine --mock --data ./data
```

In another terminal:

```bash
cd frontend
npm install
npm test
npm run check
npm run dev
```

Open `http://127.0.0.1:5173/engine`. The development Vite proxy and production adapter-node proxy both forward the Go API over localhost.

## Readiness

GitHub Actions verifies the frontend, Go engine, installer CLI and a full localhost production-stack HTTP smoke test. **Passing mock CI is not production readiness.** Live rollout still requires a disposable VPS acceptance run, HTTPS, recovery and authorization hardening. Do not install over existing production workloads.
