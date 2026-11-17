# Drevanta — Server Management Platform

A self-hosted GUI-first hosting console. **SvelteKit + TypeScript frontend** and a **Go deployment engine**, following the v0.16 Drevanta design.

## Current milestone: M08A (mock API integration)

The source for the uploaded Go M01 engine is committed in `engine/`. Its new mock-only safety gate prevents it from altering Docker, Caddy, DNS or VPS resources while release-specific container switching and HTTPS failure handling are still being hardened.

The Svelte portal now has an **Engine** entry in the infrastructure sidebar. This is the first screen to connect to the Go REST API. It displays detected host facts, authenticated job state, project and release history, and uses authenticated streaming logs. The remaining native screens still use their browser-local demo stores and **must not be mistaken for live infrastructure**.

## Run locally (two terminals)

Requirements: Go 1.23+, Node.js 22+.

Terminal A — mock Go engine:

```sh
cd engine
go test ./...
go run ./cmd/drevanta-engine --mock --addr 127.0.0.1:8080 --data ./data
```

The first run creates `engine/data/admin-password` with owner-only permissions. Use that password in the portal. It is **reused** on later restarts. Do not commit it or copy it into source files.

Terminal B — portal:

```sh
cd frontend
npm install
npm test
npm run check
npm run dev
```

Open `http://localhost:5173/engine`. Vite proxies `/api/v1` to `127.0.0.1:8080`. Sign in and try **Deploy image → Job history → Logs → Rollback**, all in mock mode. The original v0.16 design reference remains at `/reference/v016.html`.

## Tests

- [Frontend CI](.github/workflows/frontend.yml): unit tests, Svelte type-check, production build.
- [Go engine CI](.github/workflows/engine.yml): Go tests, `go vet`, build.
- [Migration history](frontend/MIGRATION.md), [engine integration and safety notes](engine/docs/M08A_INTEGRATION.md).

## What is NOT live yet

Real Docker deployments and blue/green rollback, HTTPS/Caddy routing, server installation, Git builds, databases, file uploads, mail delivery, tasks and backups. **The imported engine explicitly rejects `--live` at this milestone.** Do not deploy it on a public VPS as a production management panel.

Never commit secrets or real credentials. Prefer a disposable test server for the forthcoming M08B/M08C steps.
