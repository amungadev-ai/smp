# M08C — disposable VPS acceptance

This milestone adds a real HTTP smoke test for the Go engine and explicit protection against accidental shared Caddy takeover.

## Test modes

**Mock / CI:** From `engine/` run:

```bash
go build -o drevanta-engine ./cmd/drevanta-engine
./scripts/mock-ci.sh
```

The script boots the actual Go HTTP service on loopback, logs in, deploys a first release, switches to a second release, rolls back and checks authenticated SSE logs. It uses an ephemeral data directory and makes no Docker/Caddy changes.

**Live (disposable VPS only):** You must own a fresh, disposable VPS with Docker installed, a dedicated Caddy instance (not the distro's shared/default Caddy) and a DNS name pointing at the VPS. Do **not** test on production or a machine hosting other sites.

A dedicated Caddy instance must expose `http://127.0.0.1:2021` as its administration endpoint, and exclusively own HTTP/HTTPS ports 80 and 443. The engine now refuses the default shared admin endpoint at port 2019. Ensure the dedicated Caddy service is actually yours and is not carrying other traffic, because engine `POST /load` replaces its *entire* configuration.

Start the Go engine on the disposable host using explicit confirmation:

```bash
DRVT_ADMIN_PASSWORD='replace-with-a-strong-disposable-password' \
./drevanta-engine --live --ack-disposable-vps \
  --caddy-admin http://127.0.0.1:2021 \
  --addr 127.0.0.1:8080 --data ./lab-state --web ./web
```

From a separate local shell on that same VPS, with the identical admin password in your environment and a **fresh unused** test hostname:

```bash
export DRVT_ADMIN_PASSWORD='replace-with-the-same-password'
export DRVT_SMOKE_MODE=live
export DRVT_SMOKE_CONFIRM=I_HAVE_A_DISPOSABLE_VPS
export DRVT_TEST_DOMAIN=app.your-test-domain.example
./scripts/smoke.sh
```

Dependencies: `curl` and `jq`. The script refuses non-loopback API addresses, non-live engine mode, unconfirmed live tests and hosts with existing engine projects. It creates one test project and leaves it for inspection. **Never use a production domain.**

Checks: API health/login/readiness, first container, second release and active image, real public HTTPS with certificate validation, failed image pull leaving the previous release active, successful rollback, release history and authorized logs. Live HTTP(S) and Docker/Caddy operations are *not* run in CI.

## Fault-injection coverage

Unit and integration tests simulate Caddy returning an error after a candidate was staged. The engine attempts to restore the previous route before cleaning up. If both route update and restoration fail, it keeps the candidate container and marks the project failed for manual intervention, rather than misreporting availability.

## Still required before production

- A genuine disposable VPS run with captured output, DNS resolution, valid ACME certificate and observed application traffic
- Test real Docker failures, restart recovery, Caddy downtime, and state-volume backup/restore
- Validate stable-domain behavior and zero-downtime external probing under load
- Replace JSON state and single-admin lab authentication with hardened production controls
- Finish a dedicated Caddy installer, service user, runtime resource limits and observability

Passing mock CI **does not** certify production readiness.
