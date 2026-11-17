# Drevanta — Server Management Platform

GUI-first self-hosted hosting console: SvelteKit + TypeScript frontend, Go server engine.

## Current milestone: M08B (blue/green groundwork)

The Svelte dashboard and original v0.16 styling are retained. The dedicated **Engine** page communicates with the Go API at `/api/v1`. Other UI modules are still browser-local samples. Go M08B now creates release-specific containers, checks health, switches Caddy only when the candidate is healthy, persists active image and mapped host port atomically, and uses the same release flow for rollback.

**The default engine mode is mock.** Real Docker/Caddy mode requires an explicit `--live` flag and a **disposable test VPS** with a dedicated Caddy instance. Public HTTPS, TLS issuance and production safety have **not** been tested end to end. Do not use on existing production workloads.

## Developer workflow

Run Go tests and the mock engine in terminal A:
```sh
cd engine
go test ./...
go vet ./...
go run ./cmd/drevanta-engine --mock --data ./data
```
The first run creates `engine/data/admin-password` (0600). Use that password in the portal; keep it secret.

Run the frontend in terminal B:
```sh
cd frontend
npm install
npm test
npm run check
npm run dev
```
Open `http://localhost:5173/engine`. The Vite proxy connects to the Go API on 127.0.0.1:8080. Test image deployment, job state, release history, logs and rollback **in mock mode**.

See [M08B release design](engine/docs/M08B_DEPLOYMENTS.md) and [Go tests](.github/workflows/engine.yml). The original static design reference remains at `/reference/v016.html`.
