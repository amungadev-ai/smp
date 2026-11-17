# M09 — Drevanta VPS installer (localhost-only preview)

M09 installs the **SvelteKit adapter-node frontend and Go engine** as two versioned systemd services. **It does not enable real Docker, Caddy, HTTPS or production hosting.**

## Prerequisites and scope

Use a dedicated test VPS running Ubuntu 22.04/24.04 or Debian 12 on x86_64/arm64 with systemd. Install Go 1.22+, Node.js 22+, npm, curl and normal Linux utilities (tar, runuser, flock). The installer does not run apt or change the firewall. It needs network access to install Node dependencies.

Do not use an existing production VPS. Live engine and public portal exposure are intentionally disabled.

## Inspect and install

From an audited checkout of the Drevanta repository:

```bash
./deploy/install.sh --plan
sudo ./deploy/install.sh --install --accept-mock-only
./deploy/install.sh --verify
```

The script validates dependencies, creates two dedicated accounts, builds and tests Go/SvelteKit as `drevanta-build` (which cannot read engine state), stages a root-owned release, installs sandboxed systemd units, enables them and verifies local-only health and authentication. The runtime account is `drevanta`, **not in the Docker group**. Existing, non-M09 systemd units are not overwritten.

Services:

| Service | Endpoint | Mode |
| --- | --- | --- |
| drevanta-engine | 127.0.0.1:8080 | `--mock`; state under `/var/lib/drevanta-engine` |
| drevanta-portal | 127.0.0.1:3000 | SvelteKit; proxies `/api/v1/*` to the Go engine |

The first engine boot creates `/var/lib/drevanta-engine/admin-password` (0600). A local VPS administrator can access it with `sudo cat /var/lib/drevanta-engine/admin-password`. Do not share or paste this secret into public logs.

## Access privately

From your own computer, open a tunnel to the test VPS:

```bash
ssh -L 3000:127.0.0.1:3000 user@your-test-vps
```

Then open `http://127.0.0.1:3000/engine` **on your computer**. This preserves a loopback-only, non-public installation. Avoid exposing port 3000 or the Go API directly on the internet.

The Engine page uses the real Go API in **mock mode**. Other UI resources remain browser-local samples where indicated.

## Upgrade, rollback, and service checks

From the audited repository checkout:

```bash
sudo ./deploy/install.sh --upgrade --accept-mock-only
sudo ./deploy/install.sh --rollback
./deploy/install.sh --verify
sudo systemctl status drevanta-engine drevanta-portal
sudo journalctl -u drevanta-engine -u drevanta-portal
```

Upgrades preserve `/var/lib/drevanta-engine`, build into `/opt/drevanta/releases/<release>`, atomically update `/opt/drevanta/current`, health-check and record the prior code release as `/opt/drevanta/previous`. If health fails, the installer attempts to restore the previous code and units. Explicit rollback selects the previous release and verifies it. Neither command restores data: **back up the state directory before important changes**.

The installer does not manage database migrations or a production snapshot/restore strategy.

## Architecture

```text
Your computer -- SSH tunnel --> SvelteKit 127.0.0.1:3000
                                 |
                                 | same-origin /api/v1/*
                                 v
                             Go engine 127.0.0.1:8080
                               --mock only
                                 |
                                 +-- /var/lib/drevanta-engine
                                 +-- No Docker socket / no Caddy / no public listener
```

## Validation

GitHub Actions runs `deploy/tests/installer-ci.sh` to check safe CLI and unit defaults, followed by `deploy/tests/stack-ci.sh`, which boots the **real production adapter-node output** and Go server over loopback and verifies health, protected API responses and login.

Passing mock CI does not demonstrate real HTTPS, ACME, Docker networking, zero downtime or disaster recovery. See [M08C disposable VPS acceptance](../engine/docs/M08C-ACCEPTANCE.md) before considering live mode. The M09 installer intentionally never enables `--live`, installs Caddy or publishes a site.
