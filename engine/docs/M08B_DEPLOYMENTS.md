# M08B — release-specific Docker deployment and safe Caddy switching

Status: **Implemented in Go; real VPS acceptance test pending.**

## Deployment contract
1. An authenticated request enqueues a background job with an immutable requested image.
2. The engine pulls the image; Docker creates a **new release-specific container**, labelled with project ID and release ID. The container's ephemeral published host port is bound to **127.0.0.1**, never a public interface.
3. The engine probes the new container's mapped host port. If unhealthy, it deletes only that candidate; the previous release remains active.
4. The engine asks Caddy's admin API to load an updated configuration pointing the domain at the **new mapped host port**. A rejected update fails the job; the previous release and image remain active.
5. Once Caddy accepts the configuration, the project pointer, image, release and active upstream are persisted together. If persistence fails, the engine attempts to restore Caddy's previous configuration. If even that fails, it retains the candidate for manual recovery instead of deleting potentially live traffic.
6. Only after success are old candidate containers eligible for cleanup. Up to the latest two healthy release containers are retained; historical image references remain in the store.
7. Rollback redeploys the selected previous healthy image as a fresh release through the same health-check and traffic-switch sequence. It is **not** a mutable rollback tag.

The worker serializes jobs because Caddy configurations are applied as a full document. Queued jobs are resumed after a crash; interrupted running jobs are marked failed for explicit retry. Remote Docker API calls use contexts, and container log frames are bounded.

## Safety boundary
- The default is **mock mode**. Live operations require explicit `--live` and a reachable Docker socket, a **dedicated** Caddy instance with admin API, and a loopback-bound Drevanta API.
- Do **not** use the server's existing shared Caddy instance: Drevanta applies its own complete Caddy configuration.
- Live project deletion and domain mutation endpoints currently reject requests until transactional route removal is implemented.
- The API reports a successful Caddy configuration load, **not** independently verified public DNS, ACME certificate issuance, or end-to-end HTTPS. These must be verified on a disposable VPS before production use.
- Persistent application volumes, secrets, image signing, backup/restore and production hardening are outside this milestone. Do not deploy stateful apps with this version.
- The administrator endpoint must remain private behind authenticated same-origin access.

## Local validation
```sh
cd engine
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/drevanta-engine
go run ./cmd/drevanta-engine --mock --data ./data
```

## Disposable VPS acceptance criteria (NOT YET VERIFIED)
Verify readiness and DNS; deploy `nginx:alpine` over HTTPS; stream logs; redeploy `nginx:1.25`; confirm traffic served by the new container; roll back; inject an unhealthy image and a Caddy failure; confirm the old site remains available throughout. Measure outage, check exact labels/port bindings and inspect Docker/Caddy logs. Do not enable `--live` on an existing production VPS until these checks pass.
