# M08A integration and safety gate

This engine is imported from the user-supplied `drevanta-engine-m01(1).zip` (SHA-256: `0a3a0a10abfdb06a07fff4c7d0296669eef3199b569a97b62e060a3716d7fe7e`), with these changes:

- Persistent administrator password across restarts, generated once with 0600 permissions. Explicit flags/environment values override stored credentials.
- Authentication for streaming logs requires a Bearer header. URL query tokens are no longer accepted.
- Fail-closed M08A mode: **Docker and Caddy operations run only in mock mode**. Explicit `--live` is rejected until a release-safe blue/green rollout, Caddy routing, credential hardening and VPS integration tests are finished.
- Go unit tests and static analysis are run by CI.

The Go engine listens on 127.0.0.1:8080 by default. The Svelte portal should connect through a same-origin reverse proxy on `/api/v1`. In development, Vite proxies that path to localhost port 8080.

## Lab smoke test

```sh
cd engine
go test ./...
go vet ./...
go run ./cmd/drevanta-engine --mock --addr 127.0.0.1:8080 --data ./data
```

On the first run, read the generated administrator password from `engine/data/admin-password`. Do **not** commit any state or credentials. The `/engine` Svelte screen performs an authenticated login and can test deploy / logs / rollback in mock mode.

**No real applications, domains, certificates or VPS resources are managed in this milestone.** Live operation remains blocked until M08C.
